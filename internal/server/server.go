//go:generate go run github.com/deepmap/oapi-codegen/cmd/oapi-codegen --package=server --generate types,server,spec -o api.go api.yaml
package server

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/osbuild/image-builder/internal/cloudapi"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/labstack/echo/v4"
	"github.com/sirupsen/logrus"
)

type Server struct {
	logger   *logrus.Logger
	echo     *echo.Echo
	client   cloudapi.OsbuildClient
	awsCreds *awsCreds
	orgIds   string
}

type awsCreds struct {
	Region          string
	AccessKeyId     string
	SecretAccessKey string
	S3Bucket        string
}

type Handlers struct {
	server *Server
}

func NewServer(logger *logrus.Logger, client cloudapi.OsbuildClient, region string, keyId string, secret string, s3Bucket string, orgIds string) *Server {
	spec, err := GetSwagger()
	if err != nil {
		panic(err)
	}
	majorVersion := strings.Split(spec.Info.Version, ".")[0]

	s := Server{
		logger,
		echo.New(),
		client,
		&awsCreds{
			region,
			keyId,
			secret,
			s3Bucket,
		},
		orgIds,
	}
	var h Handlers
	h.server = &s
	s.echo.Binder = binder{}
	s.echo.HTTPErrorHandler = s.HTTPErrorHandler
	RegisterHandlers(s.echo.Group(fmt.Sprintf("%s/v%s", RoutePrefix(), majorVersion), s.VerifyIdentityHeader), &h)
	RegisterHandlers(s.echo.Group(fmt.Sprintf("%s/v%s", RoutePrefix(), spec.Info.Version), s.VerifyIdentityHeader), &h)

	/* Used for the liveness- and readinessProbe */
	s.echo.GET("/status", func(c echo.Context) error {
		return h.GetVersion(c)
	})

	return &s
}

func (s *Server) Run(address string) {
	s.logger.Infof("🚀 Starting image-builder server on %v ...\n", address)
	err := s.echo.Start(address)
	if err != nil {
		s.logger.Errorln(fmt.Sprintf("Error starting echo server: %v", err))
	}
}

func (h *Handlers) GetVersion(ctx echo.Context) error {
	spec, err := GetSwagger()
	if err != nil {
		return err
	}
	version := Version{spec.Info.Version}
	return ctx.JSON(http.StatusOK, version)
}

func (h *Handlers) GetOpenapiJson(ctx echo.Context) error {
	spec, err := GetSwagger()
	if err != nil {
		return err
	}
	spec.AddServer(&openapi3.Server{URL: fmt.Sprintf("%s/v%s", RoutePrefix(), spec.Info.Version)})
	return ctx.JSON(http.StatusOK, spec)
}

func (h *Handlers) GetDistributions(ctx echo.Context) error {
	distributions, err := AvailableDistributions()
	if err != nil {
		return err
	}
	return ctx.JSON(http.StatusOK, distributions)
}

func (h *Handlers) GetArchitectures(ctx echo.Context, distribution string) error {
	archs, err := ArchitecturesForImage(distribution)
	if err != nil {
		return err
	}
	return ctx.JSON(http.StatusOK, archs)
}

func (h *Handlers) GetComposeStatus(ctx echo.Context, composeId string) error {
	resp, err := h.server.client.ComposeStatus(composeId)
	if err != nil {
		return err
	}

	if resp.StatusCode == 404 {
		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return err
		}
		return echo.NewHTTPError(http.StatusNotFound, fmt.Sprintf("%s", body))
	}

	var composeStatus cloudapi.ComposeStatus
	err = json.NewDecoder(resp.Body).Decode(&composeStatus)
	if err != nil {
		return err
	}
	return ctx.JSON(http.StatusOK, ComposeStatus{
		Status: composeStatus.Status,
	})
}

func (h *Handlers) ComposeImage(ctx echo.Context) error {
	var composeRequest ComposeRequest
	err := ctx.Bind(&composeRequest)
	if err != nil {
		return err
	}

	if len(composeRequest.ImageRequests) != 1 {
		return echo.NewHTTPError(http.StatusBadRequest, "Exactly one image request should be included")
	}

	if len(composeRequest.ImageRequests[0].UploadRequests) != 1 {
		return echo.NewHTTPError(http.StatusBadRequest, "Exactly one upload request should be included")
	}

	repositories, err := RepositoriesForImage(composeRequest.Distribution, composeRequest.ImageRequests[0].Architecture)
	if err != nil {
		return err
	}

	uploadReq, err := h.server.buildUploadRequest(composeRequest.ImageRequests[0].UploadRequests[0])
	if err != nil {
		return err
	}

	custom, err := buildCustomizations(composeRequest.Customizations)
	if err != nil {
		return err
	}

	cloudCR := cloudapi.ComposeRequest{
		Distribution:   composeRequest.Distribution,
		Customizations: custom,
		ImageRequests: []cloudapi.ImageRequest{
			{
				Architecture: composeRequest.ImageRequests[0].Architecture,
				ImageType:    composeRequest.ImageRequests[0].ImageType,
				Repositories: repositories,
				UploadRequests: []cloudapi.UploadRequest{
					uploadReq,
				},
			},
		},
	}

	resp, err := h.server.client.Compose(cloudCR)
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusCreated {
		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return echo.NewHTTPError(resp.StatusCode, "Failed posting compose request to osbuild-composer")
		}
		return echo.NewHTTPError(resp.StatusCode, fmt.Sprintf("Failed posting compose request to osbuild-composer: %s", body))
	}

	var composeResult cloudapi.ComposeResult
	err = json.NewDecoder(resp.Body).Decode(&composeResult)
	if err != nil {
		return err
	}
	return ctx.JSON(http.StatusCreated, ComposeResponse{
		Id: composeResult.Id,
	})
}

func (s *Server) buildUploadRequest(ur UploadRequest) (cloudapi.UploadRequest, error) {
	if ur.Type == "aws" {
		awsOptions := AWSUploadRequestOptions(ur.Options)
		return cloudapi.UploadRequest{
			Type: ur.Type,
			Options: cloudapi.AWSUploadRequestOptions{
				Ec2: cloudapi.AWSUploadRequestOptionsEc2{
					AccessKeyId:       s.awsCreds.AccessKeyId,
					SecretAccessKey:   s.awsCreds.SecretAccessKey,
					ShareWithAccounts: &awsOptions.ShareWithAccounts,
				},
				S3: cloudapi.AWSUploadRequestOptionsS3{
					AccessKeyId:     s.awsCreds.AccessKeyId,
					SecretAccessKey: s.awsCreds.SecretAccessKey,
					Bucket:          s.awsCreds.S3Bucket,
				},
				Region: s.awsCreds.Region,
			},
		}, nil
	}
	return cloudapi.UploadRequest{}, echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("Unknown UploadRequest type %s", ur.Type))
}

func buildCustomizations(cust *Customizations) (*cloudapi.Customizations, error) {
	if cust == nil {
		return nil, nil
	}

	res := &cloudapi.Customizations{}
	if cust.Subscription != nil {
		res.Subscription = &cloudapi.Subscription{
			ActivationKey: cust.Subscription.ActivationKey,
			BaseUrl:       cust.Subscription.BaseUrl,
			Insights:      cust.Subscription.Insights,
			Organization:  cust.Subscription.Organization,
			ServerUrl:     cust.Subscription.ServerUrl,
		}
	}
	return res, nil
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

type IdentityHeader struct {
	Identity struct {
		Internal struct {
			OrgId *string `json:"org_id"`
		} `json:"internal"`
	} `json:"identity"`
}

// clouddot guidelines requires 404 instead of 403 when unauthorized
func (s *Server) VerifyIdentityHeader(nextHandler echo.HandlerFunc) echo.HandlerFunc {
	return func(ctx echo.Context) error {
		request := ctx.Request()

		idHeaderB64 := request.Header["X-Rh-Identity"]
		if len(idHeaderB64) != 1 {
			return echo.NewHTTPError(http.StatusNotFound, "x-rh-identity header is not present")
		}

		b64Result, err := base64.StdEncoding.DecodeString(idHeaderB64[0])
		if err != nil {
			return echo.NewHTTPError(http.StatusNotFound, "x-rh-identity header doesn't seem to be a base64 string")
		}

		if s.orgIds != "*" {
			var idHeader IdentityHeader
			err = json.Unmarshal([]byte(strings.TrimSuffix(fmt.Sprintf("%s", b64Result), "\n")), &idHeader)
			if err != nil {
				return echo.NewHTTPError(http.StatusNotFound, "x-rh-identity header isn't json")
			}

			if idHeader.Identity.Internal.OrgId == nil {
				return echo.NewHTTPError(http.StatusNotFound, "x-rh-identity header doesn't contain valid orgId")
			}

			if !strings.Contains(s.orgIds, *idHeader.Identity.Internal.OrgId) {
				return echo.NewHTTPError(http.StatusNotFound, "x-rh-identity not authorized")
			}
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

	// Only log internal errors
	if he.Code == http.StatusInternalServerError {
		s.logger.Errorln(fmt.Sprintf("Internal error %v: %v, %v", he.Code, he.Message, err))

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
