package distro_test

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"slices"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/osbuild/blueprint/pkg/blueprint"
	"github.com/osbuild/images/internal/common"
	"github.com/osbuild/images/pkg/container"
	"github.com/osbuild/images/pkg/distro"
	"github.com/osbuild/images/pkg/distrofactory"
	"github.com/osbuild/images/pkg/flatpak"
	"github.com/osbuild/images/pkg/manifestgen/manifestmock"
	"github.com/osbuild/images/pkg/ostree"
	"github.com/osbuild/images/pkg/rpmmd"
	testrepos "github.com/osbuild/images/test/data/repositories"
)

// listTestedDistros returns a list of distro names that are explicitly tested
func listTestedDistros(t *testing.T) []string {
	testRepoRegistry, err := testrepos.New()
	require.Nil(t, err)
	require.NotEmpty(t, testRepoRegistry)
	distros := testRepoRegistry.ListDistros()
	require.NotEmpty(t, distros)
	return distros
}

// Ensure all image types report the correct names for their pipelines.
// Each image type contains a list of build and payload pipelines. They are
// needed for knowing the names of pipelines from the static object without
// having access to a manifest, which we need when parsing metadata from build
// results.
// NOTE: The static list of pipelines really only needs to include those that
// have rpm or ostree metadata in them.
func TestImageTypePipelineNames(t *testing.T) {
	// types for parsing the opaque manifest with just the fields we care about
	type rpmStageOptions struct {
		GPGKeys []string `json:"gpgkeys"`
	}
	type stage struct {
		Type    string          `json:"type"`
		Options rpmStageOptions `json:"options"`
	}
	type pipeline struct {
		Name   string  `json:"name"`
		Stages []stage `json:"stages"`
	}
	type manifest struct {
		Pipelines []pipeline `json:"pipelines"`
	}

	distroFactory := distrofactory.NewDefault()
	distros := listTestedDistros(t)
	for _, distroName := range distros {
		d := distroFactory.GetDistro(distroName)
		for _, archName := range d.ListArches() {
			arch, err := d.GetArch(archName)
			assert.Nil(t, err)
			for _, imageTypeName := range arch.ListImageTypes() {
				t.Run(fmt.Sprintf("%s/%s/%s", distroName, archName, imageTypeName), func(t *testing.T) {
					t.Parallel()
					assert := assert.New(t)
					imageType, err := arch.GetImageType(imageTypeName)
					assert.Nil(err)

					// set up bare minimum args for image type
					var customizations *blueprint.Customizations
					if imageType.Name() == "edge-simplified-installer" || imageType.Name() == "iot-simplified-installer" {
						customizations = &blueprint.Customizations{
							InstallationDevice: "/dev/null",
						}
					}
					bp := blueprint.Blueprint{
						Customizations: customizations,
					}
					options := distro.ImageOptions{}
					// This base repo's GPG keys should get included in the
					// OS pipeline's RPM stage since packages are resolved
					// from it. The repo has no PackageSets filter, so it
					// applies to all package sets.
					repos := []rpmmd.RepoConfig{
						{
							Name:     "baseos",
							BaseURLs: []string{"http://baseos.example.com"},
							GPGKeys:  []string{"baseos-gpg-key"},
							CheckGPG: common.ToPtr(true),
						},
					}
					seed := int64(0)

					// Add ostree options for image types that require them
					if imageType.OSTreeRef() != "" {
						options.OSTree = &ostree.ImageOptions{
							URL: "https://example.com",
						}
					}

					m, _, err := imageType.Manifest(&bp, options, repos, &seed)
					assert.NoError(err)

					containers := make(map[string][]container.Spec, 0)
					flatpaks := make(map[string][]flatpak.Spec, 0)

					// Pipelines that require content (packages, ostree
					// commits) will fail if none are defined. OS pipelines
					// require a kernel that matches the kernel name defined
					// for the image (or 'kernel' if it's not defined).
					// Get the content and "fake resolve" it to pass to
					// Serialize().
					packageSets, err := m.GetPackageSetChains()
					assert.NoError(err)
					depsolvedSets, err := manifestmock.Depsolve(packageSets, archName, nil, false)
					assert.NoError(err)

					ostreeSources := m.GetOSTreeSourceSpecs()
					commits := make(map[string][]ostree.CommitSpec, len(ostreeSources))
					for name, commitSources := range ostreeSources {
						commitSpecs := make([]ostree.CommitSpec, len(commitSources))
						for idx, commitSource := range commitSources {
							commitSpecs[idx] = ostree.CommitSpec{
								Ref:      commitSource.Ref,
								URL:      commitSource.URL,
								Checksum: fmt.Sprintf("%x", sha256.Sum256([]byte(commitSource.URL+commitSource.Ref))),
							}
						}
						commits[name] = commitSpecs
					}

					mf, err := m.Serialize(depsolvedSets, containers, commits, flatpaks, nil)
					assert.NoError(err)
					pm := new(manifest)
					err = json.Unmarshal(mf, pm)
					assert.NoError(err)

					var pmNames []string
					for idx := range pm.Pipelines {
						// Gather the names of the manifest piplines for later
						pmNames = append(pmNames, pm.Pipelines[idx].Name)

						if pm.Pipelines[idx].Name == "os" {
							rpmStagePresent := false
							for _, s := range pm.Pipelines[idx].Stages {
								if s.Type == "org.osbuild.rpm" {
									rpmStagePresent = true
									if imageTypeName != "azure-eap7-rhui" {
										// NOTE (akoutsou): Ideally, at some point we will
										// have a good way of reading what's supported by
										// each image type and we can skip or adapt tests
										// based on this information. For image types with
										// a preset workload, payload packages are ignored
										// and dropped and so are the payload
										// repo gpg keys.
										assert.Equal(repos[0].GPGKeys, s.Options.GPGKeys)
									}
								}
							}
							// make sure the gpg keys check was reached
							assert.True(rpmStagePresent)
						}
					}

					// The last pipeline should match the export pipeline.
					// This might change in the future, but for now, let's make
					// sure they match.
					assert.Equal(imageType.Exports()[0], pm.Pipelines[len(pm.Pipelines)-1].Name)

					// The pipelines named in allPipelines must exist in the manifest, and in the
					// order specified (eg. 'build' first) but it does not need to be an exact
					// match. Only the pipelines with rpm or ostree metadata are required.
					var order int
					allPipelines := append(m.BuildPipelines(), m.PayloadPipelines()...)
					for _, name := range allPipelines {
						idx := slices.Index(pmNames, name)
						assert.True(idx >= order, "%s not in order %v", name, pmNames)
						order = idx
					}
				})
			}
		}
	}
}

// Ensure repositories are assigned to package sets properly.
//
// Each package set should include all the global repositories as well as any
// pipeline/package-set specific repositories.
func TestPipelineRepositories(t *testing.T) {
	type testCase struct {
		// Repo configs for pipeline generator
		repos []rpmmd.RepoConfig

		// Expected result: map of pipelines to repo names (we only check names for the test).
		// Use the pipeline name * for global repos.
		result map[string][]stringSet
	}

	testCases := map[string]testCase{
		"globalonly": { // only global repos: most common scenario
			repos: []rpmmd.RepoConfig{
				{
					Name:     "global-1",
					BaseURLs: []string{"http://global-1.example.com"},
				},
				{
					Name:     "global-2",
					BaseURLs: []string{"http://global-2.example.com"},
				},
			},
			result: map[string][]stringSet{
				"*": {newStringSet([]string{"global-1", "global-2"})},
			},
		},
		"global+build": { // global repos with build-specific repos: secondary common scenario
			repos: []rpmmd.RepoConfig{
				{
					Name:     "global-11",
					BaseURLs: []string{"http://global-11.example.com"},
				},
				{
					Name:     "global-12",
					BaseURLs: []string{"http://global-12.example.com"},
				},
				{
					Name:        "build-1",
					BaseURLs:    []string{"http://build-1.example.com"},
					PackageSets: []string{"build"},
				},
				{
					Name:        "build-2",
					BaseURLs:    []string{"http://build-2.example.com"},
					PackageSets: []string{"build"},
				},
			},
			result: map[string][]stringSet{
				"*":     {newStringSet([]string{"global-11", "global-12"})},
				"build": {newStringSet([]string{"build-1", "build-2"})},
			},
		},
		"global+os": { // global repos with os-specific repos
			repos: []rpmmd.RepoConfig{
				{
					Name:     "global-21",
					BaseURLs: []string{"http://global-11.example.com"},
				},
				{
					Name:     "global-22",
					BaseURLs: []string{"http://global-12.example.com"},
				},
				{
					Name:        "os-1",
					BaseURLs:    []string{"http://os-1.example.com"},
					PackageSets: []string{"os"},
				},
				{
					Name:        "os-2",
					BaseURLs:    []string{"http://os-2.example.com"},
					PackageSets: []string{"os"},
				},
			},
			result: map[string][]stringSet{
				"*":  {newStringSet([]string{"global-21", "global-22"})},
				"os": {newStringSet([]string{"os-1", "os-2"}), newStringSet([]string{"os-1", "os-2"}), newStringSet([]string{"os-1", "os-2"})},
			},
		},
		"global+os+payload": { // global repos with os-specific repos and (user-defined) payload repositories
			repos: []rpmmd.RepoConfig{
				{
					Name:     "global-21",
					BaseURLs: []string{"http://global-11.example.com"},
				},
				{
					Name:     "global-22",
					BaseURLs: []string{"http://global-12.example.com"},
				},
				{
					Name:        "os-1",
					BaseURLs:    []string{"http://os-1.example.com"},
					PackageSets: []string{"os"},
				},
				{
					Name:        "os-2",
					BaseURLs:    []string{"http://os-2.example.com"},
					PackageSets: []string{"os"},
				},
				{
					Name:     "payload",
					BaseURLs: []string{"http://payload.example.com"},
					// User-defined payload repositories automatically get the "blueprint" key.
					// This is handled by the APIs.
					PackageSets: []string{"blueprint"},
				},
			},
			result: map[string][]stringSet{
				"*": {newStringSet([]string{"global-21", "global-22"})},
				"os": {
					// chain with payload repo only in the third set for the blueprint package depsolve
					newStringSet([]string{"os-1", "os-2"}),
					newStringSet([]string{"os-1", "os-2"}),
					newStringSet([]string{"os-1", "os-2", "payload"}),
				},
			},
		},
		"noglobal": { // no global repositories; only pipeline restricted ones (unrealistic but technically valid)
			repos: []rpmmd.RepoConfig{
				{
					Name:        "build-1",
					BaseURLs:    []string{"http://build-1.example.com"},
					PackageSets: []string{"build"},
				},
				{
					Name:        "build-2",
					BaseURLs:    []string{"http://build-2.example.com"},
					PackageSets: []string{"build"},
				},
				{
					Name:        "os-1",
					BaseURLs:    []string{"http://os-1.example.com"},
					PackageSets: []string{"os"},
				},
				{
					Name:        "os-2",
					BaseURLs:    []string{"http://os-2.example.com"},
					PackageSets: []string{"os"},
				},
				{
					Name:        "anaconda-1",
					BaseURLs:    []string{"http://anaconda-1.example.com"},
					PackageSets: []string{"anaconda-tree"},
				},
				{
					Name:        "container-1",
					BaseURLs:    []string{"http://container-1.example.com"},
					PackageSets: []string{"container-tree"},
				},
				{
					Name:        "coi-1",
					BaseURLs:    []string{"http://coi-1.example.com"},
					PackageSets: []string{"coi-tree"},
				},
			},
			result: map[string][]stringSet{
				"*":              nil,
				"build":          {newStringSet([]string{"build-1", "build-2"})},
				"os":             {newStringSet([]string{"os-1", "os-2"}), newStringSet([]string{"os-1", "os-2"}), newStringSet([]string{"os-1", "os-2"})},
				"anaconda-tree":  {newStringSet([]string{"anaconda-1"})},
				"container-tree": {newStringSet([]string{"container-1"})},
				"coi-tree":       {newStringSet([]string{"coi-1"})},
			},
		},
		"global+unknown": { // package set names that don't match a pipeline are ignored
			repos: []rpmmd.RepoConfig{
				{
					Name:     "global-1",
					BaseURLs: []string{"http://global-1.example.com"},
				},
				{
					Name:     "global-2",
					BaseURLs: []string{"http://global-2.example.com"},
				},
				{
					Name:        "custom-1",
					BaseURLs:    []string{"http://custom.example.com"},
					PackageSets: []string{"notapipeline"},
				},
			},
			result: map[string][]stringSet{
				"*": {newStringSet([]string{"global-1", "global-2"})},
			},
		},
		"none": { // empty
			repos:  []rpmmd.RepoConfig{},
			result: map[string][]stringSet{},
		},
	}

	distroFactory := distrofactory.NewDefault()
	distros := listTestedDistros(t)
	for tName, tCase := range testCases {
		t.Run(tName, func(t *testing.T) {
			t.Parallel()
			for _, distroName := range distros {
				d := distroFactory.GetDistro(distroName)
				for _, archName := range d.ListArches() {
					arch, err := d.GetArch(archName)
					require.Nil(t, err)
					for _, imageTypeName := range arch.ListImageTypes() {
						if imageTypeName == "azure-eap7-rhui" {
							// NOTE (akoutsou): Ideally, at some point we will
							// have a good way of reading what's supported by
							// each image type and we can skip or adapt tests
							// based on this information. For image types with
							// a preset workload, payload packages are ignored
							// and dropped.
							// This is now supported in Fedora. RHEL coming
							// soon.
							continue
						}
						t.Run(fmt.Sprintf("%s/%s/%s", distroName, archName, imageTypeName), func(t *testing.T) {
							t.Parallel()
							require := require.New(t)
							imageType, err := arch.GetImageType(imageTypeName)
							require.Nil(err)
							// skip any image type that does not support custom packages
							if !slices.Contains(imageType.SupportedBlueprintOptions(), "packages") {
								return
							}

							// set up bare minimum args for image type
							var customizations *blueprint.Customizations
							if imageType.Name() == "edge-simplified-installer" || imageType.Name() == "iot-simplified-installer" {
								customizations = &blueprint.Customizations{
									InstallationDevice: "/dev/null",
								}
							}
							bp := blueprint.Blueprint{
								Customizations: customizations,
								Packages: []blueprint.Package{
									{Name: "filesystem"},
								},
							}
							options := distro.ImageOptions{}

							// Add ostree options for image types that require them
							if imageType.OSTreeRef() != "" {
								options.OSTree = &ostree.ImageOptions{
									URL: "https://example.com",
								}
							}

							repos := tCase.repos
							manifest, _, err := imageType.Manifest(&bp, options, repos, nil)
							require.NoError(err)
							packageSets, err := manifest.GetPackageSetChains()
							assert.NoError(t, err)

							var globals stringSet
							if len(tCase.result["*"]) > 0 {
								globals = tCase.result["*"][0]
							}

							// NOTE: Image types that don't use any customizations in their base
							// definition do not install any customization packages (e.g., SELinux
							// policy, subscription-manager, etc.). As a result, they have only 2
							// package sets in the OS chain (base packages and blueprint packages).
							// Other image types have 3. These are typically minimal/container
							// image types.
							// NOTE: This list must be updated when new image types are added that
							// don't use any customizations in their base definition.
							noCustomizationPkgsImageTypes := []string{
								"container",
								"container-minimal",
								"generic-container",
								"wsl",
								"generic-wsl",
							}

							for psName, psChain := range packageSets {
								// test run in parallel but expChain is mutated during the test so we need a clone
								expChain := slices.Clone(tCase.result[psName])

								// Adjust expected chain for image types without any customization packages
								if psName == "os" && len(expChain) == 3 && slices.Contains(noCustomizationPkgsImageTypes, imageTypeName) {
									// Remove the middle entry (customization packages) from expected chain
									expChain = []stringSet{expChain[0], expChain[2]}
								}

								if len(expChain) > 0 {
									// if we specified an expected chain it should match the returned.
									if len(expChain) != len(psChain) {
										t.Fatalf("expected %d package sets in the %q chain; got %d", len(expChain), psName, len(psChain))
									}
								} else {
									// if we didn't, initialise to empty before merging globals
									expChain = make([]stringSet, len(psChain))
								}

								for idx := range expChain {
									// merge the globals into each expected set
									expChain[idx] = expChain[idx].Merge(globals)
								}

								for setIdx, set := range psChain {
									// collect repositories in the package set
									repoNamesSet := newStringSet(nil)
									for _, repo := range set.Repositories {
										repoNamesSet.Add(repo.Name)
									}

									// expected set for current package set should be merged with globals
									expected := expChain[setIdx]

									if !repoNamesSet.Equals(expected) {
										t.Errorf("repos for package set %q [idx: %d] %s (distro %q image type %q) do not match expected %s", psName, setIdx, repoNamesSet, d.Name(), imageType.Name(), expected)
									}
								}
							}
						})
					}
				}
			}
		})
	}
}

// a very basic implementation of a Set of strings
type stringSet struct {
	elems map[string]bool
}

func newStringSet(init []string) stringSet {
	s := stringSet{elems: make(map[string]bool)}
	for _, elem := range init {
		s.Add(elem)
	}
	return s
}

func (s stringSet) String() string {
	elemSlice := make([]string, 0, len(s.elems))
	for elem := range s.elems {
		elemSlice = append(elemSlice, elem)
	}
	return "{" + strings.Join(elemSlice, ", ") + "}"
}

func (s stringSet) Add(elem string) {
	s.elems[elem] = true
}

func (s stringSet) Contains(elem string) bool {
	return s.elems[elem]
}

func (s stringSet) Equals(other stringSet) bool {
	if len(s.elems) != len(other.elems) {
		return false
	}

	for elem := range s.elems {
		if !other.Contains(elem) {
			return false
		}
	}

	return true
}

func (s stringSet) Merge(other stringSet) stringSet {
	merged := newStringSet(nil)
	for elem := range s.elems {
		merged.Add(elem)
	}
	for elem := range other.elems {
		merged.Add(elem)
	}
	return merged
}

// Check that Manifest() function returns an warning when FIPS
// customization is enabled and the host is not FIPS
func TestDistro_ManifestFIPSWarning(t *testing.T) {
	ostreeImages := []string{
		"edge-installer",
		"edge-raw-image",
		"edge-ami",
		"edge-vsphere",
		"edge-simplified-installer",
		"edge-qcow2-image",
		"iot-installer",
		"iot-raw-xz",
		"iot-simplified-installer",
		"iot-qcow2",
		"kinoite-installer",
		"silverblue-installer",
		"sway-atomic-installer",
		"budgie-atomic-installer",
		"cosmic-atomic-installer",
	}

	distroFactory := distrofactory.NewDefault()
	distros := listTestedDistros(t)
	for _, distroName := range distros {
		// FIPS blueprint customization is not supported for RHEL 7 images
		if strings.HasPrefix(distroName, "rhel-7") {
			continue
		}
		d := distroFactory.GetDistro(distroName)
		require.NotNil(t, d)

		fips_enabled := true
		msg := common.FIPSEnabledImageWarning + "\n"

		for _, archName := range d.ListArches() {
			arch, _ := d.GetArch(archName)
			for _, imgTypeName := range arch.ListImageTypes() {
				t.Run(fmt.Sprintf("fips:%s/%s/%s", distroName, archName, imgTypeName), func(t *testing.T) {
					t.Parallel()

					bp := blueprint.Blueprint{
						Customizations: &blueprint.Customizations{
							FIPS: &fips_enabled,
						},
					}
					imgType, _ := arch.GetImageType(imgTypeName)
					imgOpts := distro.ImageOptions{
						Size: imgType.Size(0),
					}
					if slices.Contains(ostreeImages, imgTypeName) {
						imgOpts.OSTree = &ostree.ImageOptions{URL: "http://localhost/repo"}
					}
					if strings.HasSuffix(imgTypeName, "simplified-installer") {
						bp.Customizations.InstallationDevice = "/dev/dummy"
					}
					_, warn, _ := imgType.Manifest(&bp, imgOpts, nil, nil)
					switch imgTypeName {
					case "workstation-live-installer", "container", "wsl", "tar":
						// NOTE (validation-warnings): blueprint validation errors have temporarily been converted to warnings
						assert.Contains(t, warn, fmt.Sprintf("blueprint validation failed for image type %q: customizations.fips: not supported", imgTypeName))
						assert.Equal(t, slices.Contains(warn, msg), !common.IsBuildHostFIPSEnabled(), "FIPS warning not shown for image: distro='%s', imgTypeName='%s', archName='%s', warn='%v'", distroName, imgTypeName, archName, warn)
					default:
						assert.Equal(t, slices.Contains(warn, msg), !common.IsBuildHostFIPSEnabled(),
							"FIPS warning not shown for image: distro='%s', imgTypeName='%s', archName='%s', warn='%v'", distroName, imgTypeName, archName, warn)
					}
				})
			}
		}
	}
}

// Test that passing options.OSTree for non-OSTree image types results in an error
func TestOSTreeOptionsErrorForNonOSTreeImgTypes(t *testing.T) {
	assert := assert.New(t)
	distroFactory := distrofactory.NewDefault()
	assert.NotNil(distroFactory)

	distros := listTestedDistros(t)
	assert.NotEmpty(distros)

	for _, distroName := range distros {
		d := distroFactory.GetDistro(distroName)
		assert.NotNil(d)

		arches := d.ListArches()
		assert.NotEmpty(arches)

		for _, archName := range arches {
			arch, err := d.GetArch(archName)
			assert.Nil(err)

			imgTypes := arch.ListImageTypes()
			assert.NotEmpty(imgTypes)

			for _, imageTypeName := range imgTypes {
				t.Run(fmt.Sprintf("%s/%s/%s", distroName, archName, imageTypeName), func(t *testing.T) {
					t.Parallel()
					imageType, err := arch.GetImageType(imageTypeName)
					assert.Nil(err)

					// set up bare minimum args for image type
					var customizations *blueprint.Customizations
					if imageType.Name() == "edge-simplified-installer" || imageType.Name() == "iot-simplified-installer" {
						customizations = &blueprint.Customizations{
							InstallationDevice: "/dev/null",
						}
					}
					bp := blueprint.Blueprint{
						Customizations: customizations,
					}
					options := distro.ImageOptions{
						OSTree: &ostree.ImageOptions{
							URL: "https://example.com",
						},
					}

					_, _, err = imageType.Manifest(&bp, options, nil, nil)
					if imageType.OSTreeRef() == "" {
						assert.Errorf(err,
							"OSTree options should not be allowed for non-OSTree image type %s/%s/%s",
							imageTypeName, archName, distroName)
					} else {
						assert.NoErrorf(err,
							"OSTree options should be allowed for OSTree image type %s/%s/%s",
							imageTypeName, archName, distroName)
					}
				})
			}
		}
	}
}

func TestGetImageTypes(t *testing.T) {
	assert := assert.New(t)
	distroFactory := distrofactory.NewDefault()
	assert.NotNil(distroFactory)

	distros := listTestedDistros(t)
	assert.NotEmpty(distros)

	for _, distroName := range distros {
		d := distroFactory.GetDistro(distroName)
		assert.NotNil(d)

		arches := d.ListArches()
		assert.NotEmpty(arches)

		for _, archName := range arches {
			imageTypes, err := distro.GetImageTypes(d, archName)
			assert.Nil(err)
			assert.NotEmpty(imageTypes)

			arch, err := d.GetArch(archName)
			assert.Nil(err)

			expectedNames := arch.ListImageTypes()
			assert.Len(imageTypes, len(expectedNames))

			for i, imageType := range imageTypes {
				assert.Equal(expectedNames[i], imageType.Name())
				assert.Equal(arch, imageType.Arch())
			}

		}
	}
}
