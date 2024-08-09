package compliance

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"slices"

	"github.com/redhatinsights/identity"
)

var (
	ErrorAuth         = errors.New("User is not authorized")
	ErrorMajorVersion = errors.New("Major version of policy doesn't match requested major version")
	ErrorMinorVersion = errors.New("No minor version of tailoring found in the requested policy")
	ErrorNotFound     = errors.New("Policy or its tailorings are missing")
	ErrorNotOk        = errors.New("Unexpected http status")
)

type ComplianceClient struct {
	url    string
	client *http.Client
}

type ComplianceClientConfig struct {
	URL string
}

type PolicyData struct {
	PolicyID      string
	ProfileID     string
	TailoringID   string
	TailoringData json.RawMessage
}

func NewClient(conf ComplianceClientConfig) *ComplianceClient {
	return &ComplianceClient{
		url:    conf.URL,
		client: &http.Client{},
	}
}

func (cc *ComplianceClient) request(ctx context.Context, method, url string) (*http.Response, error) {
	req, err := http.NewRequest(method, url, nil)
	if err != nil {
		return nil, err
	}

	id, ok := identity.GetIdentityHeader(ctx)
	if !ok {
		return nil, fmt.Errorf("Unable to get identity from context")
	}
	req.Header.Add("x-rh-identity", id)
	req.Header.Add("content-type", "application/json")

	return cc.client.Do(req)
}

type v2PolicyResponse struct {
	Data v2PolicyData `json:"data"`
}

type v2PolicyData struct {
	ID             string `json:"id"`
	RefID          string `json:"ref_id"`
	OSMajorVersion int    `json:"os_major_version"`
}

type v2TailoringsResponse struct {
	Data []v2TailoringItem `json:"data"`
}

type v2TailoringItem struct {
	ID             string `json:"id"`
	OSMajorVersion int    `json:"os_major_version"`
	OSMinorVersion int    `json:"os_minor_version"`
}

func (cc *ComplianceClient) PolicyDataForMinorVersion(ctx context.Context, majorVersion, minorVersion int, policyID string) (*PolicyData, error) {
	policiesResp, err := cc.request(ctx, "GET", fmt.Sprintf("%s/policies/%s", cc.url, policyID))
	if err != nil {
		return nil, err
	}

	if policiesResp.StatusCode == http.StatusUnauthorized || policiesResp.StatusCode == http.StatusForbidden {
		return nil, ErrorAuth
	} else if policiesResp.StatusCode == http.StatusNotFound {
		return nil, ErrorNotFound
	} else if policiesResp.StatusCode != http.StatusOK {
		return nil, ErrorNotOk
	}

	defer policiesResp.Body.Close()
	var v2pr v2PolicyResponse
	err = json.NewDecoder(policiesResp.Body).Decode(&v2pr)
	if err != nil {
		return nil, err
	}

	if v2pr.Data.OSMajorVersion != majorVersion {
		return nil, ErrorMajorVersion
	}

	tailoringsResp, err := cc.request(ctx, "GET", fmt.Sprintf("%s/policies/%s/tailorings", cc.url, policyID))
	if err != nil {
		return nil, err
	}
	defer tailoringsResp.Body.Close()

	if tailoringsResp.StatusCode == http.StatusUnauthorized || tailoringsResp.StatusCode == http.StatusForbidden {
		return nil, ErrorAuth
	} else if tailoringsResp.StatusCode == http.StatusNotFound {
		return nil, ErrorNotFound
	} else if tailoringsResp.StatusCode != http.StatusOK {
		return nil, ErrorNotOk
	}

	var v2tr v2TailoringsResponse
	err = json.NewDecoder(tailoringsResp.Body).Decode(&v2tr)
	if err != nil {
		return nil, err
	}

	tailoringIdx := slices.IndexFunc(v2tr.Data, func(item v2TailoringItem) bool {
		return item.OSMinorVersion == minorVersion
	})
	if tailoringIdx == -1 {
		return nil, ErrorMinorVersion
	}

	tailoringFileResp, err := cc.request(ctx, "GET", fmt.Sprintf("%s/policies/%s/tailorings/%s/tailoring_file.json", cc.url, policyID, "tailoring-id"))
	if err != nil {
		return nil, err
	}
	defer tailoringFileResp.Body.Close()

	if tailoringFileResp.StatusCode == http.StatusUnauthorized || tailoringFileResp.StatusCode == http.StatusForbidden {
		return nil, ErrorAuth
	} else if tailoringFileResp.StatusCode == http.StatusNotFound {
		return nil, ErrorNotFound
	} else if tailoringFileResp.StatusCode != http.StatusOK && tailoringFileResp.StatusCode != http.StatusNoContent {
		return nil, ErrorNotOk
	}

	var tailoringData json.RawMessage
	// returns 204 if there's no tailoring attached to the policy
	if tailoringFileResp.StatusCode != http.StatusNoContent {
		tailoringData, err = io.ReadAll(tailoringFileResp.Body)
		if err != nil {
			return nil, err
		}
	}

	return &PolicyData{
		PolicyID:      v2pr.Data.ID,
		ProfileID:     v2pr.Data.RefID,
		TailoringID:   v2tr.Data[tailoringIdx].ID,
		TailoringData: tailoringData,
	}, nil
}
