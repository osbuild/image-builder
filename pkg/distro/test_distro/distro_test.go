package test_distro_test

import (
	"fmt"
	"testing"

	"github.com/osbuild/image-builder/v73/pkg/distro"
	"github.com/osbuild/image-builder/v73/pkg/distro/test_distro"
	"github.com/stretchr/testify/require"
)

func TestTestDistroGetPipelines(t *testing.T) {
	testDistro := test_distro.DistroFactory(test_distro.TestDistro1Name)
	for _, testArchName := range testDistro.ListArches() {
		testArch, err := testDistro.GetArch(testArchName)
		require.NoError(t, err)

		for _, testImageTypeName := range testArch.ListImageTypes() {
			t.Run(fmt.Sprintf("%s/%s", testArchName, testImageTypeName), func(t *testing.T) {
				testImageType, err := testArch.GetImageType(testImageTypeName)
				require.NoError(t, err)
				m, _, err := testImageType.Manifest(nil, distro.ImageOptions{}, nil, nil)
				require.NoError(t, err)
				require.NotNil(t, m)

				buildPipelines := m.BuildPipelines()
				require.Len(t, buildPipelines, 1)
				require.Equal(t, buildPipelines[0], "build")

				payloadPipelines := m.PayloadPipelines()
				require.Len(t, payloadPipelines, 1)
				require.Equal(t, payloadPipelines[0], "os")
			})
		}
	}
}
