package v1

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"time"

	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5/pgconn"
	openapi_types "github.com/oapi-codegen/runtime/types"
	"github.com/osbuild/image-builder/internal/common"
	"github.com/osbuild/image-builder/internal/db"

	"slices"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

var (
	blueprintNameRegex         = regexp.MustCompile(`\S+`)
	blueprintInvalidNameDetail = "The blueprint name must contain at least two characters."
)

type BlueprintBody struct {
	Customizations Customizations `json:"customizations"`
	Distribution   Distributions  `json:"distribution"`
	ImageRequests  []ImageRequest `json:"image_requests"`
}

func BlueprintFromAPI(cbr CreateBlueprintRequest) BlueprintBody {
	return BlueprintBody{
		Customizations: cbr.Customizations,
		Distribution:   cbr.Distribution,
		ImageRequests:  cbr.ImageRequests,
	}
}

func BlueprintFromEntry(be *db.BlueprintEntry) (BlueprintBody, error) {
	var result BlueprintBody
	err := json.Unmarshal(be.Body, &result)
	if err != nil {
		return BlueprintBody{}, err
	}

	return result, nil
}

func (h *Handlers) CreateBlueprint(ctx echo.Context) error {
	userID, err := h.server.getIdentity(ctx)
	if err != nil {
		return err
	}

	var blueprintRequest CreateBlueprintRequest
	err = ctx.Bind(&blueprintRequest)
	if err != nil {
		return err
	}

	blueprint := BlueprintFromAPI(blueprintRequest)
	body, err := json.Marshal(blueprint)
	if err != nil {
		return err
	}

	if !blueprintNameRegex.MatchString(blueprintRequest.Name) {
		return ctx.JSON(http.StatusUnprocessableEntity, HTTPErrorList{
			Errors: []HTTPError{{
				Title:  "Invalid blueprint name",
				Detail: blueprintInvalidNameDetail,
			}},
		})
	}

	id := uuid.New()
	versionId := uuid.New()
	ctx.Logger().Infof("Inserting blueprint: %s (%s), for orgID: %s and account: %s", blueprintRequest.Name, id, userID.OrgID(), userID.AccountNumber())
	desc := ""
	if blueprintRequest.Description != nil {
		desc = *blueprintRequest.Description
	}
	err = h.server.db.InsertBlueprint(ctx.Request().Context(), id, versionId, userID.OrgID(), userID.AccountNumber(), blueprintRequest.Name, desc, body)
	if err != nil {
		ctx.Logger().Errorf("Error inserting id into db: %s", err.Error())

		var e *pgconn.PgError
		if errors.As(err, &e) && e.Code == pgerrcode.UniqueViolation {
			return ctx.JSON(http.StatusUnprocessableEntity, HTTPErrorList{
				Errors: []HTTPError{{
					Title:  "Name not unique",
					Detail: "A blueprint with the same name already exists.",
				}},
			})
		}
		return err
	}
	ctx.Logger().Infof("Inserted blueprint %s", id)
	return ctx.JSON(http.StatusCreated, ComposeResponse{
		Id: id,
	})
}

func (h *Handlers) GetBlueprint(ctx echo.Context, id openapi_types.UUID) error {
	userID, err := h.server.getIdentity(ctx)
	if err != nil {
		return err
	}

	ctx.Logger().Infof("Fetching blueprint %s", id)
	blueprintEntry, err := h.server.db.GetBlueprint(ctx.Request().Context(), id, userID.OrgID())
	if err != nil {
		if errors.Is(err, db.BlueprintNotFoundError) {
			return echo.NewHTTPError(http.StatusNotFound, err)
		}
		return err
	}
	blueprint, err := BlueprintFromEntry(blueprintEntry)
	if err != nil {
		return err
	}

	blueprintResponse := BlueprintResponse{
		Id:             id,
		Name:           blueprintEntry.Name,
		Description:    blueprintEntry.Description,
		ImageRequests:  blueprint.ImageRequests,
		Distribution:   blueprint.Distribution,
		Customizations: blueprint.Customizations,
	}

	return ctx.JSON(http.StatusOK, blueprintResponse)
}

func (h *Handlers) UpdateBlueprint(ctx echo.Context, blueprintId uuid.UUID) error {
	userID, err := h.server.getIdentity(ctx)
	if err != nil {
		return err
	}

	var blueprintRequest CreateBlueprintRequest
	err = ctx.Bind(&blueprintRequest)
	if err != nil {
		return err
	}

	blueprint := BlueprintFromAPI(blueprintRequest)
	body, err := json.Marshal(blueprint)
	if err != nil {
		return err
	}

	if !blueprintNameRegex.MatchString(blueprintRequest.Name) {
		return ctx.JSON(http.StatusUnprocessableEntity, HTTPErrorList{
			Errors: []HTTPError{{
				Title:  "Invalid blueprint name",
				Detail: blueprintInvalidNameDetail,
			}},
		})
	}

	versionId := uuid.New()
	desc := ""
	if blueprintRequest.Description != nil {
		desc = *blueprintRequest.Description
	}
	err = h.server.db.UpdateBlueprint(ctx.Request().Context(), versionId, blueprintId, userID.OrgID(), blueprintRequest.Name, desc, body)
	if err != nil {
		ctx.Logger().Errorf("Error updating blueprint in db: %v", err)
		if errors.Is(err, db.BlueprintNotFoundError) {
			return echo.NewHTTPError(http.StatusNotFound, err)
		}
		return err
	}
	ctx.Logger().Infof("Updated blueprint %s", blueprintId)
	return ctx.JSON(http.StatusCreated, ComposeResponse{
		Id: blueprintId,
	})
}

func (h *Handlers) ComposeBlueprint(ctx echo.Context, id openapi_types.UUID) error {
	var requestBody ComposeBlueprintJSONBody
	err := ctx.Bind(&requestBody)
	if err != nil {
		return err
	}

	userID, err := h.server.getIdentity(ctx)
	if err != nil {
		return err
	}

	blueprintEntry, err := h.server.db.GetBlueprint(ctx.Request().Context(), id, userID.OrgID())
	if err != nil {
		return err
	}
	blueprint, err := BlueprintFromEntry(blueprintEntry)
	if err != nil {
		return err
	}
	composeResponses := make([]ComposeResponse, 0, len(blueprint.ImageRequests))
	clientId := ClientId("api")
	if ctx.Request().Header.Get("X-ImageBuilder-ui") != "" {
		clientId = "ui"
	}
	for _, imageRequest := range blueprint.ImageRequests {
		if requestBody.ImageTypes != nil && !slices.Contains(*requestBody.ImageTypes, imageRequest.ImageType) {
			continue
		}
		composeRequest := ComposeRequest{
			Customizations:   &blueprint.Customizations,
			Distribution:     blueprint.Distribution,
			ImageRequests:    []ImageRequest{imageRequest},
			ImageName:        &blueprintEntry.Name,
			ImageDescription: &blueprintEntry.Description,
			ClientId:         &clientId,
		}
		composesResponse, err := h.handleCommonCompose(ctx, composeRequest, &blueprintEntry.VersionId)
		if err != nil {
			return err
		}
		composeResponses = append(composeResponses, composesResponse)
	}

	return ctx.JSON(http.StatusCreated, composeResponses)
}

func (h *Handlers) GetBlueprints(ctx echo.Context, params GetBlueprintsParams) error {
	spec, err := GetSwagger()
	if err != nil {
		return err
	}
	userID, err := h.server.getIdentity(ctx)
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
	var blueprints []db.BlueprintWithNoBody
	var count int

	if params.Name != nil && common.FromPtr(params.Name) != "" {
		blueprint, err := h.server.db.FindBlueprintByName(ctx.Request().Context(), userID.OrgID(), *params.Name)
		if err != nil {
			return err
		}
		if blueprint != nil {
			blueprints = []db.BlueprintWithNoBody{*blueprint}
			count = 1
		}
		// Else no blueprint found - return empty list and count = 0
	} else if params.Search != nil && common.FromPtr(params.Search) != "" {
		blueprints, count, err = h.server.db.FindBlueprints(ctx.Request().Context(), userID.OrgID(), *params.Search, limit, offset)
		if err != nil {
			return err
		}
	} else {
		blueprints, count, err = h.server.db.GetBlueprints(ctx.Request().Context(), userID.OrgID(), limit, offset)
		if err != nil {
			return err
		}
	}

	ctx.Logger().Debugf("Getting blueprint list of %d items", count)

	data := make([]BlueprintItem, 0, len(blueprints))
	for _, blueprint := range blueprints {
		data = append(data, BlueprintItem{
			Id:             blueprint.Id,
			Name:           blueprint.Name,
			Description:    blueprint.Description,
			Version:        blueprint.Version,
			LastModifiedAt: blueprint.LastModifiedAt.Format(time.RFC3339),
		})
	}
	lastOffset := count - 1
	if lastOffset < 0 {
		lastOffset = 0
	}
	return ctx.JSON(http.StatusOK, BlueprintsResponse{
		Meta: ListResponseMeta{count},
		Links: ListResponseLinks{
			fmt.Sprintf("%v/v%v/composes?offset=0&limit=%v",
				RoutePrefix(), spec.Info.Version, limit),
			fmt.Sprintf("%v/v%v/composes?offset=%v&limit=%v",
				RoutePrefix(), spec.Info.Version, lastOffset, limit),
		},
		Data: data,
	})
}

func (h *Handlers) GetBlueprintComposes(ctx echo.Context, blueprintId openapi_types.UUID, params GetBlueprintComposesParams) error {
	userID, err := h.server.getIdentity(ctx)
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

	since := time.Hour * 24 * 14

	if params.BlueprintVersion != nil && *params.BlueprintVersion < 0 {
		*params.BlueprintVersion, err = h.server.db.GetLatestBlueprintVersionNumber(ctx.Request().Context(), userID.OrgID(), blueprintId)
		if err != nil {
			return err
		}
	}

	composes, err := h.server.db.GetBlueprintComposes(ctx.Request().Context(), userID.OrgID(), blueprintId, params.BlueprintVersion, since, limit, offset, ignoreImageTypeStrings)
	if err != nil {
		return err
	}
	count, err := h.server.db.CountBlueprintComposesSince(ctx.Request().Context(), userID.OrgID(), blueprintId, params.BlueprintVersion, since, ignoreImageTypeStrings)
	if err != nil {
		return err
	}

	data := make([]ComposesResponseItem, 0, len(composes))
	for _, c := range composes {
		bId := c.BlueprintId
		version := c.BlueprintVersion
		var cmpr ComposeRequest
		err = json.Unmarshal(c.Request, &cmpr)
		if err != nil {
			return err
		}
		data = append(data, ComposesResponseItem{
			BlueprintId:      &bId,
			BlueprintVersion: &version,
			CreatedAt:        c.CreatedAt.Format(time.RFC3339),
			Id:               c.Id,
			ImageName:        c.ImageName,
			Request:          cmpr,
			ClientId:         (*ClientId)(c.ClientId),
		})
	}

	linkParams := url.Values{}
	linkParams.Add("blueprint_id", blueprintId.String())
	if params.BlueprintVersion != nil {
		linkParams.Add("blueprint_version", strconv.Itoa(*params.BlueprintVersion))
	}
	return ctx.JSON(http.StatusOK, ComposesResponse{
		Data:  data,
		Meta:  ListResponseMeta{count},
		Links: h.newLinksWithExtraParams("composes", count, limit, linkParams),
	})
}

func (h *Handlers) DeleteBlueprint(ctx echo.Context, blueprintId openapi_types.UUID) error {
	userID, err := h.server.getIdentity(ctx)
	if err != nil {
		return err
	}

	err = h.server.db.DeleteBlueprint(ctx.Request().Context(), blueprintId, userID.OrgID(), userID.AccountNumber())
	if err != nil {
		if errors.Is(err, db.BlueprintNotFoundError) {
			return echo.NewHTTPError(http.StatusNotFound)
		}
		return err
	}
	return ctx.NoContent(http.StatusNoContent)
}
