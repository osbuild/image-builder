package v1

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/osbuild/image-builder/internal/common"
	"github.com/osbuild/image-builder/internal/composer"
	"github.com/osbuild/image-builder/internal/db"
	"github.com/osbuild/image-builder/internal/provisioning"

	"github.com/labstack/echo/v4"
	"github.com/sirupsen/logrus"
)

const (
	ComposeRunningOrFailedError = "IMAGE-BUILDER-COMPOSER-31"

	// 64 GiB
	FSMaxSize = 68719476736
)

func (h *Handlers) GetVersion(ctx echo.Context) error {
	version := Version{h.server.spec.Info.Version}
	return ctx.JSON(http.StatusOK, version)
}

func (h *Handlers) GetReadiness(ctx echo.Context) error {
	resp, err := h.server.cClient.OpenAPI()
	if err != nil {
		return err
	}
	defer closeBody(resp.Body)

	if resp.StatusCode != 200 {
		body, err := io.ReadAll(resp.Body)
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
	idHeader, err := getIdentityHeader(ctx)
	if err != nil {
		return err
	}

	var distributions DistributionsResponse
	for k, d := range dr.Map() {
		if d.IsRestricted() {
			allowOk, err := h.server.allowList.IsAllowed(idHeader.Identity.Internal.OrgID, d.Distribution.Name)
			if err != nil {
				return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
			}
			if !allowOk {
				continue
			}
		}
		distributions = append(distributions, DistributionItem{
			Description: d.Distribution.Description,
			Name:        k,
		})
	}

	return ctx.JSON(http.StatusOK, distributions)
}

func (h *Handlers) GetArchitectures(ctx echo.Context, distro Distributions) error {
	d, err := h.server.getDistro(ctx, distro)
	if err != nil {
		return err
	}

	var archs Architectures
	var reposArchX86 []Repository
	var reposAarch64 []Repository
	if d.ArchX86 != nil {
		for _, r := range d.ArchX86.Repositories {
			if r.ImageTypeTags == nil {
				reposArchX86 = append(reposArchX86,
					Repository{
						Baseurl:  r.Baseurl,
						Metalink: r.Metalink,
						Rhsm:     r.Rhsm,
					})
			}
		}
	}

	if d.Aarch64 != nil {
		for _, r := range d.Aarch64.Repositories {
			if r.ImageTypeTags == nil {
				reposAarch64 = append(reposAarch64,
					Repository{
						Baseurl:  r.Baseurl,
						Metalink: r.Metalink,
						Rhsm:     r.Rhsm,
					})
			}
		}
	}

	if d.ArchX86 != nil {
		archs = append(archs, ArchitectureItem{
			Arch:         "x86_64",
			ImageTypes:   d.ArchX86.ImageTypes,
			Repositories: reposArchX86,
		})
	}
	if d.Aarch64 != nil {
		archs = append(archs, ArchitectureItem{
			Arch:         "aarch64",
			ImageTypes:   d.Aarch64.ImageTypes,
			Repositories: reposAarch64,
		})
	}

	return ctx.JSON(http.StatusOK, archs)
}

func (h *Handlers) GetPackages(ctx echo.Context, params GetPackagesParams) error {
	d, err := h.server.getDistro(ctx, params.Distribution)
	if err != nil {
		return err
	}

	arch, err := d.Architecture(string(params.Architecture))
	if err != nil {
		return err
	}

	pkgs := arch.FindPackages(params.Search)
	packages := []Package{}
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
		if *params.Offset > len(packages) {
			offset = len(packages)
		} else if *params.Offset > 0 {
			offset = *params.Offset
		}
	}

	upto := offset + limit
	if upto > len(packages) {
		upto = len(packages)
	}

	lastOffset := len(packages) - 1
	if lastOffset < 0 {
		lastOffset = 0
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
			fmt.Sprintf("%v/v%v/packages?search=%v&distribution=%v&architecture=%v&offset=0&limit=%v",
				RoutePrefix(), h.server.spec.Info.Version, params.Search, params.Distribution, params.Architecture, limit),
			fmt.Sprintf("%v/v%v/packages?search=%v&distribution=%v&architecture=%v&offset=%v&limit=%v",
				RoutePrefix(), h.server.spec.Info.Version, params.Search, params.Distribution, params.Architecture, lastOffset, limit),
		},
		Data: packages[offset:upto],
	})
}

func (h *Handlers) GetComposeStatus(ctx echo.Context, composeId uuid.UUID) error {
	composeEntry, err := h.getComposeByIdAndOrgId(ctx, composeId)
	if err != nil {
		return err
	}

	resp, err := h.server.cClient.ComposeStatus(composeId)
	if err != nil {
		return err
	}
	defer closeBody(resp.Body)

	if resp.StatusCode == http.StatusNotFound {
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return err
		}
		// Composes can get deleted in composer, usually when the image is expired
		return echo.NewHTTPError(http.StatusNotFound, string(body))
	} else if resp.StatusCode != http.StatusOK {
		httpError := echo.NewHTTPError(http.StatusInternalServerError, "Failed querying compose status")
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			ctx.Logger().Errorf("Unable to parse composer's compose response: %v", err)
		} else {
			_ = httpError.SetInternal(fmt.Errorf("%s", body))
		}
		return httpError
	}

	var composeRequest ComposeRequest
	err = json.Unmarshal(composeEntry.Request, &composeRequest)
	if err != nil {
		return err
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
		Request: composeRequest,
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

func (h *Handlers) DeleteCompose(ctx echo.Context, composeId uuid.UUID) error {
	idHeader, err := getIdentityHeader(ctx)
	if err != nil {
		return err
	}

	err = h.server.db.DeleteCompose(composeId, idHeader.Identity.OrgID)
	if err != nil {
		if errors.Is(err, db.ComposeNotFoundError) {
			return echo.NewHTTPError(http.StatusNotFound, err)
		}
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	return ctx.NoContent(http.StatusOK)
}

func (h *Handlers) GetComposeMetadata(ctx echo.Context, composeId uuid.UUID) error {
	err := h.canUserAccessComposeId(ctx, composeId)
	if err != nil {
		return err
	}

	resp, err := h.server.cClient.ComposeMetadata(composeId)
	if err != nil {
		return err
	}
	defer closeBody(resp.Body)

	if resp.StatusCode == http.StatusNotFound {
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return err
		}
		return echo.NewHTTPError(http.StatusNotFound, string(body))
	} else if resp.StatusCode != http.StatusOK {
		httpError := echo.NewHTTPError(http.StatusInternalServerError, "Failed querying compose status")
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			ctx.Logger().Errorf("Unable to parse composer's compose response: %v", err)
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

// return compose from the database or error when user does not have composeId associated to its OrgId in the DB
func (h *Handlers) getComposeByIdAndOrgId(ctx echo.Context, composeId uuid.UUID) (*db.ComposeEntry, error) {
	idHeader, err := getIdentityHeader(ctx)
	if err != nil {
		return nil, err
	}

	composeEntry, err := h.server.db.GetCompose(composeId, idHeader.Identity.OrgID)
	if err != nil {
		if errors.Is(err, db.ComposeNotFoundError) {
			return nil, echo.NewHTTPError(http.StatusNotFound, err)
		} else {
			return nil, err
		}
	}
	return composeEntry, nil
}

// return an error if the user does not have the composeId associated to its OrgID in the DB, nil otherwise
func (h *Handlers) canUserAccessComposeId(ctx echo.Context, composeId uuid.UUID) error {
	_, err := h.getComposeByIdAndOrgId(ctx, composeId)
	return err
}

func convertIgnoreImageTypeToSlice(ignoreImageTypes *[]ImageTypes) []string {
	if ignoreImageTypes == nil {
		return nil
	}

	stringSlice := make([]string, len(*ignoreImageTypes))
	for i, imageType := range *ignoreImageTypes {
		stringSlice[i] = string(imageType)
	}

	return stringSlice
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
	ignoreImageTypeStrings := convertIgnoreImageTypeToSlice(params.IgnoreImageTypes)

	// composes in the last 14 days
	composes, count, err := h.server.db.GetComposes(idHeader.Identity.OrgID, (time.Hour * 24 * 14), limit, offset, ignoreImageTypeStrings)
	if err != nil {
		return err
	}

	data := []ComposesResponseItem{}
	for _, c := range composes {
		var cmpr ComposeRequest
		err = json.Unmarshal(c.Request, &cmpr)
		if err != nil {
			return err
		}
		data = append(data, ComposesResponseItem{
			CreatedAt: c.CreatedAt.Format(time.RFC3339),
			Id:        c.Id,
			ImageName: c.ImageName,
			Request:   cmpr,
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
			fmt.Sprintf("%v/v%v/composes?offset=0&limit=%v",
				RoutePrefix(), spec.Info.Version, limit),
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

	d, err := h.server.getDistro(ctx, composeRequest.Distribution)
	if err != nil {
		return err
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
				Rhsm:     common.ToPtr(r.Rhsm),
			})
		}
	}

	uploadOptions, imageType, err := h.buildUploadOptions(ctx, composeRequest.ImageRequests[0].UploadRequest, composeRequest.ImageRequests[0].ImageType)
	if err != nil {
		return err
	}

	err = validateComposeRequest(&composeRequest)
	if err != nil {
		return err
	}

	distro := d.Distribution.Name
	if d.Distribution.ComposerName != nil {
		distro = *d.Distribution.ComposerName
	}

	cloudCR := composer.ComposeRequest{
		Distribution:   distro,
		Customizations: buildCustomizations(composeRequest.Customizations),
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
		return err
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
				return httpError
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

	err = h.server.db.InsertCompose(composeResult.Id, idHeader.Identity.AccountNumber, idHeader.Identity.User.Email, idHeader.Identity.Internal.OrgID, composeRequest.ImageName, rawCR)
	if err != nil {
		logrus.Error("Error inserting id into db", err)
		return err
	}

	ctx.Logger().Info("Compose result", composeResult)

	return ctx.JSON(http.StatusCreated, ComposeResponse{
		Id: composeResult.Id,
	})
}

func (h *Handlers) buildUploadOptions(ctx echo.Context, ur UploadRequest, it ImageTypes) (composer.UploadOptions, composer.ImageTypes, error) {
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

		if (awsOptions.ShareWithAccounts == nil || len(*awsOptions.ShareWithAccounts) == 0) && (awsOptions.ShareWithSources == nil || len(*awsOptions.ShareWithSources) == 0) {
			return nil, "", echo.NewHTTPError(http.StatusBadRequest, "Expected at least one source or account to share the image with")
		}

		var shareWithAccounts []string
		if awsOptions.ShareWithAccounts != nil {
			shareWithAccounts = append(shareWithAccounts, *awsOptions.ShareWithAccounts...)
		}

		if awsOptions.ShareWithSources != nil {
			for _, source := range *awsOptions.ShareWithSources {
				resp, err := h.server.pClient.GetUploadInfo(ctx.Request().Context(), source)
				if err != nil {
					logrus.Error(err)
					return nil, "", echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("Unable to request source: %s", source))
				}
				defer closeBody(resp.Body)

				var uploadInfo provisioning.V1SourceUploadInfoResponse
				err = json.NewDecoder(resp.Body).Decode(&uploadInfo)
				if err != nil {
					return nil, "", echo.NewHTTPError(http.StatusInternalServerError, fmt.Sprintf("Unable to resolve source: %s", source))
				}

				if uploadInfo.Aws == nil || uploadInfo.Aws.AccountId == nil || len(*uploadInfo.Aws.AccountId) != 12 {
					return nil, "", echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("Unable to resolve source %s to an aws account id", source))
				}

				ctx.Logger().Info(fmt.Sprintf("Resolved source %s, to account id %s", strings.Replace(source, "\n", "", -1), *uploadInfo.Aws.AccountId))
				shareWithAccounts = append(shareWithAccounts, *uploadInfo.Aws.AccountId)
			}
		}

		return composer.AWSEC2UploadOptions{
			Region:            h.server.aws.Region,
			ShareWithAccounts: shareWithAccounts,
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
		case ImageTypesVsphereOva:
			composerImageType = composer.ImageTypesVsphereOva
		case ImageTypesWsl:
			composerImageType = composer.ImageTypesWsl
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
			Bucket:            &h.server.gcp.Bucket,
			Region:            h.server.gcp.Region,
			ShareWithAccounts: gcpOptions.ShareWithAccounts,
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

		if (azureOptions.SourceId == nil && (azureOptions.TenantId == nil || azureOptions.SubscriptionId == nil)) ||
			(azureOptions.SourceId != nil && (azureOptions.TenantId != nil || azureOptions.SubscriptionId != nil)) {
			return nil, "", echo.NewHTTPError(http.StatusBadRequest, "Request must contain either (1) a source id, and no tenant or subscription ids or (2) tenant and subscription ids, and no source id.")
		}

		var tenantId string
		var subscriptionId string

		if azureOptions.SourceId == nil {
			tenantId = *azureOptions.TenantId
			subscriptionId = *azureOptions.SubscriptionId
		}

		if azureOptions.SourceId != nil {
			resp, err := h.server.pClient.GetUploadInfo(ctx.Request().Context(), *azureOptions.SourceId)
			if err != nil {
				logrus.Error(err)
				return nil, "", echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("Unable to request source: %s", *azureOptions.SourceId))
			}
			defer closeBody(resp.Body)

			var uploadInfo provisioning.V1SourceUploadInfoResponse
			err = json.NewDecoder(resp.Body).Decode(&uploadInfo)

			if err != nil {
				return nil, "", echo.NewHTTPError(http.StatusInternalServerError, fmt.Sprintf("Unable to resolve source: %s", *azureOptions.SourceId))
			}

			if uploadInfo.Azure == nil || uploadInfo.Azure.TenantId == nil || uploadInfo.Azure.SubscriptionId == nil {
				return nil, "", echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("Unable to resolve source %s to an Azure tenant id or subscription id. ", *azureOptions.SourceId))
			}

			ctx.Logger().Info(fmt.Sprintf("Resolved source %s to tenant id %s and subscription id %s", *azureOptions.SourceId, *uploadInfo.Azure.TenantId, *uploadInfo.Azure.SubscriptionId))
			tenantId = *uploadInfo.Azure.TenantId
			subscriptionId = *uploadInfo.Azure.SubscriptionId
		}

		uploadOptions := composer.AzureUploadOptions{
			TenantId:       tenantId,
			SubscriptionId: subscriptionId,
			ResourceGroup:  azureOptions.ResourceGroup,
			ImageName:      azureOptions.ImageName,
		}
		return uploadOptions, composerImageType, nil
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
				Key:    &u.SshKey,
				Groups: groups,
			})
		}
		res.Users = &users
	}

	return res
}

func (h *Handlers) CloneCompose(ctx echo.Context, composeId uuid.UUID) error {
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
		ctx.Logger().Errorf("Error querying image type for compose %v: %v", composeId, err)
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

		var shareWithAccounts []string
		if awsEC2CloneReq.ShareWithAccounts != nil {
			shareWithAccounts = append(shareWithAccounts, *awsEC2CloneReq.ShareWithAccounts...)
		}

		if awsEC2CloneReq.ShareWithSources != nil {
			for _, source := range *awsEC2CloneReq.ShareWithSources {
				resp, err := h.server.pClient.GetUploadInfo(ctx.Request().Context(), source)
				if err != nil {
					logrus.Error(err)
					return echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("Unable to request source: %s", source))
				}
				defer closeBody(resp.Body)

				var uploadInfo provisioning.V1SourceUploadInfoResponse
				err = json.NewDecoder(resp.Body).Decode(&uploadInfo)
				if err != nil {
					return echo.NewHTTPError(http.StatusInternalServerError, fmt.Sprintf("Unable to resolve source: %s", source))
				}

				if uploadInfo.Aws == nil || uploadInfo.Aws.AccountId == nil || len(*uploadInfo.Aws.AccountId) != 12 {
					return echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("Unable to resolve source %s to an aws account id: %v", source, uploadInfo.Aws.AccountId))
				}

				logrus.Info(fmt.Sprintf("Resolved source %s, to account id %s", strings.Replace(source, "\n", "", -1), *uploadInfo.Aws.AccountId))
				shareWithAccounts = append(shareWithAccounts, *uploadInfo.Aws.AccountId)
			}
		}

		resp, err = h.server.cClient.CloneCompose(composeId, composer.AWSEC2CloneCompose{
			Region:            awsEC2CloneReq.Region,
			ShareWithAccounts: &shareWithAccounts,
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
	defer closeBody(resp.Body)
	if resp.StatusCode != http.StatusCreated {
		var cError composer.Error
		err = json.NewDecoder(resp.Body).Decode(&cError)
		if err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, "Unable to parse error returned by image-builder-composer service")
		}
		if cError.Code == ComposeRunningOrFailedError {
			return echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("image-builder-composer compose failed: %s", cError.Reason))
		}
		return echo.NewHTTPError(http.StatusInternalServerError, fmt.Sprintf("image-builder-composer service returned an error: %s", cError.Reason))
	}

	var cloneResponse composer.CloneComposeResponse
	err = json.NewDecoder(resp.Body).Decode(&cloneResponse)
	if err != nil {
		ctx.Logger().Errorf("Unable to decode CloneComposeResponse: %v", err)
		return err
	}

	err = h.server.db.InsertClone(composeId, cloneResponse.Id, rawCR)
	if err != nil {
		ctx.Logger().Errorf("Error inserting clone into db for compose %v: %v", err, composeId)
		return echo.NewHTTPError(http.StatusInternalServerError, "Something went wrong saving the clone")
	}

	return ctx.JSON(http.StatusCreated, CloneResponse{
		Id: cloneResponse.Id,
	})
}

func (h *Handlers) GetCloneStatus(ctx echo.Context, id uuid.UUID) error {
	idHeader, err := getIdentityHeader(ctx)
	if err != nil {
		return err
	}

	cloneEntry, err := h.server.db.GetClone(id, idHeader.Identity.OrgID)
	if err != nil {
		if errors.Is(err, db.CloneNotFoundError) {
			return echo.NewHTTPError(http.StatusNotFound, err)
		}
		ctx.Logger().Errorf("Error querying clone %v: %v", id, err)
		return echo.NewHTTPError(http.StatusInternalServerError, "Something went wrong querying this clone")
	}
	if cloneEntry == nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Requested clone cannot be found")
	}

	resp, err := h.server.cClient.CloneStatus(id)
	if err != nil {
		ctx.Logger().Errorf("Error requesting clone status for clone %v: %v", id, err)
		return err
	}
	defer closeBody(resp.Body)
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
		ctx.Logger().Errorf("Unable to decode clone status: %v", err)
		return err
	}

	return ctx.JSON(http.StatusOK, UploadStatus{
		Status:  UploadStatusStatus(cloudStat.Status),
		Type:    UploadTypes(cloudStat.Type),
		Options: cloudStat.Options,
	})
}

func (h *Handlers) GetComposeClones(ctx echo.Context, composeId uuid.UUID, params GetComposeClonesParams) error {
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
		ctx.Logger().Errorf("Error querying clones for compose %v: %v", composeId, err)
		return echo.NewHTTPError(http.StatusInternalServerError, "Something went wrong querying clones for this compose")
	}

	data := []ClonesResponseItem{}
	for _, c := range cloneEntries {
		var cr CloneRequest
		err = json.Unmarshal(c.Request, &cr)
		if err != nil {
			return echo.NewHTTPError(
				http.StatusInternalServerError, "Something went wrong querying clones for this compose")
		}
		data = append(data, ClonesResponseItem{
			Id:        c.Id,
			ComposeId: composeId,
			Request:   cr,
			CreatedAt: c.CreatedAt.Format(time.RFC3339),
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

func closeBody(body io.Closer) {
	err := body.Close()
	if err != nil {
		logrus.Errorf("closing response body failed: %v", err)
	}
}
