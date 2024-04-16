package v1

import (
	"encoding/json"
	"net/http"

	"github.com/osbuild/image-builder/internal/clients/recommendations"

	"github.com/labstack/echo/v4"
)

func (h *Handlers) RecommendPackage(ctx echo.Context) error {
	var uploadPackageRequest RecommendPackageRequest
	err := ctx.Bind(&uploadPackageRequest)
	if err != nil {
		return err
	}

	recommendationResponse, err := h.handleRecommendationsResponse(ctx, uploadPackageRequest)
	if err != nil {
		ctx.Logger().Errorf("Failed to recommendation package Response: %v", err)
		return err
	}

	return ctx.JSON(http.StatusOK, recommendationResponse)
}

func (h *Handlers) handleRecommendationsResponse(ctx echo.Context, uploadPackageRequest RecommendPackageRequest) (RecommendationsResponse, error) {
	cloudRP := recommendations.RecommendPackageRequest{
		Packages:            uploadPackageRequest.Packages,
		RecommendedPackages: uploadPackageRequest.RecommendedPackages,
	}

	resp, err := h.server.rClient.RecommendationsPackages(cloudRP)
	if err != nil {
		ctx.Logger().Errorf("Failed to get recommendation response: %v", err)
		return RecommendationsResponse{}, err
	}

	ctx.Logger().Debugf("Getting Response list of items %v", resp)
	defer closeBody(ctx, resp.Body)
	var packages []string
	err = json.NewDecoder(resp.Body).Decode(&packages)
	if err != nil {
		return RecommendationsResponse{}, err
	}

	if len(packages) == 0 {
		ctx.Logger().Errorf("User should define packages")
		return RecommendationsResponse{}, nil
	}

	return RecommendationsResponse{Packages: packages}, nil
}
