package tutils

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"testing"
	"time"

	"github.com/osbuild/image-builder/internal/db"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

// org_id 000000, account_number 500000
var AuthString0 = GetCompleteBas64Header("500000", "000000")

// org_id 000001, account_number 600000
var AuthString1 = GetCompleteBas64Header("600000", "000001")

func GetResponseError(url string) (*http.Response, error) {
	client := &http.Client{}
	request, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	request.Header.Add("x-rh-identity", AuthString0)

	return client.Do(request)
}

func GetResponseBody(t *testing.T, url string, auth *string) (*http.Response, string) {
	client := &http.Client{}
	request, err := http.NewRequest("GET", url, nil)
	require.NoError(t, err)
	if auth != nil {
		request.Header.Add("x-rh-identity", *auth)
	}

	response, err := client.Do(request)
	require.NoError(t, err)

	body, err := ioutil.ReadAll(response.Body)
	require.NoError(t, err)

	err = response.Body.Close()
	require.NoError(t, err)

	return response, string(body)
}

func PostResponseBody(t *testing.T, url string, compose interface{}) (*http.Response, string) {
	buf, err := json.Marshal(compose)
	require.NoError(t, err)

	client := &http.Client{}
	request, err := http.NewRequest("POST", url, bytes.NewReader(buf))
	require.NoError(t, err)
	request.Header.Add("Content-Type", "application/json")
	request.Header.Add("x-rh-identity", AuthString0)

	response, err := client.Do(request)
	require.NoError(t, err)

	body, err := ioutil.ReadAll(response.Body)
	require.NoError(t, err)

	err = response.Body.Close()
	require.NoError(t, err)

	return response, string(body)
}

type dB struct {
	accountOwernship map[string][]db.ComposeEntry
}

func InitDB() db.DB {
	return &dB{
		make(map[string][]db.ComposeEntry),
	}
}

func (d *dB) InsertCompose(jobId, accountNumber, orgId string, imageName *string, request json.RawMessage) error {
	id, err := uuid.Parse(jobId)
	if err != nil {
		return err
	}
	dbEntry := db.ComposeEntry{
		Id:        id,
		Request:   request,
		CreatedAt: time.Now(),
		ImageName: imageName,
	}
	if d.accountOwernship[orgId] == nil {
		d.accountOwernship[orgId] = make([]db.ComposeEntry, 0)
	}
	d.accountOwernship[orgId] = append(d.accountOwernship[orgId], dbEntry)
	return nil
}

func (d *dB) GetComposes(orgId string, since time.Duration, limit, offset int) ([]db.ComposeEntry, int, error) {
	if d.accountOwernship[orgId] != nil {
		return d.accountOwernship[orgId], len(d.accountOwernship[orgId]), nil
	} else {
		return make([]db.ComposeEntry, 0), 0, nil
	}
}

func (d *dB) GetCompose(jobId string, orgId string) (*db.ComposeEntry, error) {
	if d.accountOwernship[orgId] != nil {
		for _, composeEntry := range d.accountOwernship[orgId] {
			if composeEntry.Id.String() == jobId {
				return &composeEntry, nil
			}
		}
	}
	return nil, db.ComposeNotFoundError
}

func (d *dB) CountComposesSince(orgId string, duration time.Duration) (int, error) {
	_, count, err := d.GetComposes(orgId, duration, 100, 0)
	return count, err
}
