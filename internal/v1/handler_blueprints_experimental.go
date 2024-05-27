package v1

import (
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	openapi_types "github.com/oapi-codegen/runtime/types"
)

func (h *Handlers) CreateBlueprintExperimental(ctx echo.Context) error {
	return h.CreateBlueprint(ctx)
}

func (h *Handlers) GetBlueprintExperimental(ctx echo.Context, id uuid.UUID) error {
	return h.GetBlueprint(ctx, id)
}

func (h *Handlers) UpdateBlueprintExperimental(ctx echo.Context, id uuid.UUID) error {
	return h.UpdateBlueprint(ctx, id)
}

func (h *Handlers) ComposeBlueprintExperimental(ctx echo.Context, id uuid.UUID) error {
	return h.ComposeBlueprint(ctx, id)
}

func (h *Handlers) GetBlueprintsExperimental(ctx echo.Context, params GetBlueprintsExperimentalParams) error {
	return h.GetBlueprints(ctx, GetBlueprintsParams(params))
}

func (h *Handlers) GetBlueprintComposesExperimental(ctx echo.Context, blueprintId openapi_types.UUID, params GetBlueprintComposesExperimentalParams) error {
	return h.GetBlueprintComposes(ctx, blueprintId, GetBlueprintComposesParams(params))
}

func (h *Handlers) DeleteBlueprintExperimental(ctx echo.Context, id uuid.UUID) error {
	return h.DeleteBlueprint(ctx, id)
}
