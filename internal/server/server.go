//go:generate go run github.com/deepmap/oapi-codegen/cmd/oapi-codegen --package=cloudapi --generate types,client -o ../cloudapi/cloudapi_client.go ../cloudapi/cloudapi_client.yml
//go:generate go run github.com/deepmap/oapi-codegen/cmd/oapi-codegen --package=server --generate types,server,spec,client -o api.go api.yaml
package server

import (
	"context"
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
	logger *logrus.Logger
	echo   *echo.Echo
	client *cloudapi.OsbuildClient
}

type Handlers struct {
	server *Server
}

func NewServer(logger *logrus.Logger, client *cloudapi.OsbuildClient) *Server {
	spec, err := GetSwagger()
	if err != nil {
		panic(err)
	}
	majorVersion := strings.Split(spec.Info.Version, ".")[0]

	s := Server{logger, echo.New(), client}
	var h Handlers
	h.server = &s
	s.echo.Binder = binder{}
	s.echo.HTTPErrorHandler = s.HTTPErrorHandler
	s.echo.Pre(VerifyIdentityHeader)
	RegisterHandlers(s.echo.Group(fmt.Sprintf("%s/v%s", RoutePrefix(), majorVersion)), &h)
	RegisterHandlers(s.echo.Group(fmt.Sprintf("%s/v%s", RoutePrefix(), spec.Info.Version)), &h)
	return &s
}

func (s *Server) Run(address string) {
	s.logger.Infoln("🚀 Starting image-builder server on %s ...\n", address)
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
	client, err := h.server.client.Get()
	if err != nil {
		return err
	}

	resp, err := client.ComposeStatus(context.Background(), composeId)
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

	var composeStatus ComposeStatus
	err = json.NewDecoder(resp.Body).Decode(&composeStatus)
	if err != nil {
		return err
	}
	return ctx.JSON(http.StatusOK, composeStatus)
}

func (h *Handlers) ComposeImage(ctx echo.Context) error {
	composeRequest := &cloudapi.ComposeJSONRequestBody{}
	err := ctx.Bind(composeRequest)
	if err != nil {
		return err
	}

	if len(composeRequest.ImageRequests) != 1 {
		return echo.NewHTTPError(http.StatusBadRequest, "Exactly one image request should be included")
	}

	if len(composeRequest.ImageRequests[0].Repositories) != 0 {
		return echo.NewHTTPError(http.StatusBadRequest, "Repositories are specified by image-builder itself")
	}

	repositories, err := RepositoriesForImage(composeRequest.Distribution, composeRequest.ImageRequests[0].Architecture)
	if err != nil {
		return err

	}
	composeRequest.ImageRequests[0].Repositories = repositories

	client, err := h.server.client.Get()
	if err != nil {
		return err
	}

	resp, err := client.Compose(context.Background(), *composeRequest)
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

	var composeResponse ComposeResponse
	err = json.NewDecoder(resp.Body).Decode(&composeResponse)
	if err != nil {
		return err
	}
	return ctx.JSON(http.StatusCreated, composeResponse)
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

func VerifyIdentityHeader(nextHandler echo.HandlerFunc) echo.HandlerFunc {
	return func(ctx echo.Context) error {
		request := ctx.Request()

		// For now just check it's there, in future we might want to
		// decode the b64 string and check a specific entitlement
		identityHeader := request.Header["X-Rh-Identity"]
		if len(identityHeader) != 1 {
			// clouddot guidelines requires 404 instead of 403
			return echo.NewHTTPError(http.StatusNotFound, "x-rh-identity header is not present")
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
