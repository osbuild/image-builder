package v1

import (
	"errors"
	"fmt"
	"net/http"
	"regexp"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/getkin/kin-openapi/routers"
	"github.com/labstack/echo/v4"
	"github.com/openshift-online/ocm-sdk-go/metrics"
)

func (s *Server) ValidateRequest(nextHandler echo.HandlerFunc) echo.HandlerFunc {
	return func(ctx echo.Context) error {
		request := ctx.Request()

		route, params, err := s.router.FindRoute(request)
		if err == routers.ErrMethodNotAllowed {
			return echo.NewHTTPError(http.StatusMethodNotAllowed, err)
		} else if err == routers.ErrPathNotFound {
			return echo.NewHTTPError(http.StatusNotFound, err)
		} else if err != nil {
			return echo.NewHTTPError(http.StatusBadRequest, err)
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

// this function replaces dynamic parameter segments in
// a route to reduce metric cardinality. i.e. Any value
// passed to `/compose/{composeId}` will be aggregated
// into the path `/compose/-`
func makeGenericPaths(paths openapi3.Paths) []string {
	var cleaned []string
	r, err := regexp.Compile("{.*?}")
	if err != nil {
		panic(err)
	}
	for path := range paths {
		path = r.ReplaceAllString(path, "-")
		cleaned = append(cleaned, path)
	}
	return cleaned
}

func ocmPrometheusMiddleware(prefix string, paths []string) func(next echo.HandlerFunc) echo.HandlerFunc {
	builder := metrics.NewHandlerWrapper().Subsystem("image_builder_crc")

	for _, path := range paths {
		// this will build the pathTree for the various endpoints
		builder.Path(fmt.Sprintf("%s%s", prefix, path))
	}

	// this function registers all the prometheus metrics
	// and we don't want to rebuild for each request
	metricsWrapper, err := builder.Build()
	if err != nil {
		// programming error
		panic(err)
	}

	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) (err error) {

			// this feels super hacky but needs to be set before`next(c)`
			// otherwise the content-type gets set to application/text
			c.Response().Header().Set("Content-Type", "application/json")

			// we need to get the error codes from the error handling
			// this is one approach to doing it see:
			// -  https://github.com/labstack/echo/discussions/1820#discussioncomment-529428
			// -  https://github.com/labstack/echo/issues/1837#issuecomment-816399630
			err = next(c)
			status := c.Response().Status
			httpErr := new(echo.HTTPError)
			if errors.As(err, &httpErr) {
				status = httpErr.Code
			}

			metricsWrapper.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if status == 0 {
					// handle a zero status code since this is
					// most likely an internal server error
					status = http.StatusInternalServerError
				}
				w.WriteHeader(status)
				c.SetRequest(r)
				c.SetResponse(echo.NewResponse(w, c.Echo()))
			})).ServeHTTP(c.Response(), c.Request())
			return
		}
	}
}
