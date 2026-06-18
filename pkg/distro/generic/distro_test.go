package generic_test

import (
	"fmt"
	"testing"

	"github.com/osbuild/blueprint/pkg/blueprint"
	"github.com/osbuild/image-builder/internal/common"
	"github.com/osbuild/image-builder/pkg/distro"
	"github.com/osbuild/image-builder/pkg/distro/generic"
	testrepos "github.com/osbuild/image-builder/test/data/repositories"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBootstrapContainers(t *testing.T) {
	repos, err := testrepos.New()
	assert.NoError(t, err)

	for _, distroName := range repos.ListDistros() {
		t.Run(distroName, func(t *testing.T) {
			d := generic.DistroFactory(distroName)
			assert.NotNil(t, d)
			assert.NotEmpty(t, d.(*generic.Distribution).DistroYAML.BootstrapContainers)
		})
	}
}

// Test that the generic Manifest() function implementation in all
// distributions returns an error when appropriate. This does not cover all
// error conditions in functions called by Manifest(), such as blueprint
// customization validation. That functionality is tested in
// [TestCheckOptions].
// All distros use the same implementation of the [distro.ImageType.Manifest]
// method, but with this test we make sure that if any distro adds its own
// implementation it does not fail to propagate errors as expected.
func TestManifestError(t *testing.T) {
	require := require.New(t)
	repos, err := testrepos.New()
	require.NoError(err)

	// use a single image type from each distro
	for _, distroName := range repos.ListDistros() {
		df := generic.DistroFactory(distroName)
		require.NotNil(df)

		dist := fedoraFamilyDistros[len(fedoraFamilyDistros)-1]
		arch, err := dist.GetArch("x86_64")
		require.NoError(err)

		ami, err := arch.GetImageType("ami")
		require.NoError(err)

		type testCase struct {
			bp          blueprint.Blueprint
			options     distro.ImageOptions
			expWarnings []string
			expErr      string
		}

		testCases := map[string]testCase{
			"happy": {},
			"bad-filesystem+disk": {
				// filesystem + disk is always an error
				bp: blueprint.Blueprint{
					Customizations: &blueprint.Customizations{
						Filesystem: []blueprint.FilesystemCustomization{
							{
								Mountpoint: "/data",
							},
						},
						Disk: &blueprint.DiskCustomization{
							Partitions: []blueprint.PartitionCustomization{
								{
									Type: "plain",
									FilesystemTypedCustomization: blueprint.FilesystemTypedCustomization{
										Mountpoint: "/data",
										FSType:     "ext4",
									},
								},
							},
						},
					},
				},
				expErr: fmt.Sprintf("blueprint validation failed for image type %q: customizations.disk cannot be used with customizations.filesystem", ami.Name()),
			},
			"bad-repos-customization": {
				// Repo customizations are read inside the Manifest function, but
				// the error for invalid repositories should be caught by the
				// options checker before they're read. This test makes sure we
				// pick up the potential error message change if the order of
				// operations is ever changed.
				bp: blueprint.Blueprint{
					Customizations: &blueprint.Customizations{
						Repositories: []blueprint.RepositoryCustomization{
							{
								// Invalid: requires ID
								BaseURLs: []string{"https://example.org/repo"},
							},
						},
					},
				},
				expErr: fmt.Sprintf("blueprint validation failed for image type %q: Repository ID is required", ami.Name()),
			},
			"warnings": {
				// make sure warnings are properly returned
				bp: blueprint.Blueprint{
					Customizations: &blueprint.Customizations{
						FIPS: common.ToPtr(true),
					},
				},
				expWarnings: []string{common.FIPSEnabledImageWarning + "\n"},
			},
		}

		for name, tc := range testCases {
			t.Run(name, func(t *testing.T) {
				assert := assert.New(t)
				_, warnings, err := ami.Manifest(&tc.bp, tc.options, nil, nil)
				assert.Equal(tc.expWarnings, warnings)
				if tc.expErr == "" {
					assert.NoError(err)
				} else {
					assert.EqualError(err, tc.expErr)
				}
			})
		}
	}
}
