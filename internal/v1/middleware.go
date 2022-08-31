package v1

import (
	"net/http"

	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/labstack/echo/v4"
)

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
