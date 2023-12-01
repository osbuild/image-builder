package v1

import (
	"encoding/json"
	"net/http"

	"github.com/osbuild/image-builder/internal/db"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/sirupsen/logrus"
)

type BlueprintV1 struct {
	Version        int            `json:"version"`
	Name           string         `json:"name"`
	Description    string         `json:"description"`
	Customizations Customizations `json:"customizations"`
	Distribution   Distributions  `json:"distribution"`
	ImageRequests  []ImageRequest `json:"image_requests"`
}

func BlueprintFromAPI(cbr CreateBlueprintRequest) BlueprintV1 {
	return BlueprintV1{
		Version:        1,
		Name:           cbr.Name,
		Description:    cbr.Description,
		Customizations: cbr.Customizations,
		Distribution:   cbr.Distribution,
		ImageRequests:  cbr.ImageRequests,
	}
}

func BlueprintFromEntry(be *db.BlueprintEntry) (BlueprintV1, error) {
	var result BlueprintV1
	err := json.Unmarshal(be.Body, &result)
	if err != nil {
		return BlueprintV1{}, err
	}
	result.Version = be.Version
	return result, nil
}

func (h *Handlers) CreateBlueprint(ctx echo.Context) error {
	idHeader, err := getIdentityHeader(ctx)
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
	err = h.server.db.InsertBlueprint(id, versionId, idHeader.Identity.OrgID, idHeader.Identity.AccountNumber, blueprint.Name, blueprint.Description, body)
	if err != nil {
		logrus.Error("Error inserting id into db", err)
		return err
	}
	ctx.Logger().Infof("Inserted blueprint %s", id)
	return ctx.JSON(http.StatusCreated, ComposeResponse{
		Id: id,
	})
}
