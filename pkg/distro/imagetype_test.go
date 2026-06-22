package distro_test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/osbuild/blueprint/pkg/blueprint"
	"github.com/osbuild/image-builder/v73/pkg/distro"
	"github.com/osbuild/image-builder/v73/pkg/distrofactory"
	"github.com/osbuild/image-builder/v73/pkg/rpmmd"
)

func TestManifestRepositoryCustomization(t *testing.T) {
	var options distro.ImageOptions
	var repos []rpmmd.RepoConfig

	distroFactory := distrofactory.NewDefault()
	for _, distroName := range []string{"fedora-42", "rhel-9.6"} {
		distro := distroFactory.GetDistro(distroName)
		arch, err := distro.GetArch("x86_64")
		assert.NoError(t, err)
		imgType, err := arch.GetImageType("qcow2")
		assert.NoError(t, err)

		for _, installFrom := range []bool{false, true} {
			t.Run(fmt.Sprintf("repo enabled for install %v", installFrom), func(t *testing.T) {
				bp := &blueprint.Blueprint{
					Packages: []blueprint.Package{{Name: "hello"}},
					Customizations: &blueprint.Customizations{
						Repositories: []blueprint.RepositoryCustomization{
							{Id: "repo1", BaseURLs: []string{"example.com/repo1"}},
							{Id: "repo2", BaseURLs: []string{"example.com/repo2"}, InstallFrom: installFrom},
						},
					},
				}
				mani, _, err := imgType.Manifest(bp, options, repos, nil)
				assert.NoError(t, err)
				chains, err := mani.GetPackageSetChains()
				assert.NoError(t, err)
				osChains := chains["os"]
				baseChain := osChains[0]
				assert.Contains(t, baseChain.Include, "kernel")
				payloadChain := osChains[2]
				assert.Equal(t, []string{"hello"}, payloadChain.Include)
				if installFrom {
					// the bp repo got added for this payload resolv
					assert.Equal(t, 1, len(payloadChain.Repositories))
					expected := []rpmmd.RepoConfig{
						{Id: "repo2", BaseURLs: []string{"example.com/repo2"}, GPGKeys: []string{}},
					}
					assert.Equal(t, expected, payloadChain.Repositories)
				} else {
					// we configured no base repos and the bp repo is not included
					assert.Equal(t, 0, len(payloadChain.Repositories))
				}
			})
		}
	}
}
