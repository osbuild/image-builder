package main

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestWriteUploadResult(t *testing.T) {
	const imageID = "ocid1.image.oc1.test.example"

	for _, tc := range []struct {
		name   string
		format string
		want   string
	}{
		{
			name: "text",
			want: "Image ocid1.image.oc1.test.example was uploaded and created successfully\n",
		},
		{
			name:   "json",
			format: "json",
			want:   "{\"image_id\":\"ocid1.image.oc1.test.example\"}\n",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var out bytes.Buffer
			require.NoError(t, writeUploadResult(&out, tc.format, imageID))
			require.Equal(t, tc.want, out.String())
		})
	}
}

func TestWriteUploadResultRejectsUnknownFormat(t *testing.T) {
	var out bytes.Buffer
	err := writeUploadResult(&out, "yaml", "ocid1.image.oc1.test.example")
	require.EqualError(t, err, `unsupported format "yaml", supported formats: text, json`)
}
