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

	"github.com/osbuild/image-builder/internal/cloudapi"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/labstack/echo/v4"
)

type Handlers struct{}

func (s *Handlers) GetVersion(ctx echo.Context) error {
	spec, err := GetSwagger()
	if err != nil {
		return err
	}
	version := Version{spec.Info.Version}
	return ctx.JSON(http.StatusOK, version)
}

func (s *Handlers) GetOpenapiJson(ctx echo.Context) error {
	spec, err := GetSwagger()
	if err != nil {
		return err
	}
	spec.AddServer(&openapi3.Server{URL: RoutePrefix()})
	return ctx.JSON(http.StatusOK, spec)
}

func (s *Handlers) GetDistributions(ctx echo.Context) error {
	distributions, err := AvailableDistributions()
	if err != nil {
		return err
	}
	return ctx.JSON(http.StatusOK, distributions)
}

func (s *Handlers) GetArchitectures(ctx echo.Context, distribution string) error {
	archs, err := ArchitecturesForImage(distribution)
	if err != nil {
		return err
	}
	return ctx.JSON(http.StatusOK, archs)
}

func (s *Handlers) GetComposeStatus(ctx echo.Context, composeId string) error {
	socket, ok := os.LookupEnv("OSBUILD_SERVICE")
	if !ok {
		socket = "http://127.0.0.1:80"
	}
	endpoint := "compose/" + composeId

	resp, err := http.Get(fmt.Sprintf("%s/%s", socket, endpoint))
	if err != nil {
		return err
	}

	var composeStatus ComposeStatus
	err = json.NewDecoder(resp.Body).Decode(&composeStatus)
	if err != nil {
		return err
	}
	return ctx.JSON(http.StatusOK, composeStatus)
}

func (s *Handlers) ComposeImage(ctx echo.Context) error {
	socket, ok := os.LookupEnv("OSBUILD_SERVICE")
	if !ok {
		socket = "http://127.0.0.1:80/"
	}

	decoder := json.NewDecoder(ctx.Request().Body)
	var composeRequest cloudapi.ComposeJSONRequestBody
	err := decoder.Decode(&composeRequest)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("Failed decoding the compose request %v", err))
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

	client, err := cloudapi.NewClient(socket)
	if err != nil {
		return err
	}

	resp, err := client.Compose(context.Background(), composeRequest)
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusCreated {
		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return echo.NewHTTPError(resp.StatusCode, "Failed posting compose request to osbuild-composer")
		}
		return echo.NewHTTPError(resp.StatusCode, fmt.Sprintf("Failed posting compose request to osbuild-composer: %v", body))
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
	spec, err := GetSwagger()
	if err != nil {
		panic(err)
	}
	return fmt.Sprintf("/%s/%s/v%s", pathPrefix, appName, spec.Info.Version)
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
		panic("Failed to write response")
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
			return echo.NewHTTPError(http.StatusUnauthorized, "x-rh-identity header is not present")
		}

		return nextHandler(ctx)
	}
}

func Run(address string) {
	fmt.Printf("🚀 Starting image-builder server on %s ...\n", address)
	var s Handlers
	e := echo.New()
	e.Binder = binder{}
	e.Pre(VerifyIdentityHeader)
	RegisterHandlers(e.Group(RoutePrefix()), &s)
	e.Start(address)
}
