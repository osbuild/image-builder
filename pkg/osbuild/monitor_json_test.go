package osbuild_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/osbuild/image-builder/pkg/osbuild"
)

const osbuildMonitorJSON_1 = `
{
  "message": "Top level message",
  "context": {
    "origin": "osbuild.monitor",
    "pipeline": {
      "name": "source org.osbuild.curl",
      "id": "598849389c35f93efe2412446f5ca6919434417b9bcea040ea5f9203de81db2c",
      "stage": {}
    },
    "id": "69816755441434713b7567970edfdd42d58193f163e1fdd506274d52246e87f2"
  },
  "timestamp": 1731585664.9090264,
  "progress": {
    "name": "name",
    "total": 4,
    "done": 1,
    "progress": {
      "name": "nested-name",
      "total": 8,
      "done": 2,
      "progress": {
        "name": "nested-nested-name",
        "total": 16,
        "done": 4
      }
    }
  }
}`

func TestOsbuildStatusNestingWorks(t *testing.T) {
	var status osbuild.StatusJSON

	err := json.Unmarshal([]byte(osbuildMonitorJSON_1), &status)
	assert.NoError(t, err)
	assert.Equal(t, "Top level message", status.Message)
	assert.Equal(t, "name", status.Progress.Name)
	assert.Equal(t, "69816755441434713b7567970edfdd42d58193f163e1fdd506274d52246e87f2", status.Context.ID)
	assert.Equal(t, 4, status.Progress.Total)
	assert.Equal(t, "nested-name", status.Progress.SubProgress.Name)
	assert.Equal(t, 8, status.Progress.SubProgress.Total)
	assert.Equal(t, "nested-nested-name", status.Progress.SubProgress.SubProgress.Name)
	assert.Equal(t, 16, status.Progress.SubProgress.SubProgress.Total)
	assert.Nil(t, status.Progress.SubProgress.SubProgress.SubProgress)
}
