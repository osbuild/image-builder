package v1

import (
	"testing"

	"github.com/osbuild/image-builder/internal/common"
	"github.com/stretchr/testify/assert"
)

// buildTestRequest builds a ComposeRequest with optional file and image sizes
func buildComposeRequest(fsSize *uint64, imgSize *uint64, imgType ImageTypes) *ComposeRequest {
	cr := &ComposeRequest{
		Distribution: "centos-8",
		ImageRequests: []ImageRequest{
			{
				Architecture:  "x86_64",
				ImageType:     imgType,
				Size:          imgSize,
				UploadRequest: UploadRequest{},
			},
		},
	}

	// Add a filesystem size
	if fsSize != nil {
		cr.Customizations = &Customizations{
			Filesystem: &[]Filesystem{
				{
					Mountpoint: "/var",
					MinSize:    *fsSize,
				},
			},
		}
	}

	return cr
}

func TestValidateComposeRequest(t *testing.T) {
	testData := []struct {
		fsSize  *uint64
		imgSize *uint64
		isError bool
	}{
		// Filesystem, Image, Error expected for ami/azure images
		{nil, nil, false}, // No sizes
		{common.ToPtr(uint64(68719476736)), nil, false}, // Just filesystem size, smaller than FSMaxSize

		{nil, common.ToPtr(uint64(13958643712)), false},                                  // Just image size, smaller than FSMaxSize
		{common.ToPtr(uint64(FSMaxSize + 1)), nil, true},                                 // Just filesystem size, larger than FSMaxSize
		{nil, common.ToPtr(uint64(FSMaxSize + 1)), true},                                 // Just image side, larger than FSMaxSize
		{common.ToPtr(uint64(68719476736)), common.ToPtr(uint64(13958643712)), false},    // filesystem smaller, image smaller
		{common.ToPtr(uint64(FSMaxSize + 1)), common.ToPtr(uint64(13958643712)), true},   // filesystem larger, image smaller
		{common.ToPtr(uint64(68719476736)), common.ToPtr(uint64(FSMaxSize + 1)), true},   // filesystem smaller, image larger
		{common.ToPtr(uint64(FSMaxSize + 1)), common.ToPtr(uint64(FSMaxSize + 1)), true}, // filesystem larger, image larger
	}

	// Guest Image has no errors even when the size is larger
	for idx, td := range testData {
		assert.Nil(t, validateComposeRequest(buildComposeRequest(td.fsSize, td.imgSize, ImageTypesGuestImage)), "%v: idx=%d", ImageTypesGuestImage, idx)
	}

	// Test the aws and azure types for expected errors
	for _, it := range []ImageTypes{ImageTypesAmi, ImageTypesAws, ImageTypesAzure, ImageTypesVhd} {
		for idx, td := range testData {
			if td.isError {
				assert.Error(t, validateComposeRequest(buildComposeRequest(td.fsSize, td.imgSize, it)), "%v: idx=%d", it, idx)
			} else {
				assert.Nil(t, validateComposeRequest(buildComposeRequest(td.fsSize, td.imgSize, it)), "%v: idx=%d", it, idx)
			}
		}
	}
}
