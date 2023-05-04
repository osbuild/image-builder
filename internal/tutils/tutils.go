package tutils

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/osbuild/image-builder/internal/db"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

// org_id 000000
var AuthString0 = GetCompleteBase64Header("000000")
var AuthString0WithoutEntitlements = GetBase64HeaderWithoutEntitlements("000000")

// org_id 000001
var AuthString1 = GetCompleteBase64Header("000001")

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

type dB struct {
	composes map[string][]db.ComposeEntry
	clones   map[uuid.UUID][]db.CloneEntry
}

func InitDB() db.DB {
	return &dB{
		make(map[string][]db.ComposeEntry),
		make(map[uuid.UUID][]db.CloneEntry),
	}
}

func (d *dB) InsertCompose(jobId uuid.UUID, accountNumber, orgId string, imageName *string, request json.RawMessage) error {
	if jobId == uuid.Nil {
		return errors.New("invalid jobId")
	}
	dbEntry := db.ComposeEntry{
		Id:        jobId,
		Request:   request,
		CreatedAt: time.Now(),
		ImageName: imageName,
	}
	if d.composes[orgId] == nil {
		d.composes[orgId] = make([]db.ComposeEntry, 0)
	}
	d.composes[orgId] = append(d.composes[orgId], dbEntry)
	return nil
}

func (d *dB) GetComposes(orgId string, since time.Duration, limit, offset int) ([]db.ComposeEntry, int, error) {
	if d.composes[orgId] != nil {
		return d.composes[orgId], len(d.composes[orgId]), nil
	} else {
		return make([]db.ComposeEntry, 0), 0, nil
	}
}

func (d *dB) GetCompose(jobId uuid.UUID, orgId string) (*db.ComposeEntry, error) {
	if d.composes[orgId] != nil {
		for _, composeEntry := range d.composes[orgId] {
			if composeEntry.Id == jobId {
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

func (d *dB) GetComposeImageType(jobId uuid.UUID, orgId string) (string, error) {
	return "aws", nil
}

func (d *dB) InsertClone(composeId, cloneId uuid.UUID, request json.RawMessage) error {
	if composeId == uuid.Nil {
		return errors.New("invalid composeId")
	}

	if cloneId == uuid.Nil {
		return errors.New("invalid cloneId")
	}

	entry := db.CloneEntry{
		Id:        cloneId,
		Request:   request,
		CreatedAt: time.Now(),
	}

	if d.clones[composeId] == nil {
		d.clones[composeId] = make([]db.CloneEntry, 0)
	}
	d.clones[composeId] = append(d.clones[composeId], entry)
	return nil

}

func (d *dB) GetClonesForCompose(composeId uuid.UUID, orgId string, limit, offset int) ([]db.CloneEntry, int, error) {
	if composeId == uuid.Nil {
		return nil, 0, errors.New("invalid composeId")
	}
	return d.clones[composeId], len(d.clones[composeId]), nil
}

func (d *dB) GetClone(clId uuid.UUID, orgId string) (*db.CloneEntry, error) {
	if clId == uuid.Nil {
		return nil, errors.New("invalid cloneId")
	}
	for _, v := range d.clones {
		for _, cl := range v {
			if cl.Id == clId {
				return &cl, nil
			}
		}
	}
	return nil, nil
}
