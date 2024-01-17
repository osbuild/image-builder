package tutils

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"testing"

	fedora_identity "github.com/osbuild/community-gateway/oidc-authorizer/pkg/identity"
	"github.com/stretchr/testify/require"
)

// org_id 000000
var AuthString0 = GetCompleteBase64Header("000000")
var AuthString0WithoutEntitlements = GetBase64HeaderWithoutEntitlements("000000")

// org_id 000001
var AuthString1 = GetCompleteBase64Header("000001")

var FedAuth = getBase64Header(fedoraHeader, "User")

func GetResponseError(url string) (*http.Response, error) {
	client := &http.Client{}
	request, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	request.Header.Add("x-rh-identity", AuthString0)

	return client.Do(request)
}

func GetResponseBody(t *testing.T, url string, auth *string) (int, string) {
	client := &http.Client{}
	request, err := http.NewRequest("GET", url, nil)
	require.NoError(t, err)
	if auth != nil {
		request.Header.Add("x-rh-identity", *auth)
		request.Header.Add(fedora_identity.FedoraIDHeader, *auth)
	}

	response, err := client.Do(request)
	require.NoError(t, err)
	if err != nil {
		/* #nosec G307 */
		defer response.Body.Close()
	}

	body, err := io.ReadAll(response.Body)
	require.NoError(t, err)

	return response.StatusCode, string(body)
}

func PostResponseBody(t *testing.T, url string, compose interface{}) (int, string) {
	buf, err := json.Marshal(compose)
	require.NoError(t, err)

	client := &http.Client{}
	request, err := http.NewRequest("POST", url, bytes.NewReader(buf))
	require.NoError(t, err)
	request.Header.Add("Content-Type", "application/json")
	request.Header.Add("x-rh-identity", AuthString0)

	response, err := client.Do(request)
	require.NoError(t, err)
	if err != nil {
		/* #nosec G307 */
		defer response.Body.Close()
	}

	body, err := io.ReadAll(response.Body)
	require.NoError(t, err)

	return response.StatusCode, string(body)
}
