package v1

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"time"

	"github.com/osbuild/image-builder/internal/common"
	"github.com/osbuild/image-builder/internal/composer"
	"github.com/osbuild/image-builder/internal/db"
	"github.com/osbuild/image-builder/internal/distribution"

	"github.com/labstack/echo/v4"
	"github.com/sirupsen/logrus"
)

func (h *Handlers) GetVersion(ctx echo.Context) error {
	version := Version{h.server.spec.Info.Version}
	return ctx.JSON(http.StatusOK, version)
}

func (h *Handlers) GetReadiness(ctx echo.Context) error {
	resp, err := h.server.client.OpenAPI()
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return err
		}
		return echo.NewHTTPError(http.StatusInternalServerError, fmt.Sprintf("Failed to contact osbuild-composer: %s", body))
	}

	ready := map[string]string{
		"readiness": "ready",
	}

	return ctx.JSON(http.StatusOK, ready)
}

func (h *Handlers) GetOpenapiJson(ctx echo.Context) error {
	return ctx.JSON(http.StatusOK, h.server.spec)
}

func (h *Handlers) GetDistributions(ctx echo.Context) error {
	dr := h.server.distroRegistry(ctx)

	var distributions DistributionsResponse
	for _, d := range dr.List() {
		distributions = append(distributions, DistributionItem{
			Description: d.Distribution.Description,
			Name:        d.Distribution.Name,
		})
	}

	return ctx.JSON(http.StatusOK, distributions)
}

func (h *Handlers) GetArchitectures(ctx echo.Context, distro string) error {
	d, err := h.server.distroRegistry(ctx).Get(distroToStr(Distributions(distro)))
	if err == distribution.DistributionNotFound {
		return echo.NewHTTPError(http.StatusBadRequest, err)
	}
	if err != nil {
		return err
	}

	var archs Architectures
	if d.ArchX86 != nil {
		archs = append(archs, ArchitectureItem{
			Arch:       "x86_64",
			ImageTypes: d.ArchX86.ImageTypes,
		})
	}

	return ctx.JSON(http.StatusOK, archs)
}

func (h *Handlers) GetPackages(ctx echo.Context, params GetPackagesParams) error {
	dr := h.server.distroRegistry(ctx)
	d, err := dr.Get(distroToStr(params.Distribution))
	if err != nil {
		return err
	}
	arch, err := d.Architecture(string(params.Architecture))
	if err != nil {
		return err
	}

	pkgs := arch.FindPackages(params.Search)
	var packages []Package
	for _, p := range pkgs {
		packages = append(packages,
			Package{
				Name:    p.Name,
				Summary: p.Summary,
			})
	}

	limit := 100
	if params.Limit != nil {
		if *params.Limit > 0 {
			limit = *params.Limit
		}
	}

	offset := 0
	if params.Offset != nil {
		if *params.Offset >= len(packages) {
			offset = len(packages) - 1
		} else if *params.Offset > 0 {
			offset = *params.Offset
		}
	}

	upto := offset + limit
	if upto > len(packages) {
		upto = len(packages)
	}

	return ctx.JSON(http.StatusOK, PackagesResponse{
		Meta: struct {
			Count int `json:"count"`
		}{
			len(packages),
		},
		Links: struct {
			First string `json:"first"`
			Last  string `json:"last"`
		}{
			fmt.Sprintf("%v/v%v/packages?search=%v&distribution=%v&architecture=%v&offset=%v&limit=%v",
				RoutePrefix(), h.server.spec.Info.Version, params.Search, params.Distribution, params.Architecture, offset, limit),
			fmt.Sprintf("%v/v%v/packages?search=%v&distribution=%v&architecture=%v&offset=%v&limit=%v",
				RoutePrefix(), h.server.spec.Info.Version, params.Search, params.Distribution, params.Architecture, len(packages)-1, limit),
		},
		Data: packages[offset:upto],
	})
}

func (h *Handlers) GetComposeStatus(ctx echo.Context, composeId string) error {
	err := h.canUserAccessComposeId(ctx, composeId)
	if err != nil {
		return err
	}

	resp, err := h.server.client.ComposeStatus(composeId)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return err
		}
		// Composes can get deleted in composer, usually when the image is expired
		return echo.NewHTTPError(http.StatusNotFound, fmt.Sprintf("%s", body))
	} else if resp.StatusCode != http.StatusOK {
		httpError := echo.NewHTTPError(http.StatusInternalServerError, "Failed querying compose status")
		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			logrus.Errorf("Unable to parse composer's compose response: %v", err)
		} else {
			_ = httpError.SetInternal(fmt.Errorf("%s", body))
		}
		return httpError
	}

	var cloudStat composer.ComposeStatus
	err = json.NewDecoder(resp.Body).Decode(&cloudStat)
	if err != nil {
		return err
	}

	status := ComposeStatus{
		ImageStatus: ImageStatus{
			Status:       ImageStatusStatus(cloudStat.ImageStatus.Status),
			UploadStatus: nil,
		},
	}

	if cloudStat.ImageStatus.UploadStatus != nil {
		status.ImageStatus.UploadStatus = &UploadStatus{
			Status:  UploadStatusStatus(cloudStat.ImageStatus.UploadStatus.Status),
			Type:    UploadTypes(cloudStat.ImageStatus.UploadStatus.Type),
			Options: cloudStat.ImageStatus.UploadStatus.Options,
		}
	}

	if cloudStat.ImageStatus.Error != nil {
		status.ImageStatus.Error = parseComposeStatusError(cloudStat.ImageStatus.Error)
	}

	return ctx.JSON(http.StatusOK, status)
}

func parseComposeStatusError(composeErr *composer.ComposeStatusError) *ComposeStatusError {
	if composeErr == nil {
		return nil
	}

	// Default top-level error
	fbErr := &ComposeStatusError{
		Id:      composeErr.Id,
		Reason:  composeErr.Reason,
		Details: composeErr.Details,
	}

	switch composeErr.Id {
	case 5: // manifest error: depsolve dependency failure
		fallthrough
	case 9: // osbuild error: manifest dependency failure
		fallthrough
	case 26: // ErrorJobDependency: generic dependency failure
		fallthrough
	case 28: // osbuild target errors are added to details
		if composeErr.Details != nil {
			intfs := (*composeErr.Details).([]interface{})
			if len(intfs) == 0 {
				return fbErr
			}

			// Try to remarshal the details as another composer.ComposeStatusError
			jsonDetails, err := json.Marshal(intfs[0])
			if err != nil {
				logrus.Errorf("Error processing compose status error details: %v", err)
				return fbErr
			}
			var newErr composer.ComposeStatusError
			err = json.Unmarshal(jsonDetails, &newErr)
			if err != nil {
				logrus.Errorf("Error processing compose status error details: %v", err)
				return fbErr
			}

			return parseComposeStatusError(&newErr)
		}
		return fbErr
	default:
		return fbErr
	}
}

func (h *Handlers) GetComposeMetadata(ctx echo.Context, composeId string) error {
	err := h.canUserAccessComposeId(ctx, composeId)
	if err != nil {
		return err
	}

	resp, err := h.server.client.ComposeMetadata(composeId)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return err
		}
		return echo.NewHTTPError(http.StatusNotFound, fmt.Sprintf("%s", body))
	} else if resp.StatusCode != http.StatusOK {
		httpError := echo.NewHTTPError(http.StatusInternalServerError, "Failed querying compose status")
		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			logrus.Errorf("Unable to parse composer's compose response: %v", err)
		} else {
			_ = httpError.SetInternal(fmt.Errorf("%s", body))
		}
		return httpError
	}

	var cloudStat composer.ComposeMetadata
	err = json.NewDecoder(resp.Body).Decode(&cloudStat)
	if err != nil {
		return err
	}

	var packages []PackageMetadata
	if cloudStat.Packages != nil {
		packages = make([]PackageMetadata, len(*cloudStat.Packages))
		for idx, cloudPkg := range *cloudStat.Packages {
			packages[idx] = PackageMetadata{
				Arch:      cloudPkg.Arch,
				Epoch:     cloudPkg.Epoch,
				Name:      cloudPkg.Name,
				Release:   cloudPkg.Release,
				Sigmd5:    cloudPkg.Sigmd5,
				Signature: cloudPkg.Signature,
				Type:      cloudPkg.Type,
				Version:   cloudPkg.Version,
			}
		}
	}
	status := ComposeMetadata{
		OstreeCommit: cloudStat.OstreeCommit,
		Packages:     &packages,
	}

	return ctx.JSON(http.StatusOK, status)
}

// return an error if the user does not have the composeId associated to its OrgID in the DB, nil otherwise
func (h *Handlers) canUserAccessComposeId(ctx echo.Context, composeId string) error {
	idHeader, err := getIdentityHeader(ctx)
	if err != nil {
		return err
	}

	_, err = h.server.db.GetCompose(composeId, idHeader.Identity.OrgID)
	if err != nil {
		if errors.Is(err, db.ComposeNotFoundError) {
			return echo.NewHTTPError(http.StatusNotFound, err)
		} else {
			return err
		}
	}
	return nil
}

func (h *Handlers) GetComposes(ctx echo.Context, params GetComposesParams) error {
	spec, err := GetSwagger()
	if err != nil {
		return err
	}

	idHeader, err := getIdentityHeader(ctx)
	if err != nil {
		return err
	}

	limit := 100
	if params.Limit != nil {
		if *params.Limit > 0 {
			limit = *params.Limit
		}
	}

	offset := 0
	if params.Offset != nil {
		offset = *params.Offset
	}

	// composes in the last 14 days
	composes, count, err := h.server.db.GetComposes(idHeader.Identity.OrgID, (time.Hour * 24 * 14), limit, offset)
	if err != nil {
		return err
	}

	var data []ComposesResponseItem
	for _, c := range composes {
		data = append(data, ComposesResponseItem{
			CreatedAt: c.CreatedAt.String(),
			Id:        c.Id.String(),
			ImageName: c.ImageName,
			Request:   c.Request,
		})
	}

	lastOffset := count - 1
	if lastOffset < 0 {
		lastOffset = 0
	}

	return ctx.JSON(http.StatusOK, ComposesResponse{
		Meta: struct {
			Count int `json:"count"`
		}{
			count,
		},
		Links: struct {
			First string `json:"first"`
			Last  string `json:"last"`
		}{
			fmt.Sprintf("%v/v%v/composes?offset=%v&limit=%v",
				RoutePrefix(), spec.Info.Version, offset, limit),
			fmt.Sprintf("%v/v%v/composes?offset=%v&limit=%v",
				RoutePrefix(), spec.Info.Version, lastOffset, limit),
		},
		Data: data,
	})
}

func (h *Handlers) ComposeImage(ctx echo.Context) error {
	idHeader, err := getIdentityHeader(ctx)
	if err != nil {
		return err
	}

	quotaOk, err := common.CheckQuota(idHeader.Identity.OrgID, h.server.db, h.server.quotaFile)
	if err != nil {
		return err
	}
	if !quotaOk {
		return echo.NewHTTPError(http.StatusForbidden, "Quota exceeded for user")
	}

	var composeRequest ComposeRequest
	err = ctx.Bind(&composeRequest)
	if err != nil {
		return err
	}

	if (composeRequest.ImageRequests[0].UploadRequest == UploadRequest{}) {
		return echo.NewHTTPError(http.StatusBadRequest, "Exactly one upload request should be included")
	}

	d, err := h.server.distroRegistry(ctx).Get(distroToStr(composeRequest.Distribution))
	if err != nil {
		return err
	}

	if d.IsRestricted() {
		allowOk, err := h.server.allowList.IsAllowed(idHeader.Identity.Internal.OrgID, distroToStr(composeRequest.Distribution))
		if err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
		}
		if !allowOk {
			message := fmt.Sprintf("This account's organization is not authorized to build %s images", distroToStr(composeRequest.Distribution))
			return echo.NewHTTPError(http.StatusForbidden, message)
		}
	}

	var repositories []composer.Repository
	arch, err := d.Architecture(string(composeRequest.ImageRequests[0].Architecture))
	if err != nil {
		return err
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
				Rhsm:     common.BoolToPtr(r.Rhsm),
			})
		}
	}

	uploadOptions, imageType, err := h.buildUploadOptions(composeRequest.ImageRequests[0].UploadRequest, composeRequest.ImageRequests[0].ImageType)
	if err != nil {
		return err
	}

	// check here for installer types
	if composeRequest.Customizations != nil && composeRequest.Customizations.Users != nil && !strings.Contains(string(imageType), "installer") {
		return echo.NewHTTPError(http.StatusBadRequest, "User customization only applies to installer image types")
	}

	cloudCR := composer.ComposeRequest{
		Distribution:   distroToStr(composeRequest.Distribution),
		Customizations: buildCustomizations(composeRequest.Customizations),
		ImageRequest: &composer.ImageRequest{
			Architecture:  string(composeRequest.ImageRequests[0].Architecture),
			ImageType:     imageType,
			Ostree:        buildOSTreeOptions(composeRequest.ImageRequests[0].Ostree),
			Repositories:  repositories,
			UploadOptions: &uploadOptions,
		},
	}

	resp, err := h.server.client.Compose(cloudCR)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		httpError := echo.NewHTTPError(http.StatusInternalServerError, "Failed posting compose request to osbuild-composer")
		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			logrus.Errorf("Unable to parse composer's compose response: %v", err)
		} else {
			_ = httpError.SetInternal(fmt.Errorf("%s", body))
		}
		return httpError
	}

	var composeResult composer.ComposeId
	err = json.NewDecoder(resp.Body).Decode(&composeResult)
	if err != nil {
		return err
	}

	rawCR, err := json.Marshal(composeRequest)
	if err != nil {
		return err
	}

	err = h.server.db.InsertCompose(composeResult.Id.String(), idHeader.Identity.AccountNumber, idHeader.Identity.Internal.OrgID, composeRequest.ImageName, rawCR)
	if err != nil {
		logrus.Error("Error inserting id into db", err)
		return err
	}

	logrus.Info("Compose result", composeResult)
	return ctx.JSON(http.StatusCreated, ComposeResponse{
		Id: composeResult.Id.String(),
	})
}

func distroToStr(distro Distributions) string {
	switch distro {
	case Rhel8:
		return string(Rhel86)
	default:
		return string(distro)
	}
}

func (h *Handlers) buildUploadOptions(ur UploadRequest, it ImageTypes) (composer.UploadOptions, composer.ImageTypes, error) {
	// HACK deepmap doesn't really support `oneOf`, so marshal and unmarshal into target object
	optionsJSON, err := json.Marshal(ur.Options)
	if err != nil {
		return nil, "", echo.NewHTTPError(http.StatusBadRequest, "Unable to marshal UploadRequestOptions")
	}
	switch ur.Type {
	case UploadTypesAws:
		var composerImageType composer.ImageTypes
		switch it {
		case ImageTypesAws:
			fallthrough
		case ImageTypesAmi:
			composerImageType = composer.ImageTypesAws
		default:
			return nil, "", echo.NewHTTPError(http.StatusBadRequest, "Invalid image type for upload target")
		}
		var awsOptions AWSUploadRequestOptions
		err = json.Unmarshal(optionsJSON, &awsOptions)
		if err != nil {
			return nil, "", echo.NewHTTPError(http.StatusBadRequest, "Unable to unmarshal UploadRequestOptions")
		}
		return composer.AWSEC2UploadOptions{
			Region:            h.server.aws.Region,
			ShareWithAccounts: awsOptions.ShareWithAccounts,
		}, composerImageType, nil
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
		default:
			return nil, "", echo.NewHTTPError(http.StatusBadRequest, "Invalid image type for upload target")
		}
		var awsOptions AWSS3UploadRequestOptions
		err = json.Unmarshal(optionsJSON, &awsOptions)
		if err != nil {
			return nil, "", echo.NewHTTPError(http.StatusBadRequest, "Unable to unmarshal UploadRequestOptions")
		}
		return composer.AWSS3UploadOptions{
			Region: h.server.aws.Region,
		}, composerImageType, nil
	case UploadTypesGcp:
		var composerImageType composer.ImageTypes
		switch it {
		case ImageTypesGcp:
			composerImageType = composer.ImageTypesGcp
		default:
			return nil, "", echo.NewHTTPError(http.StatusBadRequest, "Invalid image type for upload target")
		}
		var gcpOptions GCPUploadRequestOptions
		err = json.Unmarshal(optionsJSON, &gcpOptions)
		if err != nil {
			return nil, "", echo.NewHTTPError(http.StatusBadRequest, "Unable to unmarshal into GCPUploadRequestOptions")
		}
		return composer.GCPUploadOptions{
			Bucket:            h.server.gcp.Bucket,
			Region:            h.server.gcp.Region,
			ShareWithAccounts: &gcpOptions.ShareWithAccounts,
		}, composerImageType, nil
	case UploadTypesAzure:
		var composerImageType composer.ImageTypes
		switch it {
		case ImageTypesAzure:
			fallthrough
		case ImageTypesVhd:
			composerImageType = composer.ImageTypesAzure
		default:
			return nil, "", echo.NewHTTPError(http.StatusBadRequest, "Invalid image type for upload target")
		}
		var azureOptions AzureUploadRequestOptions
		err = json.Unmarshal(optionsJSON, &azureOptions)
		if err != nil {
			return nil, "", echo.NewHTTPError(http.StatusBadRequest, "Unable to unmarshal into AzureUploadRequestOptions")
		}
		return composer.AzureUploadOptions{
			TenantId:       azureOptions.TenantId,
			SubscriptionId: azureOptions.SubscriptionId,
			ResourceGroup:  azureOptions.ResourceGroup,
			Location:       h.server.azure.Location,
		}, composerImageType, nil
	default:
		return nil, "", echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("Unknown UploadRequest type %s", ur.Type))
	}
}

func buildOSTreeOptions(ostreeOptions *OSTree) *composer.OSTree {
	if ostreeOptions == nil {
		return nil
	}

	cloudOptions := new(composer.OSTree)
	if ostreeOptions != nil {
		if ref := ostreeOptions.Ref; ref != nil {
			cloudOptions.Ref = ref
		}
		if url := ostreeOptions.Url; url != nil {
			cloudOptions.Url = url
		}
		if parent := ostreeOptions.Parent; parent != nil {
			cloudOptions.Parent = parent
		}
	}
	return cloudOptions
}

func buildCustomizations(cust *Customizations) *composer.Customizations {
	if cust == nil {
		return nil
	}

	res := &composer.Customizations{}
	if cust.Subscription != nil {
		res.Subscription = &composer.Subscription{
			ActivationKey: cust.Subscription.ActivationKey,
			BaseUrl:       cust.Subscription.BaseUrl,
			Insights:      cust.Subscription.Insights,
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
			payloadRepositories[i].Rhsm = common.BoolToPtr(payloadRepository.Rhsm)
		}
		res.PayloadRepositories = &payloadRepositories
	}

	if cust.Filesystem != nil && len(*cust.Filesystem) > 1 {
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
				Key:    &u.SshKey,
				Groups: groups,
			})
		}
		res.Users = &users
	}

	return res
}

func (h *Handlers) CloneCompose(ctx echo.Context, composeId string) error {
	err := h.canUserAccessComposeId(ctx, composeId)
	if err != nil {
		return err
	}

	idHeader, err := getIdentityHeader(ctx)
	if err != nil {
		return err
	}
	imageType, err := h.server.db.GetComposeImageType(composeId, idHeader.Identity.OrgID)
	if err != nil {
		if errors.Is(err, db.ComposeNotFoundError) {
			return echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("Unable to find compose %v", composeId))
		}
		logrus.Errorf("Error querying image type for compose %v: %v", composeId, err)
		return echo.NewHTTPError(http.StatusInternalServerError, "Something went wrong querying the compose")
	}

	var resp *http.Response
	var rawCR json.RawMessage
	if ImageTypes(imageType) == ImageTypesAws || ImageTypes(imageType) == ImageTypesAmi {
		var awsEC2CloneReq AWSEC2Clone
		err = ctx.Bind(&awsEC2CloneReq)
		if err != nil {
			return err
		}

		rawCR, err = json.Marshal(awsEC2CloneReq)
		if err != nil {
			return err
		}

		resp, err = h.server.client.CloneCompose(composeId, composer.AWSEC2CloneCompose{
			Region:            awsEC2CloneReq.Region,
			ShareWithAccounts: awsEC2CloneReq.ShareWithAccounts,
		})
		if err != nil {
			return err
		}
	} else {
		return echo.NewHTTPError(http.StatusBadRequest, "Cloning a compose is only available for AWS composes")
	}

	if resp == nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Something went wrong creating the clone")
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		var cError composer.Error
		err = json.NewDecoder(resp.Body).Decode(&cError)
		if err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, "Unable to parse error returned by image-builder-composer service")
		}
		return echo.NewHTTPError(http.StatusInternalServerError, fmt.Sprintf("image-builder-composer service returned an error: %s", cError.Reason))
	}

	var cloneResponse composer.CloneComposeResponse
	err = json.NewDecoder(resp.Body).Decode(&cloneResponse)
	if err != nil {
		logrus.Errorf("Unable to decode CloneComposeResponse: %v", err)
		return err
	}

	err = h.server.db.InsertClone(composeId, cloneResponse.Id.String(), rawCR)
	if err != nil {
		logrus.Errorf("Error inserting clone into db for compose %v: %v", err, composeId)
		return echo.NewHTTPError(http.StatusInternalServerError, "Something went wrong saving the clone")
	}

	return ctx.JSON(http.StatusCreated, CloneResponse{
		Id: cloneResponse.Id.String(),
	})
}

func (h *Handlers) GetCloneStatus(ctx echo.Context, id string) error {
	idHeader, err := getIdentityHeader(ctx)
	if err != nil {
		return err
	}

	cloneEntry, err := h.server.db.GetClone(id, idHeader.Identity.OrgID)
	if err != nil {
		if errors.Is(err, db.CloneNotFoundError) {
			return echo.NewHTTPError(http.StatusNotFound, err)
		}
		logrus.Errorf("Error querying clone %v: %v", id, err)
		return echo.NewHTTPError(http.StatusInternalServerError, "Something went wrong querying this clone")
	}
	if cloneEntry == nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Requested clone cannot be found")
	}

	resp, err := h.server.client.CloneStatus(id)
	if err != nil {
		logrus.Errorf("Error requesting clone status for clone %v: %v", id, err)
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		var cErr composer.Error
		err = json.NewDecoder(resp.Body).Decode(&cErr)
		if err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, "Unable to parse composer error")
		}
		return echo.NewHTTPError(http.StatusInternalServerError, fmt.Sprintf("Unable to create clone job: %v", cErr.Reason))
	}

	var cloudStat composer.CloneStatus
	err = json.NewDecoder(resp.Body).Decode(&cloudStat)
	if err != nil {
		logrus.Errorf("Unable to decode clone status: %v", err)
		return err
	}

	return ctx.JSON(http.StatusOK, UploadStatus{
		Status:  UploadStatusStatus(cloudStat.Status),
		Type:    UploadTypes(cloudStat.Type),
		Options: cloudStat.Options,
	})
}

func (h *Handlers) GetComposeClones(ctx echo.Context, composeId string, params GetComposeClonesParams) error {
	err := h.canUserAccessComposeId(ctx, composeId)
	if err != nil {
		return err
	}

	idHeader, err := getIdentityHeader(ctx)
	if err != nil {
		return err
	}

	limit := 100
	if params.Limit != nil && *params.Limit > 0 {
		limit = *params.Limit
	}

	offset := 0
	if params.Offset != nil {
		offset = *params.Offset
	}

	cloneEntries, count, err := h.server.db.GetClonesForCompose(composeId, idHeader.Identity.OrgID, limit, offset)
	if err != nil {
		logrus.Errorf("Error querying clones for compose %v: %v", composeId, err)
		return echo.NewHTTPError(http.StatusInternalServerError, "Something went wrong querying clones for this compose")
	}

	var data []ClonesResponseItem
	for _, c := range cloneEntries {
		data = append(data, ClonesResponseItem{
			Id:        c.Id.String(),
			Request:   c.Request,
			CreatedAt: c.CreatedAt.String(),
		})
	}

	lastOffset := count - 1
	if lastOffset < 0 {
		lastOffset = 0
	}

	spec, err := GetSwagger()
	if err != nil {
		return err
	}

	return ctx.JSON(http.StatusOK, ClonesResponse{
		Meta: struct {
			Count int `json:"count"`
		}{
			count,
		},
		Links: struct {
			First string `json:"first"`
			Last  string `json:"last"`
		}{
			fmt.Sprintf("%v/v%v/composes/%v/clones?offset=%v&limit=%v",
				RoutePrefix(), spec.Info.Version, composeId, 0, limit),
			fmt.Sprintf("%v/v%v/composes/%v/clones?offset=%v&limit=%v",
				RoutePrefix(), spec.Info.Version, composeId, lastOffset, limit),
		},
		Data: data,
	})
}
