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
var AuthString0 = "eyJlbnRpdGxlbWVudHMiOnsiaW5zaWdodHMiOnsiaXNfZW50aXRsZWQiOnRydWV9LCJzbWFydF9tYW5hZ2VtZW50Ijp7ImlzX2VudGl0bGVkIjp0cnVlfSwib3BlbnNoaWZ0Ijp7ImlzX2VudGl0bGVkIjp0cnVlfSwiaHlicmlkIjp7ImlzX2VudGl0bGVkIjp0cnVlfSwibWlncmF0aW9ucyI6eyJpc19lbnRpdGxlZCI6dHJ1ZX0sImFuc2libGUiOnsiaXNfZW50aXRsZWQiOnRydWV9fSwiaWRlbnRpdHkiOnsiYWNjb3VudF9udW1iZXIiOiI1MDAwMDAiLCJ0eXBlIjoiVXNlciIsInVzZXIiOnsidXNlcm5hbWUiOiJ1c2VyIiwiZW1haWwiOiJ1c2VyQHVzZXIudXNlciIsImZpcnN0X25hbWUiOiJ1c2VyIiwibGFzdF9uYW1lIjoidXNlciIsImlzX2FjdGl2ZSI6dHJ1ZSwiaXNfb3JnX2FkbWluIjp0cnVlLCJpc19pbnRlcm5hbCI6dHJ1ZSwibG9jYWxlIjoiZW4tVVMifSwiaW50ZXJuYWwiOnsib3JnX2lkIjoiMDAwMDAwIn19fQo="

// org_id 000001, account_number 600000
var AuthString1 = "eyJlbnRpdGxlbWVudHMiOnsiaW5zaWdodHMiOnsiaXNfZW50aXRsZWQiOnRydWV9LCJzbWFydF9tYW5hZ2VtZW50Ijp7ImlzX2VudGl0bGVkIjp0cnVlfSwib3BlbnNoaWZ0Ijp7ImlzX2VudGl0bGVkIjp0cnVlfSwiaHlicmlkIjp7ImlzX2VudGl0bGVkIjp0cnVlfSwibWlncmF0aW9ucyI6eyJpc19lbnRpdGxlZCI6dHJ1ZX0sImFuc2libGUiOnsiaXNfZW50aXRsZWQiOnRydWV9fSwiaWRlbnRpdHkiOnsiYWNjb3VudF9udW1iZXIiOiI2MDAwMDAiLCJ0eXBlIjoiVXNlciIsInVzZXIiOnsidXNlcm5hbWUiOiJ1c2VyIiwiZW1haWwiOiJ1c2VyQHVzZXIudXNlciIsImZpcnN0X25hbWUiOiJ1c2VyIiwibGFzdF9uYW1lIjoidXNlciIsImlzX2FjdGl2ZSI6dHJ1ZSwiaXNfb3JnX2FkbWluIjp0cnVlLCJpc19pbnRlcm5hbCI6dHJ1ZSwibG9jYWxlIjoiZW4tVVMifSwiaW50ZXJuYWwiOnsib3JnX2lkIjoiMDAwMDAxIn19fQ=="

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

func (d *dB) InsertCompose(jobId, accountNumber, orgId string, request json.RawMessage) error {
	id, err := uuid.Parse(jobId)
	if err != nil {
		return err
	}
	dbEntry := db.ComposeEntry{
		Id:        id,
		Request:   request,
		CreatedAt: time.Now(),
	}
	if d.accountOwernship[accountNumber] == nil {
		d.accountOwernship[accountNumber] = make([]db.ComposeEntry, 0)
	}
	d.accountOwernship[accountNumber] = append(d.accountOwernship[accountNumber], dbEntry)
	return nil
}

func (d *dB) GetComposes(accountNumber string, limit, offset int) ([]db.ComposeEntry, int, error) {
	if d.accountOwernship[accountNumber] != nil {
		return d.accountOwernship[accountNumber], len(d.accountOwernship[accountNumber]), nil
	} else {
		return make([]db.ComposeEntry, 0), 0, nil
	}
}

func (d *dB) GetCompose(jobId string, accountNumber string) (*db.ComposeEntry, error) {
	if d.accountOwernship[accountNumber] != nil {
		for _, composeEntry := range d.accountOwernship[accountNumber] {
			if composeEntry.Id.String() == jobId {
				return &composeEntry, nil
			}
		}
	}
	return nil, db.ComposeNotFoundError
}
