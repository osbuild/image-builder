package v1

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/osbuild/image-builder/internal/clients/compliance"
	"github.com/osbuild/image-builder/internal/clients/composer"
	"github.com/osbuild/image-builder/internal/clients/content_sources"
	"github.com/osbuild/image-builder/internal/clients/provisioning"
	"github.com/osbuild/image-builder/internal/common"
	"github.com/osbuild/image-builder/internal/distribution"
	"github.com/osbuild/image-builder/internal/unleash"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

func (h *Handlers) ComposeImage(ctx echo.Context) error {
	var composeRequest ComposeRequest
	err := ctx.Bind(&composeRequest)
	if err != nil {
		return err
	}
	composeResponse, err := h.handleCommonCompose(ctx, composeRequest, nil)
	if err != nil {
		ctx.Logger().Errorf("Failed to compose image: %v", err)
		return err
	}

	return ctx.JSON(http.StatusCreated, composeResponse)
}

func (h *Handlers) handleCommonCompose(ctx echo.Context, composeRequest ComposeRequest, blueprintVersionId *uuid.UUID) (ComposeResponse, error) {
	userID, err := h.server.getIdentity(ctx)
	if err != nil {
		return ComposeResponse{}, err
	}

	quotaOk, err := common.CheckQuota(ctx.Request().Context(), userID.OrgID(), h.server.db, h.server.quotaFile)
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

	arch, err := d.Architecture(string(composeRequest.ImageRequests[0].Architecture))
	if err != nil {
		return ComposeResponse{}, err
	}

	var customizations *composer.Customizations
	customizations, err = h.buildCustomizations(ctx, &composeRequest, d)
	if err != nil {
		ctx.Logger().Errorf("Failed building customizations: %v", err)
		return ComposeResponse{}, echo.NewHTTPError(http.StatusInternalServerError, fmt.Sprintf("Unable to build customizations: %v", err))
	}

	var repositories []composer.Repository
	if composeRequest.ImageRequests[0].SnapshotDate != nil {
		repoURLs := []string{}
		for _, r := range arch.Repositories {
			// Assume that non-rh repositories that are defined in the distributions file will not be snapshotted,
			// as there's no guarantee the rpms will be kept forever like in RH repos.
			if slices.Contains(r.ImageTypeTags, string(composeRequest.ImageRequests[0].ImageType)) && !r.Rhsm {
				repositories = append(repositories, composer.Repository{
					Baseurl:  r.Baseurl,
					Metalink: r.Metalink,
					Rhsm:     common.ToPtr(r.Rhsm),
					Gpgkey:   r.GpgKey,
					CheckGpg: r.CheckGpg,
				})
				continue
			}

			if len(r.ImageTypeTags) == 0 || slices.Contains(r.ImageTypeTags, string(composeRequest.ImageRequests[0].ImageType)) {
				repoURLs = append(repoURLs, *r.Baseurl)
			}
		}

		snapshotRepos, _, err := h.buildRepositorySnapshots(ctx, repoURLs, nil, false, *composeRequest.ImageRequests[0].SnapshotDate)
		if err != nil {
			return ComposeResponse{}, err
		}
		repositories = append(repositories, snapshotRepos...)

		// A sanity check to make sure there's a snapshot for each repo
		expected := len(buildRepositories(arch, composeRequest.ImageRequests[0].ImageType))
		if len(repositories) != expected {
			return ComposeResponse{}, fmt.Errorf("no snapshots found for all repositories (found %d, expected %d)", len(repositories), expected)
		}

	} else {
		repositories = buildRepositories(arch, composeRequest.ImageRequests[0].ImageType)
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
	defer closeBody(ctx, resp.Body)
	if resp.StatusCode != http.StatusCreated {
		httpError := echo.NewHTTPError(http.StatusInternalServerError, "Failed posting compose request to osbuild-composer")
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			ctx.Logger().Errorf("unable to parse composer's compose response: %v", err)
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

	err = h.server.db.InsertCompose(ctx.Request().Context(), composeResult.Id, userID.AccountNumber(), userID.Email(), userID.OrgID(), composeRequest.ImageName, rawCR, &clientIdString, blueprintVersionId)
	if err != nil {
		ctx.Logger().Error("Error inserting id into db", err)
		return ComposeResponse{}, err
	}

	ctx.Logger().Info("Compose result", composeResult)

	return ComposeResponse{
		Id: composeResult.Id,
	}, nil
}

func buildRepositories(arch *distribution.Architecture, imageType ImageTypes) []composer.Repository {
	var repositories []composer.Repository
	for _, r := range arch.Repositories {
		// If no image type tags are defined for the repo, add the repo
		if len(r.ImageTypeTags) == 0 || slices.Contains(r.ImageTypeTags, string(imageType)) {
			repositories = append(repositories, composer.Repository{
				Baseurl:  r.Baseurl,
				Metalink: r.Metalink,
				Rhsm:     common.ToPtr(r.Rhsm),
				Gpgkey:   r.GpgKey,
				CheckGpg: r.CheckGpg,
			})
		}
	}
	return repositories
}

func (h *Handlers) buildRepositorySnapshots(ctx echo.Context, repoURLs []string, repoIDs []string, external bool, snapshotDate string) ([]composer.Repository, []composer.CustomRepository, error) {
	date, err := time.Parse(time.RFC3339, snapshotDate)

	if err != nil {
		date, err = time.Parse(time.DateOnly, snapshotDate)
	}

	if err != nil {
		return nil, nil, echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("Snapshot date %s is not in DateOnly (yyyy-mm-dd) or RFC3339 (yyyy-mm-ddThh:mm:ssZ) format", snapshotDate))
	}

	repoMap, err := h.server.csClient.GetRepositories(ctx.Request().Context(), repoURLs, repoIDs, external)
	if err != nil {
		ctx.Logger().Warnf("Unable to get repositories for base urls: %v", err)
		return nil, nil, fmt.Errorf("unable to retrieve repositories: %v", err)
	}

	repoUUIDs := make([]string, 0, len(repoMap))
	for id, repo := range repoMap {
		repoUUIDs = append(repoUUIDs, id)
		if !*repo.Snapshot {
			return nil, nil, echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("Repository %s (id: %s) has snapshotting disabled", *repo.Url, id))
		}
	}

	snapResp, err := h.server.csClient.GetSnapshotsForDate(ctx.Request().Context(), content_sources.ApiListSnapshotByDateRequest{
		Date:            date.UTC().Format(time.RFC3339),
		RepositoryUuids: repoUUIDs,
	})
	if err != nil {
		return nil, nil, err
	}
	defer closeBody(ctx, snapResp.Body)

	if snapResp.StatusCode != http.StatusOK {
		if snapResp.StatusCode != http.StatusUnauthorized {
			body, err := io.ReadAll(snapResp.Body)
			if err != nil {
				return nil, nil, err
			}
			ctx.Logger().Warnf("Unable to resolve snapshots: %s", body)
		}
		return nil, nil, fmt.Errorf("unable to fetch snapshots for date, got %v response", snapResp.StatusCode)
	}

	var csSnapshots content_sources.ApiListSnapshotByDateResponse
	err = json.NewDecoder(snapResp.Body).Decode(&csSnapshots)
	if err != nil {
		return nil, nil, err
	}

	var repositories []composer.Repository
	var customRepositories []composer.CustomRepository
	for _, snap := range *csSnapshots.Data {
		repo, ok := repoMap[*snap.RepositoryUuid]
		if !ok {
			return repositories, customRepositories, fmt.Errorf("returned snapshot %v unexpected repository id %v", *snap.Match.Uuid, *snap.RepositoryUuid)
		}

		composerRepo := composer.Repository{
			Baseurl: common.ToPtr(h.server.csReposURL.JoinPath(*snap.Match.RepositoryPath).String()),
			Rhsm:    common.ToPtr(false),
		}

		if repo.GpgKey != nil && *repo.GpgKey != "" {
			composerRepo.Gpgkey = repo.GpgKey
		}
		if composerRepo.Gpgkey != nil && *composerRepo.Gpgkey != "" {
			composerRepo.CheckGpg = common.ToPtr(true)
		}
		composerRepo.ModuleHotfixes = repo.ModuleHotfixes
		composerRepo.CheckRepoGpg = repo.MetadataVerification
		repositories = append(repositories, composerRepo)

		// Don't enable custom repositories, as they require further setup to be useable.
		customRepo := composer.CustomRepository{
			Id:      *snap.RepositoryUuid,
			Name:    repo.Name,
			Baseurl: &[]string{*snap.Match.Url},
			Enabled: common.ToPtr(false),
		}
		if repo.GpgKey != nil && *repo.GpgKey != "" {
			customRepo.Gpgkey = &[]string{*repo.GpgKey}
			customRepo.CheckGpg = common.ToPtr(true)
		}
		customRepo.ModuleHotfixes = repo.ModuleHotfixes
		customRepo.CheckRepoGpg = repo.MetadataVerification
		customRepositories = append(customRepositories, customRepo)
	}

	ctx.Logger().Debugf("Resolved snapshots: %v", repositories)
	return repositories, customRepositories, nil
}

func (h *Handlers) buildPayloadRepositories(ctx echo.Context, payloadRepos []Repository) ([]composer.Repository, error) {
	res := make([]composer.Repository, len(payloadRepos))

	var repoIDs []string
	for _, repo := range payloadRepos {
		if repo.Id != nil {
			repoIDs = append(repoIDs, *repo.Id)
		}
	}
	repoMap, err := h.server.csClient.GetRepositories(ctx.Request().Context(), nil, repoIDs, true)
	if err != nil {
		return nil, err
	}
	repoMapUpload, err := h.server.csClient.GetRepositories(ctx.Request().Context(), nil, repoIDs, true)
	if err != nil {
		return nil, err
	}
	for k, v := range repoMapUpload {
		repoMap[k] = v
	}

	for i, pyrepo := range payloadRepos {
		var repo content_sources.ApiRepositoryResponse
		if pyrepo.Id != nil {
			if r, ok := repoMap[*pyrepo.Id]; ok {
				repo = r
			}
		}
		if pyrepo.Baseurl != nil {
			res[i].Baseurl = pyrepo.Baseurl
		} else if repo.Uuid != nil {
			// If the repo was found in content sources, its uuid will be set
			baseurl, err := content_sources.GetBaseURL(repo)
			if err != nil {
				return nil, err
			}
			res[i].Baseurl = common.ToPtr(baseurl)
		}

		if pyrepo.CheckGpg != nil {
			res[i].CheckGpg = pyrepo.CheckGpg
		} else if repo.GpgKey != nil && *repo.GpgKey != "" {
			res[i].CheckGpg = common.ToPtr(true)
		}

		if pyrepo.CheckRepoGpg != nil {
			res[i].CheckRepoGpg = pyrepo.CheckRepoGpg
		} else if repo.MetadataVerification != nil && *repo.GpgKey != "" {
			res[i].CheckRepoGpg = repo.MetadataVerification
		}

		if pyrepo.Gpgkey != nil {
			res[i].Gpgkey = pyrepo.Gpgkey
		} else if repo.GpgKey != nil && *repo.GpgKey != "" {
			res[i].Gpgkey = repo.GpgKey
		}

		if pyrepo.ModuleHotfixes != nil {
			res[i].ModuleHotfixes = pyrepo.ModuleHotfixes
		} else if repo.ModuleHotfixes != nil {
			res[i].ModuleHotfixes = repo.ModuleHotfixes
		}

		res[i].IgnoreSsl = pyrepo.IgnoreSsl
		res[i].Metalink = pyrepo.Metalink
		res[i].Mirrorlist = pyrepo.Mirrorlist
		res[i].Rhsm = common.ToPtr(pyrepo.Rhsm)
	}
	return res, nil
}

func (h *Handlers) buildCustomRepositories(ctx echo.Context, custRepos []CustomRepository) ([]composer.CustomRepository, error) {
	res := make([]composer.CustomRepository, len(custRepos))

	var repoIDs []string
	for _, repo := range custRepos {
		repoIDs = append(repoIDs, repo.Id)
	}

	repoMap, err := h.server.csClient.GetRepositories(ctx.Request().Context(), nil, repoIDs, true)
	if err != nil {
		return nil, err
	}

	for i, curepo := range custRepos {
		var repo content_sources.ApiRepositoryResponse
		if r, ok := repoMap[curepo.Id]; ok {
			repo = r
		}

		res[i].Id = curepo.Id
		if curepo.Name != nil {
			res[i].Name = curepo.Name
		} else if repo.Name != nil {
			res[i].Name = repo.Name
		}

		if curepo.Filename != nil {
			res[i].Filename = curepo.Filename
		}

		if curepo.Baseurl != nil {
			res[i].Baseurl = curepo.Baseurl
		} else if repo.Uuid != nil {
			// If the repo was found in content sources, its uuid will be set
			baseurl, err := content_sources.GetBaseURL(repo)
			if err != nil {
				return nil, err
			}
			res[i].Baseurl = common.ToPtr([]string{baseurl})
		}

		if curepo.CheckGpg != nil {
			res[i].CheckGpg = curepo.CheckGpg
		} else if repo.GpgKey != nil && *repo.GpgKey != "" {
			res[i].CheckGpg = common.ToPtr(true)
		}

		if curepo.CheckRepoGpg != nil {
			res[i].CheckRepoGpg = curepo.CheckRepoGpg
		} else if repo.MetadataVerification != nil && *repo.GpgKey != "" {
			res[i].CheckRepoGpg = repo.MetadataVerification
		}

		if curepo.Gpgkey != nil {
			res[i].Gpgkey = curepo.Gpgkey
		} else if repo.GpgKey != nil && *repo.GpgKey != "" {
			res[i].Gpgkey = common.ToPtr([]string{*repo.GpgKey})
		}

		if curepo.SslVerify != nil {
			res[i].SslVerify = curepo.SslVerify
		}
		if curepo.Metalink != nil {
			res[i].Metalink = curepo.Metalink
		}
		if curepo.Mirrorlist != nil {
			res[i].Mirrorlist = curepo.Mirrorlist
		}
		if curepo.Priority != nil {
			res[i].Priority = curepo.Priority
		}
		if curepo.Enabled != nil {
			res[i].Enabled = curepo.Enabled
		}

		if curepo.ModuleHotfixes != nil {
			res[i].ModuleHotfixes = curepo.ModuleHotfixes
		} else if repo.ModuleHotfixes != nil {
			res[i].ModuleHotfixes = repo.ModuleHotfixes
		}

	}
	return res, nil
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
					ctx.Logger().Error(err)
					return uploadOptions, "", echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("Unable to request source: %s", source))
				}
				defer closeBody(ctx, resp.Body)

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
				ctx.Logger().Error(err)
				return uploadOptions, "", echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("Unable to request source: %s", *uo.SourceId))
			}
			defer closeBody(ctx, resp.Body)

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

		var hyperVGen *composer.AzureUploadOptionsHyperVGeneration
		if uo.HyperVGeneration != nil {
			hyperVGen = common.ToPtr(composer.AzureUploadOptionsHyperVGeneration(*uo.HyperVGeneration))
		}

		err = uploadOptions.FromAzureUploadOptions(composer.AzureUploadOptions{
			TenantId:         tenantId,
			SubscriptionId:   subscriptionId,
			ResourceGroup:    uo.ResourceGroup,
			ImageName:        uo.ImageName,
			HyperVGeneration: hyperVGen,
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
	cloudOptions.Ref = ostreeOptions.Ref
	cloudOptions.Url = ostreeOptions.Url
	cloudOptions.Contenturl = ostreeOptions.Contenturl
	cloudOptions.Parent = ostreeOptions.Parent
	cloudOptions.Rhsm = ostreeOptions.Rhsm
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

func (h *Handlers) buildCustomizations(ctx echo.Context, cr *ComposeRequest, d *distribution.DistributionFile) (*composer.Customizations, error) {
	cust := cr.Customizations
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

	snapshotDate := cr.ImageRequests[0].SnapshotDate
	if cust.PayloadRepositories != nil && snapshotDate != nil {
		var repoURLs []string
		var repoIDs []string
		for _, payloadRepository := range *cust.PayloadRepositories {
			if payloadRepository.Baseurl != nil {
				repoURLs = append(repoURLs, *payloadRepository.Baseurl)
			} else if payloadRepository.Id != nil {
				repoIDs = append(repoIDs, *payloadRepository.Id)
			}
		}
		payloadRepositories, _, err := h.buildRepositorySnapshots(ctx, repoURLs, repoIDs, true, *snapshotDate)
		if err != nil {
			return nil, err
		}
		res.PayloadRepositories = &payloadRepositories
	} else if cust.PayloadRepositories != nil && len(*cust.PayloadRepositories) > 0 {
		plrepos, err := h.buildPayloadRepositories(ctx, *cust.PayloadRepositories)
		if err != nil {
			return nil, err
		}
		res.PayloadRepositories = &plrepos
	}

	if cust.CustomRepositories != nil && snapshotDate != nil {
		var repoURLs []string
		var repoIDs []string
		for _, repo := range *cust.CustomRepositories {
			if repo.Baseurl != nil && len(*repo.Baseurl) > 0 {
				repoURLs = append(repoURLs, (*repo.Baseurl)[0])
			} else if repo.Id != "" {
				repoIDs = append(repoIDs, repo.Id)
			}
		}
		_, customRepositories, err := h.buildRepositorySnapshots(ctx, repoURLs, repoIDs, true, *snapshotDate)
		if err != nil {
			return nil, err
		}
		res.CustomRepositories = &customRepositories
	} else if cust.CustomRepositories != nil && len(*cust.CustomRepositories) > 0 {
		custrepos, err := h.buildCustomRepositories(ctx, *cust.CustomRepositories)
		if err != nil {
			return nil, err
		}
		res.CustomRepositories = &custrepos
	}

	if cust.Openscap != nil {
		profile, err := cust.Openscap.AsOpenSCAPProfile()
		if err != nil {
			return nil, err
		}

		if profile.ProfileId != "" {
			res.Openscap = &composer.OpenSCAP{
				ProfileId: profile.ProfileId,
			}
		}
		policy, err := cust.Openscap.AsOpenSCAPCompliance()
		if err != nil {
			ctx.Logger().Error(err.Error())
			return nil, err
		}

		if policy.PolicyId != uuid.Nil {
			if !unleash.Enabled(unleash.CompliancePolicies) {
				return nil, echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("Feature %s not enabled", string(unleash.CompliancePolicies)))
			}

			major, minor, err := d.RHELMajorMinor()
			if err != nil {
				return nil, err
			}

			pdata, err := h.server.complianceClient.PolicyDataForMinorVersion(ctx.Request().Context(), major, minor, policy.PolicyId.String())
			if errors.Is(err, compliance.ErrorAuth) {
				return nil, echo.NewHTTPError(http.StatusForbidden, fmt.Sprintf("User is not authorized to get compliance data for given policy ID (%s)", policy.PolicyId.String()))
			} else if errors.Is(err, compliance.ErrorMajorVersion) {
				return nil, echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("Compliance policy (%s) does not support requested major version %d", policy.PolicyId.String(), major))
			} else if errors.Is(err, compliance.ErrorNotFound) {
				return nil, echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("Compliance policy (%s) or its tailorings weren't found", policy.PolicyId.String()))
			} else if err != nil {
				return nil, err
			}

			res.Openscap = &composer.OpenSCAP{
				ProfileId: pdata.ProfileID,
				PolicyId:  &policy.PolicyId,
				JsonTailoring: &composer.OpenSCAPJSONTailoring{
					ProfileId: pdata.ProfileID,
					Filepath:  "/etc/osbuild/openscap-tailoring.json",
				},
			}
			res.Directories = &[]composer.Directory{
				{
					Path: "/etc/osbuild",
				},
			}
			res.Files = &[]composer.File{
				{
					Path: "/etc/osbuild/openscap-tailoring.json",
					Data: common.ToPtr(string(pdata.TailoringData)),
				},
			}
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
			u.RedactPassword()
			users = append(users, composer.User{
				Name:     u.Name,
				Key:      u.SshKey,
				Password: u.Password,
				Groups:   u.Groups,
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
		var mode composer.BlueprintCustomizationsPartitioningMode
		switch *cust.PartitioningMode {
		case AutoLvm:
			mode = composer.BlueprintCustomizationsPartitioningModeAutoLvm
		case Lvm:
			mode = composer.BlueprintCustomizationsPartitioningModeLvm
		case Raw:
			mode = composer.BlueprintCustomizationsPartitioningModeRaw
		}
		res.PartitioningMode = common.ToPtr(composer.CustomizationsPartitioningMode(mode))
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
		// OpenSCAP tailoring creates a directory
		if res.Directories != nil && len(*res.Directories) > 0 {
			res.Directories = common.ToPtr(append(*res.Directories, dirs...))
		} else {
			res.Directories = &dirs
		}
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

			if f.Data != nil && f.DataEncoding != nil && *f.DataEncoding == "base64" {
				buf, err := base64.StdEncoding.DecodeString(*f.Data)
				if err != nil {
					return nil, err
				}
				f.Data = common.ToPtr(string(buf))
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
		// OpenSCAP tailoring creates a file
		if res.Files != nil && len(*res.Files) > 0 {
			res.Files = common.ToPtr(append(*res.Files, files...))
		} else {
			res.Files = &files
		}
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
			Masked:   cust.Services.Masked,
		}
	}

	// we need to explicitly add 'rhcd' to the enabled services
	// if openscap and subscription customizations are set, otherwise
	// the insights-client doesn't register properly
	if cust.Subscription != nil && cust.Subscription.Insights && cust.Openscap != nil {
		if res.Services == nil {
			res.Services = &composer.Services{}
		}
		if res.Services.Enabled == nil {
			res.Services.Enabled = &[]string{}
		}
		if !slices.Contains(*res.Services.Enabled, "rhcd") {
			*res.Services.Enabled = append(*res.Services.Enabled, "rhcd")
		}
	}

	if cust.Firewall != nil {
		res.Firewall = &composer.FirewallCustomization{
			Ports: cust.Firewall.Ports,
		}

		if cust.Firewall.Services != nil {
			res.Firewall.Services = &composer.FirewallServices{
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

	if cust.Installer != nil {
		res.Installer = &composer.Installer{
			SudoNopasswd: cust.Installer.SudoNopasswd,
			Unattended:   cust.Installer.Unattended,
		}
	}

	res.Hostname = cust.Hostname

	return res, nil
}
