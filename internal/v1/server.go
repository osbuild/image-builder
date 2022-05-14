//go:generate go run github.com/deepmap/oapi-codegen/cmd/oapi-codegen --package=v1 --generate types,server,spec -o api.go api.yaml
package v1

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/getkin/kin-openapi/routers"
	legacyrouter "github.com/getkin/kin-openapi/routers/legacy"
	"github.com/labstack/echo/v4"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/sirupsen/logrus"

	"github.com/osbuild/image-builder/internal/common"
	"github.com/osbuild/image-builder/internal/composer"
	"github.com/osbuild/image-builder/internal/db"
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

type IdentityHeader struct {
	Identity struct {
		AccountNumber string `json:"account_number"`
		Internal      struct {
			OrgId string `json:"org_id"`
		} `json:"internal"`
	} `json:"identity"`
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
	RegisterHandlers(s.echo.Group(fmt.Sprintf("%s/v%s", RoutePrefix(), majorVersion), s.verifyIdentityHeader, s.ValidateRequest, common.PrometheusMW), &h)
	RegisterHandlers(s.echo.Group(fmt.Sprintf("%s/v%s", RoutePrefix(), spec.Info.Version), s.verifyIdentityHeader, s.ValidateRequest, common.PrometheusMW), &h)

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

// return the Identity Header if there is a valid one in the request
func (h *Handlers) getIdentityHeader(ctx echo.Context) (*IdentityHeader, error) {
	ih := ctx.Get("IdentityHeader")
	if ih == nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Identity Header missing in request handler")
	}
	idHeader, ok := ih.(IdentityHeader)
	if !ok {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Identity Header invalid in request handler")
	}

	return &idHeader, nil
}

// return an error if the user does not have the composeId associated to its AccountId in the DB, nil otherwise
func (h *Handlers) canUserAccessComposeId(ctx echo.Context, composeId string) error {
	idHeader, err := h.getIdentityHeader(ctx)
	if err != nil {
		return err
	}

	_, err = h.server.db.GetCompose(composeId, idHeader.Identity.AccountNumber)
	if err != nil {
		if errors.As(err, &db.ComposeNotFoundError) {
			return echo.NewHTTPError(http.StatusNotFound, err)
		} else {
			return err
		}
	}
	return nil
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

// clouddot guidelines requires 404 instead of 403 when unauthorized
func (s *Server) verifyIdentityHeader(nextHandler echo.HandlerFunc) echo.HandlerFunc {
	return func(ctx echo.Context) error {
		request := ctx.Request()

		idHeaderB64 := request.Header["X-Rh-Identity"]
		if len(idHeaderB64) != 1 {
			return echo.NewHTTPError(http.StatusNotFound, "Auth header is not present")
		}

		b64Result, err := base64.StdEncoding.DecodeString(idHeaderB64[0])
		if err != nil {
			return echo.NewHTTPError(http.StatusNotFound, "Auth header has incorrect format")
		}

		var idHeader IdentityHeader
		err = json.Unmarshal([]byte(strings.TrimSuffix(fmt.Sprintf("%s", b64Result), "\n")), &idHeader)
		if err != nil {
			return echo.NewHTTPError(http.StatusNotFound, "Auth header has incorrect format")
		}

		if idHeader.Identity.AccountNumber == "" {
			return echo.NewHTTPError(http.StatusNotFound, "The Account Number is missing in the Identity Header")
		}
		ctx.Set("IdentityHeader", idHeader)

		return nextHandler(ctx)
	}
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
