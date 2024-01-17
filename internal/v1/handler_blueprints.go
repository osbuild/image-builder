package v1

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"

	openapi_types "github.com/deepmap/oapi-codegen/pkg/types"
	"github.com/osbuild/image-builder/internal/db"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/sirupsen/logrus"
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

	id := uuid.New()
	versionId := uuid.New()
	err = h.server.db.InsertBlueprint(id, versionId, userID.OrgID(), userID.AccountNumber(), blueprintRequest.Name, blueprintRequest.Description, body)
	if err != nil {
		logrus.Error("Error inserting id into db", err)
		return err
	}
	ctx.Logger().Infof("Inserted blueprint %s", id)
	return ctx.JSON(http.StatusCreated, ComposeResponse{
		Id: id,
	})
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

	versionId := uuid.New()
	err = h.server.db.UpdateBlueprint(versionId, blueprintId, userID.OrgID(), blueprintRequest.Name, blueprintRequest.Description, body)
	if err != nil {
		ctx.Logger().Errorf("Error updating blueprint in db: %v", err)
		return err
	}
	ctx.Logger().Infof("Updated blueprint %s", blueprintId)
	return ctx.JSON(http.StatusCreated, ComposeResponse{
		Id: blueprintId,
	})
}

func (h *Handlers) ComposeBlueprint(ctx echo.Context, id openapi_types.UUID) error {
	userID, err := h.server.getIdentity(ctx)
	if err != nil {
		return err
	}

	blueprintEntry, err := h.server.db.GetBlueprint(id, userID.OrgID(), userID.AccountNumber())
	if err != nil {
		return err
	}
	blueprint, err := BlueprintFromEntry(blueprintEntry)
	if err != nil {
		return err
	}
	composeResponses := make([]ComposeResponse, 0, len(blueprint.ImageRequests))
	clientId := ClientId("api")
	for _, imageRequest := range blueprint.ImageRequests {
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

	if params.Search != nil {
		blueprints, count, err = h.server.db.FindBlueprints(userID.OrgID(), *params.Search, limit, offset)
		if err != nil {
			return err
		}
	} else {
		blueprints, count, err = h.server.db.GetBlueprints(userID.OrgID(), limit, offset)
		if err != nil {
			return err
		}
	}

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

	composes, err := h.server.db.GetBlueprintComposes(userID.OrgID(), blueprintId, params.BlueprintVersion, since, limit, offset, ignoreImageTypeStrings)
	if err != nil {
		return err
	}
	count, err := h.server.db.CountBlueprintComposesSince(userID.OrgID(), blueprintId, params.BlueprintVersion, since, ignoreImageTypeStrings)
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
		Data: data,
		Meta: struct {
			Count int `json:"count"`
		}{count},
		Links: h.newLinksWithExtraParams("composes", count, limit, linkParams),
	})
}
