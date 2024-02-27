package v1

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/osbuild/image-builder/internal/common"
	"github.com/osbuild/image-builder/internal/composer"
	"github.com/osbuild/image-builder/internal/provisioning"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/sirupsen/logrus"
)

func (h *Handlers) ComposeImage(ctx echo.Context) error {
	var composeRequest ComposeRequest
	err := ctx.Bind(&composeRequest)
	if err != nil {
		return err
	}
	composeResponse, err := h.handleCommonCompose(ctx, composeRequest, nil)
	if err != nil {
		return err
	}

	return ctx.JSON(http.StatusCreated, composeResponse)
}

func (h *Handlers) handleCommonCompose(ctx echo.Context, composeRequest ComposeRequest, blueprintVersionId *uuid.UUID) (ComposeResponse, error) {
	userID, err := h.server.getIdentity(ctx)
	if err != nil {
		return ComposeResponse{}, err
	}

	quotaOk, err := common.CheckQuota(userID.OrgID(), h.server.db, h.server.quotaFile)
	if err != nil {
		return ComposeResponse{}, err
	}
	if !quotaOk {
		return ComposeResponse{}, echo.NewHTTPError(http.StatusForbidden, "Quota exceeded for user")
	}

	if string(composeRequest.ImageRequests[0].UploadRequest.Type) == "" {
		return ComposeResponse{}, echo.NewHTTPError(http.StatusBadRequest, "Exactly one upload request should be included")
	}

	d, err := h.server.getDistro(ctx, composeRequest.Distribution)
	if err != nil {
		return ComposeResponse{}, err
	}

	var repositories []composer.Repository
	arch, err := d.Architecture(string(composeRequest.ImageRequests[0].Architecture))
	if err != nil {
		return ComposeResponse{}, err
	}
	for _, r := range arch.Repositories {
		// If no image type tags are defined for the repo, add the repo
		contains := len(r.ImageTypeTags) == 0
		for _, it := range r.ImageTypeTags {
			if it == string(composeRequest.ImageRequests[0].ImageType) {
				contains = true
				break
			}
		}
		if contains {
			repositories = append(repositories, composer.Repository{
				Baseurl:  r.Baseurl,
				Metalink: r.Metalink,
				Rhsm:     common.ToPtr(r.Rhsm),
			})
		}
	}

	uploadOptions, imageType, err := h.buildUploadOptions(ctx, composeRequest.ImageRequests[0].UploadRequest, composeRequest.ImageRequests[0].ImageType)
	if err != nil {
		return ComposeResponse{}, err
	}

	err = validateComposeRequest(&composeRequest)
	if err != nil {
		return ComposeResponse{}, err
	}

	distro := d.Distribution.Name
	if d.Distribution.ComposerName != nil {
		distro = *d.Distribution.ComposerName
	}

	customizations, err := buildCustomizations(composeRequest.Customizations)
	if err != nil {
		logrus.Errorf("Failed buliding customizations: %v", err)
		return ComposeResponse{}, echo.NewHTTPError(http.StatusInternalServerError, "Unable to build customizations")
	}

	cloudCR := composer.ComposeRequest{
		Distribution:   distro,
		Customizations: customizations,
		ImageRequest: &composer.ImageRequest{
			Architecture:  string(composeRequest.ImageRequests[0].Architecture),
			ImageType:     imageType,
			Size:          composeRequest.ImageRequests[0].Size,
			Ostree:        buildOSTreeOptions(composeRequest.ImageRequests[0].Ostree),
			Repositories:  repositories,
			UploadOptions: &uploadOptions,
		},
	}

	resp, err := h.server.cClient.Compose(cloudCR)
	if err != nil {
		return ComposeResponse{}, err
	}
	defer closeBody(resp.Body)
	if resp.StatusCode != http.StatusCreated {
		httpError := echo.NewHTTPError(http.StatusInternalServerError, "Failed posting compose request to osbuild-composer")
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			ctx.Logger().Errorf("Unable to parse composer's compose response: %v", err)
		} else {
			_ = httpError.SetInternal(fmt.Errorf("%s", body))
			var serviceStat composer.Error
			if err := json.Unmarshal(body, &serviceStat); err != nil {
				return ComposeResponse{}, httpError
			}
			if serviceStat.Id == "10" {
				httpError.Message = "Error resolving OSTree repo"
				httpError.Code = http.StatusBadRequest
			}
			// missing baseurl in payload repository
			if serviceStat.Id == "24" {
				httpError.Message = serviceStat.Reason
				httpError.Code = http.StatusBadRequest
			}
			// gpg key not set when check_gpg is true
			if serviceStat.Id == "29" {
				httpError.Message = serviceStat.Reason
				httpError.Code = http.StatusBadRequest
			}
		}
		return ComposeResponse{}, httpError
	}

	var composeResult composer.ComposeId
	err = json.NewDecoder(resp.Body).Decode(&composeResult)
	if err != nil {
		return ComposeResponse{}, err
	}

	rawCR, err := json.Marshal(composeRequest)
	if err != nil {
		return ComposeResponse{}, err
	}

	clientIdString := string(*composeRequest.ClientId)

	err = h.server.db.InsertCompose(composeResult.Id, userID.AccountNumber(), userID.Email(), userID.OrgID(), composeRequest.ImageName, rawCR, &clientIdString, blueprintVersionId)
	if err != nil {
		logrus.Error("Error inserting id into db", err)
		return ComposeResponse{}, err
	}

	ctx.Logger().Info("Compose result", composeResult)

	return ComposeResponse{
		Id: composeResult.Id,
	}, nil
}

func (h *Handlers) buildUploadOptions(ctx echo.Context, ur UploadRequest, it ImageTypes) (composer.UploadOptions, composer.ImageTypes, error) {
	var uploadOptions composer.UploadOptions
	switch ur.Type {
	case UploadTypesAws:
		var composerImageType composer.ImageTypes
		switch it {
		case ImageTypesAws:
			fallthrough
		case ImageTypesAmi:
			composerImageType = composer.ImageTypesAws
		default:
			return uploadOptions, "", echo.NewHTTPError(http.StatusBadRequest, "Invalid image type for upload target")
		}
		uo, err := ur.Options.AsAWSUploadRequestOptions()
		if err != nil {
			return uploadOptions, "", echo.NewHTTPError(http.StatusBadRequest, "Unable to parse upload request options as aws options")
		}

		if (uo.ShareWithAccounts == nil || len(*uo.ShareWithAccounts) == 0) && (uo.ShareWithSources == nil || len(*uo.ShareWithSources) == 0) {
			return uploadOptions, "", echo.NewHTTPError(http.StatusBadRequest, "Expected at least one source or account to share the image with")
		}

		var shareWithAccounts []string
		if uo.ShareWithAccounts != nil {
			shareWithAccounts = append(shareWithAccounts, *uo.ShareWithAccounts...)
		}

		if uo.ShareWithSources != nil {
			for _, source := range *uo.ShareWithSources {
				resp, err := h.server.pClient.GetUploadInfo(ctx.Request().Context(), source)
				if err != nil {
					logrus.Error(err)
					return uploadOptions, "", echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("Unable to request source: %s", source))
				}
				defer closeBody(resp.Body)

				var uploadInfo provisioning.V1SourceUploadInfoResponse
				err = json.NewDecoder(resp.Body).Decode(&uploadInfo)
				if err != nil {
					return uploadOptions, "", echo.NewHTTPError(http.StatusInternalServerError, fmt.Sprintf("Unable to resolve source: %s", source))
				}

				if uploadInfo.Aws == nil || uploadInfo.Aws.AccountId == nil || len(*uploadInfo.Aws.AccountId) != 12 {
					return uploadOptions, "", echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("Unable to resolve source %s to an aws account id", source))
				}

				ctx.Logger().Info(fmt.Sprintf("Resolved source %s, to account id %s", strings.Replace(source, "\n", "", -1), *uploadInfo.Aws.AccountId))
				shareWithAccounts = append(shareWithAccounts, *uploadInfo.Aws.AccountId)
			}
		}
		err = uploadOptions.FromAWSEC2UploadOptions(composer.AWSEC2UploadOptions{
			Region:            h.server.aws.Region,
			ShareWithAccounts: shareWithAccounts,
		})
		if err != nil {
			return uploadOptions, "", err
		}
		return uploadOptions, composerImageType, nil
	case UploadTypesAwsS3:
		var composerImageType composer.ImageTypes
		switch it {
		case ImageTypesEdgeCommit:
			fallthrough
		case ImageTypesRhelEdgeCommit:
			composerImageType = composer.ImageTypesEdgeCommit
		case ImageTypesEdgeInstaller:
			fallthrough
		case ImageTypesRhelEdgeInstaller:
			composerImageType = composer.ImageTypesEdgeInstaller
		case ImageTypesGuestImage:
			composerImageType = composer.ImageTypesGuestImage
		case ImageTypesImageInstaller:
			composerImageType = composer.ImageTypesImageInstaller
		case ImageTypesVsphere:
			composerImageType = composer.ImageTypesVsphere
		case ImageTypesVsphereOva:
			composerImageType = composer.ImageTypesVsphereOva
		case ImageTypesWsl:
			composerImageType = composer.ImageTypesWsl
		default:
			return uploadOptions, "", echo.NewHTTPError(http.StatusBadRequest, "Invalid image type for upload target")
		}
		err := uploadOptions.FromAWSS3UploadOptions(composer.AWSS3UploadOptions{
			Region: h.server.aws.Region,
		})
		if err != nil {
			return uploadOptions, "", err
		}
		return uploadOptions, composerImageType, nil
	case UploadTypesGcp:
		var composerImageType composer.ImageTypes
		switch it {
		case ImageTypesGcp:
			composerImageType = composer.ImageTypesGcp
		default:
			return uploadOptions, "", echo.NewHTTPError(http.StatusBadRequest, "Invalid image type for upload target")
		}
		uo, err := ur.Options.AsGCPUploadRequestOptions()
		if err != nil {
			return uploadOptions, "", echo.NewHTTPError(http.StatusBadRequest, "Unable to parse upload request options as GCP options")
		}
		err = uploadOptions.FromGCPUploadOptions(composer.GCPUploadOptions{
			Bucket:            &h.server.gcp.Bucket,
			Region:            h.server.gcp.Region,
			ShareWithAccounts: uo.ShareWithAccounts,
		})
		if err != nil {
			return uploadOptions, "", err
		}
		return uploadOptions, composerImageType, nil
	case UploadTypesAzure:
		var composerImageType composer.ImageTypes
		switch it {
		case ImageTypesAzure:
			fallthrough
		case ImageTypesVhd:
			composerImageType = composer.ImageTypesAzure
		default:
			return uploadOptions, "", echo.NewHTTPError(http.StatusBadRequest, "Invalid image type for upload target")
		}

		uo, err := ur.Options.AsAzureUploadRequestOptions()
		if err != nil {
			return uploadOptions, "", echo.NewHTTPError(http.StatusBadRequest, "Unable to parse upload request options as Azure options")
		}

		if (uo.SourceId == nil && (uo.TenantId == nil || uo.SubscriptionId == nil)) ||
			(uo.SourceId != nil && (uo.TenantId != nil || uo.SubscriptionId != nil)) {
			return uploadOptions, "", echo.NewHTTPError(http.StatusBadRequest, "Request must contain either (1) a source id, and no tenant or subscription ids or (2) tenant and subscription ids, and no source id.")
		}

		var tenantId string
		var subscriptionId string

		if uo.SourceId == nil {
			tenantId = *uo.TenantId
			subscriptionId = *uo.SubscriptionId
		}

		if uo.SourceId != nil {
			resp, err := h.server.pClient.GetUploadInfo(ctx.Request().Context(), *uo.SourceId)
			if err != nil {
				logrus.Error(err)
				return uploadOptions, "", echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("Unable to request source: %s", *uo.SourceId))
			}
			defer closeBody(resp.Body)

			var uploadInfo provisioning.V1SourceUploadInfoResponse
			err = json.NewDecoder(resp.Body).Decode(&uploadInfo)

			if err != nil {
				return uploadOptions, "", echo.NewHTTPError(http.StatusInternalServerError, fmt.Sprintf("Unable to resolve source: %s", *uo.SourceId))
			}

			if uploadInfo.Azure == nil || uploadInfo.Azure.TenantId == nil || uploadInfo.Azure.SubscriptionId == nil {
				return uploadOptions, "", echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("Unable to resolve source %s to an Azure tenant id or subscription id. ", *uo.SourceId))
			}

			ctx.Logger().Info(fmt.Sprintf("Resolved source %s to tenant id %s and subscription id %s", *uo.SourceId, *uploadInfo.Azure.TenantId, *uploadInfo.Azure.SubscriptionId))
			tenantId = *uploadInfo.Azure.TenantId
			subscriptionId = *uploadInfo.Azure.SubscriptionId
		}

		err = uploadOptions.FromAzureUploadOptions(composer.AzureUploadOptions{
			TenantId:       tenantId,
			SubscriptionId: subscriptionId,
			ResourceGroup:  uo.ResourceGroup,
			ImageName:      uo.ImageName,
		})
		if err != nil {
			return uploadOptions, "", err
		}
		return uploadOptions, composerImageType, nil
	case UploadTypesOciObjectstorage:
		if it != ImageTypesOci {
			return uploadOptions, "", echo.NewHTTPError(http.StatusBadRequest, "Invalid image type for upload target")
		}
		err := uploadOptions.FromOCIUploadOptions(composer.OCIUploadOptions{})
		if err != nil {
			return uploadOptions, "", err
		}
		return uploadOptions, composer.ImageTypesOci, err
	default:
		return uploadOptions, "", echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("Unknown UploadRequest type %s", ur.Type))
	}
}

func buildOSTreeOptions(ostreeOptions *OSTree) *composer.OSTree {
	if ostreeOptions == nil {
		return nil
	}

	cloudOptions := new(composer.OSTree)
	if ostreeOptions != nil {
		cloudOptions.Ref = ostreeOptions.Ref
		cloudOptions.Url = ostreeOptions.Url
		cloudOptions.Contenturl = ostreeOptions.Contenturl
		cloudOptions.Parent = ostreeOptions.Parent
		cloudOptions.Rhsm = ostreeOptions.Rhsm
	}
	return cloudOptions
}

// validateComposeRequest makes sure the image size is not too large for AWS or Azure
// It takes into account the requested image size, and the total size of requested
// filesystem customizations.
func validateComposeRequest(cr *ComposeRequest) error {
	var totalSize uint64
	cust := cr.Customizations
	if cust != nil && cust.Filesystem != nil {
		for _, v := range *cust.Filesystem {
			totalSize += v.MinSize
		}
	}

	// The total size will be the larger of the requested size or the filesystems
	if cr.ImageRequests[0].Size != nil && *cr.ImageRequests[0].Size > totalSize {
		totalSize = *cr.ImageRequests[0].Size
	}

	if totalSize > FSMaxSize {
		it := cr.ImageRequests[0].ImageType
		switch it {
		case ImageTypesAmi, ImageTypesAws:
			return echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("Total AWS image size cannot exceed %d bytes", FSMaxSize))
		case ImageTypesAzure, ImageTypesVhd:
			return echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("Total Azure image size cannot exceed %d bytes", FSMaxSize))
		}
	}

	return nil
}

func buildCustomizations(cust *Customizations) (*composer.Customizations, error) {
	if cust == nil {
		return nil, nil
	}

	res := &composer.Customizations{}
	if cust.Subscription != nil {
		res.Subscription = &composer.Subscription{
			ActivationKey: cust.Subscription.ActivationKey,
			BaseUrl:       cust.Subscription.BaseUrl,
			Insights:      cust.Subscription.Insights,
			Rhc:           cust.Subscription.Rhc,
			Organization:  fmt.Sprintf("%d", cust.Subscription.Organization),
			ServerUrl:     cust.Subscription.ServerUrl,
		}
	}

	if cust.Packages != nil {
		res.Packages = cust.Packages
	}

	if cust.PayloadRepositories != nil {
		payloadRepositories := make([]composer.Repository, len(*cust.PayloadRepositories))
		for i, payloadRepository := range *cust.PayloadRepositories {
			if payloadRepository.Baseurl != nil {
				payloadRepositories[i].Baseurl = payloadRepository.Baseurl
			}
			if payloadRepository.CheckGpg != nil {
				payloadRepositories[i].CheckGpg = payloadRepository.CheckGpg
			}
			if payloadRepository.CheckRepoGpg != nil {
				payloadRepositories[i].CheckRepoGpg = payloadRepository.CheckRepoGpg
			}
			if payloadRepository.Gpgkey != nil {
				payloadRepositories[i].Gpgkey = payloadRepository.Gpgkey
			}
			if payloadRepository.IgnoreSsl != nil {
				payloadRepositories[i].IgnoreSsl = payloadRepository.IgnoreSsl
			}
			if payloadRepository.Metalink != nil {
				payloadRepositories[i].Metalink = payloadRepository.Metalink
			}
			if payloadRepository.Mirrorlist != nil {
				payloadRepositories[i].Mirrorlist = payloadRepository.Mirrorlist
			}
			payloadRepositories[i].ModuleHotfixes = payloadRepository.ModuleHotfixes
			payloadRepositories[i].Rhsm = common.ToPtr(payloadRepository.Rhsm)
		}
		res.PayloadRepositories = &payloadRepositories
	}

	if cust.CustomRepositories != nil {
		customRepositories := make([]composer.CustomRepository, len(*cust.CustomRepositories))
		for i, customRepository := range *cust.CustomRepositories {
			if customRepository.Id != "" {
				customRepositories[i].Id = customRepository.Id
			}
			if customRepository.Name != nil {
				customRepositories[i].Name = customRepository.Name
			}
			if customRepository.Filename != nil {
				customRepositories[i].Filename = customRepository.Filename
			}
			if customRepository.Baseurl != nil {
				customRepositories[i].Baseurl = customRepository.Baseurl
			}
			if customRepository.CheckGpg != nil {
				customRepositories[i].CheckGpg = customRepository.CheckGpg
			}
			if customRepository.CheckRepoGpg != nil {
				customRepositories[i].CheckRepoGpg = customRepository.CheckRepoGpg
			}
			if customRepository.Gpgkey != nil {
				customRepositories[i].Gpgkey = customRepository.Gpgkey
			}
			if customRepository.SslVerify != nil {
				customRepositories[i].SslVerify = customRepository.SslVerify
			}
			if customRepository.Metalink != nil {
				customRepositories[i].Metalink = customRepository.Metalink
			}
			if customRepository.Mirrorlist != nil {
				customRepositories[i].Mirrorlist = customRepository.Mirrorlist
			}
			if customRepository.Priority != nil {
				customRepositories[i].Priority = customRepository.Priority
			}
			if customRepository.Enabled != nil {
				customRepositories[i].Enabled = customRepository.Enabled
			}
			customRepositories[i].ModuleHotfixes = customRepository.ModuleHotfixes
		}
		res.CustomRepositories = &customRepositories
	}

	if cust.Openscap != nil {
		res.Openscap = &composer.OpenSCAP{
			ProfileId: cust.Openscap.ProfileId,
		}
	}

	if cust.Filesystem != nil && len(*cust.Filesystem) > 0 {
		var fsc []composer.Filesystem
		for _, v := range *cust.Filesystem {
			fsc = append(fsc, composer.Filesystem{
				Mountpoint: v.Mountpoint,
				MinSize:    v.MinSize,
			})
		}
		res.Filesystem = &fsc
	}

	if cust.Users != nil {
		var users []composer.User
		for _, u := range *cust.Users {
			groups := &[]string{"wheel"}
			users = append(users, composer.User{
				Name:   u.Name,
				Key:    common.ToPtr(u.SshKey),
				Groups: groups,
			})
		}
		res.Users = &users
	}

	if cust.Groups != nil {
		var groups []composer.Group
		for _, g := range *cust.Groups {
			groups = append(groups, composer.Group{
				Gid:  g.Gid,
				Name: g.Name,
			})
		}
		res.Groups = &groups
	}

	if cust.PartitioningMode != nil {
		switch *cust.PartitioningMode {
		case AutoLvm:
			res.PartitioningMode = common.ToPtr(composer.AutoLvm)
		case Lvm:
			res.PartitioningMode = common.ToPtr(composer.Lvm)
		case Raw:
			res.PartitioningMode = common.ToPtr(composer.Raw)
		}
	}

	if cust.Containers != nil {
		var containers []composer.Container
		for _, c := range *cust.Containers {
			containers = append(containers, composer.Container{
				Name:      c.Name,
				Source:    c.Source,
				TlsVerify: c.TlsVerify,
			})
		}
		res.Containers = &containers
	}

	if cust.Directories != nil {
		var dirs []composer.Directory
		for _, d := range *cust.Directories {
			// parse as int (DirectoryGroup1)
			var group *composer.Directory_Group
			if d.Group != nil {
				group = &composer.Directory_Group{}
				dg0, err := d.Group.AsDirectoryGroup0()
				if err == nil {
					err = group.FromDirectoryGroup0(dg0)
					if err != nil {
						return nil, err
					}

				}
				dg1, err := d.Group.AsDirectoryGroup1()
				if err == nil {
					err = group.FromDirectoryGroup1(dg1)
					if err != nil {
						return nil, err
					}
				}
			}

			var user *composer.Directory_User
			if d.User != nil {
				user = &composer.Directory_User{}
				du0, err := d.User.AsDirectoryUser0()
				if err == nil {
					err = user.FromDirectoryUser0(du0)
					if err != nil {
						return nil, err
					}
				}
				du1, err := d.User.AsDirectoryUser1()
				if err == nil {
					err = user.FromDirectoryUser1(composer.DirectoryUser1(du1))
					if err != nil {
						return nil, err
					}
				}
			}

			dirs = append(dirs, composer.Directory{
				EnsureParents: d.EnsureParents,
				Group:         group,
				Mode:          d.Mode,
				Path:          d.Path,
				User:          user,
			})
		}
		res.Directories = &dirs
	}

	if cust.Files != nil {
		var files []composer.File
		for _, f := range *cust.Files {
			var group *composer.File_Group
			if f.Group != nil {
				group = &composer.File_Group{}
				fg0, err := f.Group.AsFileGroup0()
				if err == nil {
					err = group.FromFileGroup0(fg0)
					if err != nil {
						return nil, err
					}
				}

				fg1, err := f.Group.AsFileGroup1()
				if err == nil {
					err = group.FromFileGroup1(fg1)
					if err != nil {
						return nil, err
					}
				}
			}

			var user *composer.File_User
			if f.User != nil {
				user = &composer.File_User{}
				fu0, err := f.User.AsFileUser0()
				if err == nil {
					err = user.FromFileUser0(fu0)
					if err != nil {
						return nil, err
					}
				}
				fu1, err := f.User.AsFileUser1()
				if err == nil {
					err = user.FromFileUser1(composer.FileUser1(fu1))
					if err != nil {
						return nil, err
					}
				}
			}

			files = append(files, composer.File{
				Data:          f.Data,
				EnsureParents: f.EnsureParents,
				Group:         group,
				Mode:          f.Mode,
				Path:          f.Path,
				User:          user,
			})
		}
		res.Files = &files
	}

	if cust.Locale != nil {
		res.Locale = &composer.Locale{
			Keyboard:  cust.Locale.Keyboard,
			Languages: cust.Locale.Languages,
		}
	}

	if cust.Kernel != nil {
		res.Kernel = &composer.Kernel{
			Append: cust.Kernel.Append,
			Name:   cust.Kernel.Name,
		}
	}

	if cust.Services != nil {
		res.Services = &composer.Services{
			Disabled: cust.Services.Disabled,
			Enabled:  cust.Services.Enabled,
		}
	}

	if cust.Firewall != nil {
		res.Firewall = &composer.FirewallCustomization{
			Ports: cust.Firewall.Ports,
		}

		if cust.Firewall.Services != nil {
			res.Firewall.Services = &struct {
				Disabled *[]string `json:"disabled,omitempty"`
				Enabled  *[]string `json:"enabled,omitempty"`
			}{
				Disabled: cust.Firewall.Services.Disabled,
				Enabled:  cust.Firewall.Services.Enabled,
			}
		}
	}

	if cust.Timezone != nil {
		res.Timezone = &composer.Timezone{
			Ntpservers: cust.Timezone.Ntpservers,
			Timezone:   cust.Timezone.Timezone,
		}
	}

	if cust.InstallationDevice != nil {
		res.InstallationDevice = cust.InstallationDevice
	}

	if cust.Fdo != nil {
		res.Fdo = &composer.FDO{
			DiunPubKeyHash:         cust.Fdo.DiunPubKeyHash,
			DiunPubKeyInsecure:     cust.Fdo.DiunPubKeyInsecure,
			DiunPubKeyRootCerts:    cust.Fdo.DiunPubKeyRootCerts,
			ManufacturingServerUrl: cust.Fdo.ManufacturingServerUrl,
		}
	}

	if cust.Ignition != nil {
		res.Ignition = &composer.Ignition{}
		if cust.Ignition.Embedded != nil {
			res.Ignition.Embedded = &composer.IgnitionEmbedded{
				Config: cust.Ignition.Embedded.Config,
			}
		}
		if cust.Ignition.Firstboot != nil {
			res.Ignition.Firstboot = &composer.IgnitionFirstboot{
				Url: cust.Ignition.Firstboot.Url,
			}
		}
	}

	if cust.Fips != nil {
		res.Fips = &composer.FIPS{
			Enabled: cust.Fips.Enabled,
		}
	}

	return res, nil
}
