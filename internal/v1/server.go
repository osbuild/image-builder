//go:generate go run github.com/deepmap/oapi-codegen/cmd/oapi-codegen --package=v1 --generate types,server,spec -o api.go api.yaml
package v1

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/osbuild/image-builder/internal/common"
	"github.com/osbuild/image-builder/internal/composer"
	"github.com/osbuild/image-builder/internal/db"
	"github.com/osbuild/image-builder/internal/distribution"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/getkin/kin-openapi/routers"
	legacyrouter "github.com/getkin/kin-openapi/routers/legacy"
	"github.com/labstack/echo/v4"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/redhatinsights/identity"
	"github.com/sirupsen/logrus"
)

type Server struct {
	logger    *logrus.Logger
	echo      *echo.Echo
	client    *composer.ComposerClient
	spec      *openapi3.Swagger
	router    routers.Router
	db        db.DB
	aws       AWSConfig
	gcp       GCPConfig
	azure     AzureConfig
	distsDir  string
	quotaFile string
}

type AWSConfig struct {
	Region string
}

type GCPConfig struct {
	Region string
	Bucket string
}

type AzureConfig struct {
	Location string
}

type Handlers struct {
	server *Server
}

func Attach(echoServer *echo.Echo, logger *logrus.Logger, client *composer.ComposerClient, dbase db.DB,
	awsConfig AWSConfig, gcpConfig GCPConfig, azureConfig AzureConfig, distsDir string, quotaFile string) error {
	spec, err := GetSwagger()
	if err != nil {
		return err
	}

	loader := openapi3.NewSwaggerLoader()
	if err := spec.Validate(loader.Context); err != nil {
		return err
	}

	spec.AddServer(&openapi3.Server{URL: fmt.Sprintf("%s/v%s", RoutePrefix(), spec.Info.Version)})

	router, err := legacyrouter.NewRouter(spec)
	if err != nil {
		return err
	}

	majorVersion := strings.Split(spec.Info.Version, ".")[0]

	s := Server{
		logger,
		echoServer,
		client,
		spec,
		router,
		dbase,
		awsConfig,
		gcpConfig,
		azureConfig,
		distsDir,
		quotaFile,
	}
	var h Handlers
	h.server = &s
	s.echo.Binder = binder{}
	s.echo.HTTPErrorHandler = s.HTTPErrorHandler
	RegisterHandlers(s.echo.Group(fmt.Sprintf("%s/v%s", RoutePrefix(), majorVersion), echo.WrapMiddleware(identity.Extractor), echo.WrapMiddleware(identity.BasePolicy), noAssociateAccounts, s.ValidateRequest, common.PrometheusMW), &h)
	RegisterHandlers(s.echo.Group(fmt.Sprintf("%s/v%s", RoutePrefix(), spec.Info.Version), echo.WrapMiddleware(identity.Extractor), echo.WrapMiddleware(identity.BasePolicy), noAssociateAccounts, s.ValidateRequest, common.PrometheusMW), &h)

	/* Used for the livenessProbe */
	s.echo.GET("/status", func(c echo.Context) error {
		return h.GetVersion(c)
	})

	/* Used for the readinessProbe */
	h.server.echo.GET("/ready", func(c echo.Context) error {
		return h.GetReadiness(c)
	})

	h.server.echo.GET("/metrics", echo.WrapHandler(promhttp.Handler()))
	return nil
}

func (h *Handlers) GetVersion(ctx echo.Context) error {
	version := Version{h.server.spec.Info.Version}
	return ctx.JSON(http.StatusOK, version)
}

func (h *Handlers) GetReadiness(ctx echo.Context) error {
	// make sure distributions are available
	distributions, err := distribution.AvailableDistributions(h.server.distsDir, h.server.isEntitled(ctx))
	if err != nil {
		return err
	}
	if len(distributions) == 0 {
		return echo.NewHTTPError(http.StatusInternalServerError, "no distributions defined")
	}

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
	ds, err := distribution.AvailableDistributions(h.server.distsDir, h.server.isEntitled(ctx))
	if err != nil {
		return err
	}

	var distributions DistributionsResponse
	for _, d := range ds {
		distributions = append(distributions, DistributionItem{
			Description: d.Description,
			Name:        d.Name,
		})
	}

	return ctx.JSON(http.StatusOK, distributions)
}

func (h *Handlers) GetArchitectures(ctx echo.Context, distro string) error {
	d, err := distribution.ReadDistribution(h.server.distsDir, distro)
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

// return the Identity Header if there is a valid one in the request
func getIdentityHeader(ctx echo.Context) (*identity.XRHID, error) {
	idHeader, ok := identity.Get(ctx.Request().Context())
	if !ok {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Identity Header missing in request handler")
	}

	return &idHeader, nil
}

// return whether or not the calling context is entitled to consume RHEL content
func (s *Server) isEntitled(ctx echo.Context) bool {
	idh, err := getIdentityHeader(ctx)
	if err != nil {
		return false
	}

	entitled, ok := idh.Entitlements["rhel"]
	// The entitlement should really be present in the identity header, just in case use account
	// number as a fallback
	if !ok {
		// the user's org does not have an associated EBS account number, these
		// are associated when a billing relationship exists, which is a decent
		// proxy for RHEL entitlements
		s.logger.Error("RHEL entitlement not present in identity header")
		return idh.Identity.AccountNumber != ""

	}
	return entitled.IsEntitled
}

// return an error if the user does not have the composeId associated to its OrgID in the DB, nil otherwise
func (h *Handlers) canUserAccessComposeId(ctx echo.Context, composeId string) error {
	idHeader, err := getIdentityHeader(ctx)
	if err != nil {
		return err
	}

	_, err = h.server.db.GetCompose(composeId, idHeader.Identity.OrgID)
	if err != nil {
		if errors.As(err, &db.ComposeNotFoundError) {
			return echo.NewHTTPError(http.StatusNotFound, err)
		} else {
			return err
		}
	}
	return nil
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
			Status:       string(cloudStat.ImageStatus.Status),
			UploadStatus: nil,
		},
	}

	if cloudStat.ImageStatus.UploadStatus != nil {
		status.ImageStatus.UploadStatus = &UploadStatus{
			Status:  string(cloudStat.ImageStatus.UploadStatus.Status),
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
	}
	return cloudOptions
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

	var repositories []composer.Repository
	rs, err := distribution.RepositoriesForArch(h.server.distsDir, string(composeRequest.Distribution), composeRequest.ImageRequests[0].Architecture, h.server.isEntitled(ctx))
	if err != nil {
		return err
	}
	for _, r := range rs {
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
				Baseurl: common.StringToPtr(r.Baseurl),
				Rhsm:    common.BoolToPtr(r.Rhsm),
			})
		}
	}

	uploadOptions, imageType, err := h.server.buildUploadOptions(composeRequest.ImageRequests[0].UploadRequest, composeRequest.ImageRequests[0].ImageType)
	if err != nil {
		return err
	}

	cloudCR := composer.ComposeRequest{
		Distribution:   string(composeRequest.Distribution),
		Customizations: buildCustomizations(composeRequest.Customizations),
		ImageRequest: &composer.ImageRequest{
			Architecture:  composeRequest.ImageRequests[0].Architecture,
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

	err = h.server.db.InsertCompose(composeResult.Id, idHeader.Identity.AccountNumber, idHeader.Identity.Internal.OrgID, composeRequest.ImageName, rawCR)
	if err != nil {
		h.server.logger.Error("Error inserting id into db", err)
		return err
	}

	h.server.logger.Info("Compose result", composeResult)
	return ctx.JSON(http.StatusCreated, ComposeResponse{
		Id: composeResult.Id,
	})
}

func (s *Server) buildUploadOptions(ur UploadRequest, it ImageTypes) (composer.UploadOptions, composer.ImageTypes, error) {
	// HACK deepmap doesn't really support `oneOf`, so marshal and unmarshal into target object
	optionsJSON, err := json.Marshal(ur.Options)
	if err != nil {
		return nil, "", echo.NewHTTPError(http.StatusBadRequest, "Unable to marshal UploadRequestOptions")
	}
	switch ur.Type {
	case UploadTypes_aws:
		var composerImageType composer.ImageTypes
		switch it {
		case ImageTypes_aws:
			fallthrough
		case ImageTypes_ami:
			composerImageType = composer.ImageTypes_aws
		default:
			return nil, "", echo.NewHTTPError(http.StatusBadRequest, "Invalid image type for upload target")
		}
		var awsOptions AWSUploadRequestOptions
		err = json.Unmarshal(optionsJSON, &awsOptions)
		if err != nil {
			return nil, "", echo.NewHTTPError(http.StatusBadRequest, "Unable to unmarshal UploadRequestOptions")
		}
		return composer.AWSEC2UploadOptions{
			Region:            s.aws.Region,
			ShareWithAccounts: awsOptions.ShareWithAccounts,
		}, composerImageType, nil
	case UploadTypes_aws_s3:
		var composerImageType composer.ImageTypes
		switch it {
		case ImageTypes_edge_commit:
			fallthrough
		case ImageTypes_rhel_edge_commit:
			composerImageType = composer.ImageTypes_edge_commit
		case ImageTypes_edge_container:
			composerImageType = composer.ImageTypes_edge_container
		case ImageTypes_edge_installer:
			fallthrough
		case ImageTypes_rhel_edge_installer:
			composerImageType = composer.ImageTypes_edge_installer
		case ImageTypes_guest_image:
			composerImageType = composer.ImageTypes_guest_image
		case ImageTypes_image_installer:
			composerImageType = composer.ImageTypes_image_installer
		case ImageTypes_vsphere:
			composerImageType = composer.ImageTypes_vsphere
		default:
			return nil, "", echo.NewHTTPError(http.StatusBadRequest, "Invalid image type for upload target")
		}
		var awsOptions AWSS3UploadRequestOptions
		err = json.Unmarshal(optionsJSON, &awsOptions)
		if err != nil {
			return nil, "", echo.NewHTTPError(http.StatusBadRequest, "Unable to unmarshal UploadRequestOptions")
		}
		return composer.AWSS3UploadOptions{
			Region: s.aws.Region,
		}, composerImageType, nil
	case UploadTypes_gcp:
		var composerImageType composer.ImageTypes
		switch it {
		case ImageTypes_gcp:
			composerImageType = composer.ImageTypes_gcp
		default:
			return nil, "", echo.NewHTTPError(http.StatusBadRequest, "Invalid image type for upload target")
		}
		var gcpOptions GCPUploadRequestOptions
		err = json.Unmarshal(optionsJSON, &gcpOptions)
		if err != nil {
			return nil, "", echo.NewHTTPError(http.StatusBadRequest, "Unable to unmarshal into GCPUploadRequestOptions")
		}
		return composer.GCPUploadOptions{
			Bucket:            s.gcp.Bucket,
			Region:            s.gcp.Region,
			ShareWithAccounts: &gcpOptions.ShareWithAccounts,
		}, composerImageType, nil
	case UploadTypes_azure:
		var composerImageType composer.ImageTypes
		switch it {
		case ImageTypes_azure:
			fallthrough
		case ImageTypes_vhd:
			composerImageType = composer.ImageTypes_azure
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
			Location:       s.azure.Location,
		}, composerImageType, nil
	default:
		return nil, "", echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("Unknown UploadRequest type %s", ur.Type))
	}
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

	return res
}

func (h *Handlers) GetPackages(ctx echo.Context, params GetPackagesParams) error {
	pkgs, err := distribution.FindPackages(h.server.distsDir, string(params.Distribution), params.Architecture, params.Search, h.server.isEntitled(ctx))
	if err != nil {
		return err
	}
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

func RoutePrefix() string {
	pathPrefix, ok := os.LookupEnv("PATH_PREFIX")
	if !ok {
		pathPrefix = "api"
	}
	appName, ok := os.LookupEnv("APP_NAME")
	if !ok {
		appName = "image-builder"
	}
	return fmt.Sprintf("/%s/%s", pathPrefix, appName)
}

// A simple echo.Binder(), which only accepts application/json, but is more
// strict than echo's DefaultBinder. It does not handle binding query
// parameters either.
type binder struct{}

func (b binder) Bind(i interface{}, ctx echo.Context) error {
	request := ctx.Request()

	contentType := request.Header["Content-Type"]
	if len(contentType) != 1 || contentType[0] != "application/json" {
		return echo.NewHTTPError(http.StatusUnsupportedMediaType, "request must be json-encoded")
	}

	err := json.NewDecoder(request.Body).Decode(i)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("cannot parse request body: %v", err))
	}

	return nil
}

func (s *Server) ValidateRequest(nextHandler echo.HandlerFunc) echo.HandlerFunc {
	return func(ctx echo.Context) error {
		request := ctx.Request()

		route, params, err := s.router.FindRoute(request)
		if err != nil {
			return err
		}

		requestValidationInput := &openapi3filter.RequestValidationInput{
			Request:    request,
			PathParams: params,
			Route:      route,
		}

		context := request.Context()
		if err := openapi3filter.ValidateRequest(context, requestValidationInput); err != nil {
			return echo.NewHTTPError(http.StatusBadRequest, err)
		}
		return nextHandler(ctx)
	}
}

func (s *Server) HTTPErrorHandler(err error, c echo.Context) {
	var errors []HTTPError
	he, ok := err.(*echo.HTTPError)
	if ok {
		if he.Internal != nil {
			if herr, ok := he.Internal.(*echo.HTTPError); ok {
				he = herr
			}
		}
	} else {
		he = &echo.HTTPError{
			Code:    http.StatusInternalServerError,
			Message: http.StatusText(http.StatusInternalServerError),
		}
	}

	internalError := he.Code >= http.StatusInternalServerError && he.Code <= http.StatusNetworkAuthenticationRequired
	if internalError {
		s.logger.Errorln(fmt.Sprintf("Internal error %v: %v, %v", he.Code, he.Message, err))
		if strings.HasSuffix(c.Path(), "/compose") {
			common.ComposeErrors.Inc()
		}
	}

	errors = append(errors, HTTPError{
		Title:  strconv.Itoa(he.Code),
		Detail: fmt.Sprintf("%v", he.Message),
	})

	// Send response
	if !c.Response().Committed {
		if c.Request().Method == http.MethodHead {
			err = c.NoContent(he.Code)
		} else {
			err = c.JSON(he.Code, &HTTPErrorList{
				errors,
			})
		}
		if err != nil {
			s.logger.Errorln(err)
		}
	}
}

func noAssociateAccounts(nextHandler echo.HandlerFunc) echo.HandlerFunc {
	return func(ctx echo.Context) error {
		idh, err := getIdentityHeader(ctx)
		if err != nil {
			return err
		}

		if idh.Identity.Type == "Associate" {
			// Associate account types are not guaranteed to have an associated org_id, these accounts
			// should not be able to access image-builder as long as we don't explicitly enable turnpike
			// access, or another such service forwards them to us. Explicitly reject such accounts for
			// now.
			return echo.NewHTTPError(http.StatusBadRequest, "unsupported account type: 'Associate'")
		}

		return nextHandler(ctx)
	}
}
