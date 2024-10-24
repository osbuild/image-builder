package v1

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"time"

	"github.com/osbuild/image-builder/internal/clients/content_sources"

	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5/pgconn"
	openapi_types "github.com/oapi-codegen/runtime/types"
	"github.com/osbuild/image-builder/internal/common"
	"github.com/osbuild/image-builder/internal/db"
	"github.com/osbuild/images/pkg/crypt"

	"slices"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

var (
	blueprintNameRegex         = regexp.MustCompile(`\S+`)
	customizationUserNameRegex = regexp.MustCompile(`\S+`)
	blueprintInvalidNameDetail = "The blueprint name must contain at least two characters."
)

type BlueprintBody struct {
	Customizations Customizations `json:"customizations"`
	Distribution   Distributions  `json:"distribution"`
	ImageRequests  []ImageRequest `json:"image_requests"`
}

func (u *User) CryptPassword() error {
	// Prevent empty and already hashed password  from being hashed
	if u.Password == nil || len(*u.Password) == 0 || crypt.PasswordIsCrypted(*u.Password) {
		return nil
	}

	pw, err := crypt.CryptSHA512(*u.Password)
	if err != nil {
		return err
	}
	*u.Password = pw
	return nil
}

// Set password to nil if it is not nil
func (u *User) RedactPassword() {
	u.Password = nil
}

func (bb *BlueprintBody) CryptPasswords() error {
	if bb.Customizations.Users != nil {
		for i := range *bb.Customizations.Users {
			err := (*bb.Customizations.Users)[i].CryptPassword()
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func (bb *BlueprintBody) RedactPasswords() {
	if bb.Customizations.Users != nil {
		for i := range *bb.Customizations.Users {
			(*bb.Customizations.Users)[i].RedactPassword()
		}
	}
}

// Merges Password or SshKey from other User struct to this User struct if it is not set
func (u *User) MergeExisting(other User) {
	if u.Password == nil {
		u.Password = other.Password
	}
	if u.SshKey == nil {
		u.SshKey = other.SshKey
	}
}

// User must have name and non-empty password or ssh key
func (u *User) Valid() error {
	validName := customizationUserNameRegex.MatchString(u.Name)
	validPassword := u.Password != nil && len(*u.Password) > 0
	validSshKey := u.SshKey != nil && len(*u.SshKey) > 0
	if !validName || !(validPassword || validSshKey) {
		return fmt.Errorf("User ('%s') must have a name and either a password or an SSH key set.", u.Name)
	}
	return nil
}

func (u *User) MergeForUpdate(userData []User) error {
	// If both password and ssh_key in request user we don't need to fetch user from DB
	if !(u.Password != nil && len(*u.Password) > 0 && u.SshKey != nil && len(*u.SshKey) > 0) {
		eui := slices.IndexFunc(userData, func(eu User) bool {
			return eu.Name == u.Name
		})

		if eui == -1 { // User not found in DB
			err := u.Valid()
			if err != nil {
				return err
			}
		} else {
			u.MergeExisting(userData[eui])
		}
	}

	// If there is empty string in password or ssh_key, it means that we should remove it (set to nil)
	if u.Password != nil && *u.Password == "" {
		u.Password = nil
	}
	if u.SshKey != nil && *u.SshKey == "" {
		u.SshKey = nil
	}

	if err := u.Valid(); err != nil {
		return err
	}
	return nil
}

// Util function used to create and update Blueprint from API request (WRITE)
func BlueprintFromAPI(cbr CreateBlueprintRequest) (BlueprintBody, error) {
	bb := BlueprintBody{
		Customizations: cbr.Customizations,
		Distribution:   cbr.Distribution,
		ImageRequests:  cbr.ImageRequests,
	}
	err := bb.CryptPasswords()
	if err != nil {
		return BlueprintBody{}, err
	}
	return bb, nil
}

// Util function used to create Blueprint sctruct from DB entry (READ)
func BlueprintFromEntry(be *db.BlueprintEntry) (BlueprintBody, error) {
	var result BlueprintBody
	err := json.Unmarshal(be.Body, &result)
	if err != nil {
		return BlueprintBody{}, err
	}
	return result, nil
}

func BlueprintFromEntryWithRedactedPasswords(be *db.BlueprintEntry) (BlueprintBody, error) {
	result, err := BlueprintFromEntry(be)
	if err != nil {
		return BlueprintBody{}, err
	}
	result.RedactPasswords()
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

	var metadata []byte
	if blueprintRequest.Metadata != nil {
		metadata, err = json.Marshal(blueprintRequest.Metadata)
		if err != nil {
			return err
		}
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

	users := blueprintRequest.Customizations.Users
	if users != nil {
		for _, user := range *users {
			// Make sure every user has either ssh key or password set
			if err := user.Valid(); err != nil {
				return ctx.JSON(http.StatusUnprocessableEntity, HTTPErrorList{
					Errors: []HTTPError{{
						Title:  "Invalid user",
						Detail: err.Error(),
					}},
				})
			}
		}
	}

	blueprint, err := BlueprintFromAPI(blueprintRequest)
	if err != nil {
		return err
	}

	body, err := json.Marshal(blueprint)
	if err != nil {
		return err
	}

	err = h.server.db.InsertBlueprint(ctx.Request().Context(), id, versionId, userID.OrgID(), userID.AccountNumber(), blueprintRequest.Name, desc, body, metadata)
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

func (h *Handlers) GetBlueprint(ctx echo.Context, id openapi_types.UUID, params GetBlueprintParams) error {
	userID, err := h.server.getIdentity(ctx)
	if err != nil {
		return err
	}

	ctx.Logger().Infof("Fetching blueprint %s", id)
	if params.Version != nil && *params.Version <= 0 {
		if *params.Version != -1 {
			return echo.NewHTTPError(http.StatusBadRequest, "Invalid version number")
		}
		params.Version = nil
	}

	blueprintEntry, err := h.server.db.GetBlueprint(ctx.Request().Context(), id, userID.OrgID(), params.Version)
	if err != nil {
		if errors.Is(err, db.BlueprintNotFoundError) {
			return echo.NewHTTPError(http.StatusNotFound, err)
		}
		return err
	}

	blueprint, err := BlueprintFromEntryWithRedactedPasswords(blueprintEntry)
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

func (h *Handlers) ExportBlueprint(ctx echo.Context, id openapi_types.UUID) error {
	userID, err := h.server.getIdentity(ctx)
	if err != nil {
		return err
	}

	ctx.Logger().Infof("Fetching blueprint %s", id)
	blueprintEntry, err := h.server.db.GetBlueprint(ctx.Request().Context(), id, userID.OrgID(), nil)
	if err != nil {
		if errors.Is(err, db.BlueprintNotFoundError) {
			return echo.NewHTTPError(http.StatusNotFound, err)
		}
		return err
	}

	blueprint, err := BlueprintFromEntryWithRedactedPasswords(blueprintEntry)
	if err != nil {
		return err
	}

	blueprint.Customizations.Subscription = nil
	blueprintExportResponse := BlueprintExportResponse{
		Name:           blueprintEntry.Name,
		Description:    blueprintEntry.Description,
		Distribution:   blueprint.Distribution,
		Customizations: blueprint.Customizations,
		Metadata: BlueprintMetadata{
			ExportedAt: time.Now().UTC().String(),
			ParentId:   &id,
		},
	}

	repoUUIDs := []string{}
	if blueprint.Customizations.CustomRepositories != nil {
		for _, repo := range *blueprint.Customizations.CustomRepositories {
			repoUUIDs = append(repoUUIDs, repo.Id)
		}
	}

	exportedRepositoriesResp, err := h.server.csClient.BulkExportRepositories(ctx.Request().Context(), content_sources.ApiRepositoryExportRequest{
		RepositoryUuids: common.ToPtr(repoUUIDs),
	})
	if err != nil {
		return err
	}
	defer closeBody(ctx, exportedRepositoriesResp.Body)

	if exportedRepositoriesResp.StatusCode != http.StatusOK {
		if exportedRepositoriesResp.StatusCode != http.StatusUnauthorized {
			body, err := io.ReadAll(exportedRepositoriesResp.Body)
			if err != nil {
				return err
			}
			ctx.Logger().Warnf("Unable to export custom repositories: %s", body)
		}
		return fmt.Errorf("Unable to fetch custom repositories, got %v response", exportedRepositoriesResp.StatusCode)
	}

	if exportedRepositoriesResp.Body != nil {
		bodyBytes, err := io.ReadAll(exportedRepositoriesResp.Body)
		if err != nil {
			return fmt.Errorf("Unable to export custom repositories: %w", err)
		}

		if len(bodyBytes) != 0 {
			// Checking the contents of content sources response
			var exportedRepositories []content_sources.ApiRepositoryExportResponse
			err = json.Unmarshal(bodyBytes, &exportedRepositories)
			if err != nil {
				return fmt.Errorf("Unable to export custom repositories: %w, %s", err, string(bodyBytes))
			}
			// Saving the response in plaintext
			repositoryDetails := string(bodyBytes)
			blueprintExportResponse.CustomRepositoriesDetails = &repositoryDetails
		}
	}

	return ctx.JSON(http.StatusOK, blueprintExportResponse)
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

	if !blueprintNameRegex.MatchString(blueprintRequest.Name) {
		return ctx.JSON(http.StatusUnprocessableEntity, HTTPErrorList{
			Errors: []HTTPError{{
				Title:  "Invalid blueprint name",
				Detail: blueprintInvalidNameDetail,
			}},
		})
	}

	if blueprintRequest.Customizations.Users != nil {
		be, err := h.server.db.GetBlueprint(ctx.Request().Context(), blueprintId, userID.OrgID(), nil)
		if err != nil {
			if errors.Is(err, db.BlueprintNotFoundError) {
				return echo.NewHTTPError(http.StatusNotFound, err)
			}
			return err
		}
		eb, err := BlueprintFromEntry(be)
		if err != nil {
			return err
		}
		for i := range *blueprintRequest.Customizations.Users {
			err := (*blueprintRequest.Customizations.Users)[i].MergeForUpdate(*eb.Customizations.Users)
			if err != nil {
				return ctx.JSON(http.StatusUnprocessableEntity, HTTPErrorList{
					Errors: []HTTPError{{
						Title:  "Invalid user",
						Detail: err.Error(),
					}},
				})
			}
		}
	}

	blueprint, err := BlueprintFromAPI(blueprintRequest)
	if err != nil {
		return ctx.JSON(http.StatusUnprocessableEntity, HTTPErrorList{
			Errors: []HTTPError{{
				Title:  "Invalid blueprint",
				Detail: err.Error(),
			}},
		})
	}

	body, err := json.Marshal(blueprint)
	if err != nil {
		return err
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

	blueprintEntry, err := h.server.db.GetBlueprint(ctx.Request().Context(), id, userID.OrgID(), nil)
	if err != nil {
		if errors.Is(err, db.BlueprintNotFoundError) {
			return echo.NewHTTPError(http.StatusNotFound, err)
		}
		return err
	}
	blueprint, err := BlueprintFromEntryWithRedactedPasswords(blueprintEntry)
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
		if errors.Is(err, db.BlueprintNotFoundError) {
			return echo.NewHTTPError(http.StatusNotFound)
		}
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
