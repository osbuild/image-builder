package manifestmock

import (
	"cmp"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/url"
	"slices"
	"strconv"
	"strings"

	"github.com/osbuild/images/internal/buildconfig"
	"github.com/osbuild/images/pkg/container"
	"github.com/osbuild/images/pkg/depsolvednf"
	"github.com/osbuild/images/pkg/flatpak"
	"github.com/osbuild/images/pkg/ostree"
	"github.com/osbuild/images/pkg/rpmmd"
	"github.com/osbuild/images/pkg/sbom"
)

func ResolveContainers(containerSources map[string][]container.SourceSpec) map[string][]container.Spec {
	containerSpecs := make(map[string][]container.Spec, len(containerSources))
	for plName, sourceSpecs := range containerSources {
		specs := make([]container.Spec, len(sourceSpecs))
		for idx, src := range sourceSpecs {
			digest := fmt.Sprintf("sha256:%x", sha256.Sum256([]byte(src.Name+src.Source+"digest")))
			id := fmt.Sprintf("sha256:%x", sha256.Sum256([]byte(src.Name+src.Source+"imageid")))
			listDigest := fmt.Sprintf("sha256:%x", sha256.Sum256([]byte(src.Name+src.Source+"list-digest")))
			name := src.Name
			if name == "" {
				name = src.Source
			}
			spec := container.Spec{
				Source:       src.Source,
				Digest:       digest,
				TLSVerify:    src.TLSVerify,
				ImageID:      id,
				LocalName:    name,
				ListDigest:   listDigest,
				LocalStorage: src.Local,
			}
			specs[idx] = spec
		}
		containerSpecs[plName] = specs
	}
	return containerSpecs
}

func ResolveCommits(commitSources map[string][]ostree.SourceSpec) map[string][]ostree.CommitSpec {
	commits := make(map[string][]ostree.CommitSpec, len(commitSources))
	for name, commitSources := range commitSources {
		commitSpecs := make([]ostree.CommitSpec, len(commitSources))
		for idx, commitSource := range commitSources {
			commitSpecs[idx] = mockOSTreeResolve(commitSource)
		}
		commits[name] = commitSpecs
	}
	return commits
}

func ResolveFlatpaks(flatpakSources map[string][]flatpak.SourceSpec) map[string][]flatpak.Spec {
	flatpakSpecs := make(map[string][]flatpak.Spec, len(flatpakSources))

	for plName, sourceSpecs := range flatpakSources {
		specs := make([]flatpak.Spec, len(sourceSpecs))
		for idx, src := range sourceSpecs {
			base := fmt.Sprintf(
				"%d%s%s%s",
				src.Registry.Type,
				src.Registry.RemoteName,
				src.Registry.URI,
				src.Reference.String())

			digest := fmt.Sprintf("sha256:%x", sha256.Sum256([]byte(base+"digest")))
			id := fmt.Sprintf("sha256:%x", sha256.Sum256([]byte(base+"imageid")))

			specs[idx] = flatpak.Spec{}

			if src.Registry.Type == flatpak.REGISTRY_TYPE_OCI {
				specs[idx].ContainerSpec = &container.Spec{
					Source:  src.Reference.String(),
					Digest:  digest,
					ImageID: id,
				}
			} else {
				panic("non-implemented registry type for flatpak")
			}
		}

		flatpakSpecs[plName] = specs
	}

	return flatpakSpecs
}

func Depsolve(
	packageSets map[string][]rpmmd.PackageSet,
	archName string,
	bc *buildconfig.BuildConfig,
	generateSBOM bool,
) (map[string]depsolvednf.DepsolveResult, error) {
	useRootDir := bc != nil && bc.Solver != nil && bc.Solver.UseRootDir
	depsolvedSets := make(map[string]depsolvednf.DepsolveResult)

	// NOTE: We need to simulate that the requested GPG keys that are to be imported
	// from the filesystem are actually provided by the package sets. Otherwise,
	// generating the test manifests without depsolving would fail. Define all the
	// common GPG keys imported by our base image definitions and and merge them with
	// the GPG keys from the blueprint customizations. If a new base image type imports
	// additional GPG keys, add them to this list.
	gpgKeysFromTree := []string{
		"/etc/pki/rpm-gpg/RPM-GPG-KEY-redhat-release",
		"/etc/pki/rpm-gpg/RPM-GPG-KEY-microsoft-azure-release",
	}
	if bc != nil && bc.Blueprint != nil && bc.Blueprint.Customizations != nil &&
		bc.Blueprint.Customizations.RPM != nil && bc.Blueprint.Customizations.RPM.ImportKeys != nil {
		gpgKeysFromTree = slices.Concat(gpgKeysFromTree, bc.Blueprint.Customizations.RPM.ImportKeys.Files)
	}

	for pkgSetName, pkgSetChain := range packageSets {
		transactions := make(depsolvednf.TransactionList, 0, len(pkgSetChain))
		reposById := make(map[string]rpmmd.RepoConfig)

		// NOTE: Unlike the real depsolver, the Repo pointers assigned to packages
		// don't point into DepsolveResult.Repos. This is intentional for simplicity -
		// the mock only needs non-nil, deterministic Repo assignments for test manifest
		// generation. Nothing checks pointer identity with the Repos slice.

		// Each PackageSet in the chain represents a single transaction.
		for txIdx, pkgSet := range pkgSetChain {

			if useRootDir {
				if len(pkgSet.Repositories) > 0 {
					return nil, fmt.Errorf("package set %s has repositories when useRootDir is true", pkgSetName)
				}
				// We don't have any way to properly leverage the root dir for this mock, so we just
				// add a dummy repository to the package set.
				pkgSet.Repositories = []rpmmd.RepoConfig{
					{
						Name:     "root-dir-repo",
						BaseURLs: []string{"https://example.com/root_dir_repo"},
					},
				}
			}

			// Normalize repo IDs: if Id is not set, use Hash()
			for i := range pkgSet.Repositories {
				if pkgSet.Repositories[i].Id == "" {
					pkgSet.Repositories[i].Id = pkgSet.Repositories[i].Hash()
				}
			}

			transactionPackages := make(
				rpmmd.PackageList, 0,
				len(pkgSet.Include)+len(pkgSet.Exclude)+len(pkgSet.Repositories)+1,
			)

			for _, pkgName := range pkgSet.Include {
				// Generate a unique package checksum, so that the same included package name from different
				// transactions are not considered the same package. This allows us to catch changes in the default
				// package sets when generating test manifests.
				checksum := fmt.Sprintf(
					"%x",
					sha256.Sum256([]byte(fmt.Sprintf("pkgset:%s_trans:%d_include:%s", pkgSetName, txIdx, pkgName))),
				)
				// Assign repos to packages using round-robin across the transaction's
				// repositories. This ensures deterministic output for test manifest
				// diffing and distributes packages across repos similar to how a real
				// depsolver would source packages from multiple repositories.
				// Use package name hash for deterministic selection.
				repoIdx := int(sha256.Sum256([]byte(pkgName))[0]) % len(pkgSet.Repositories)
				pkgRepo := pkgSet.Repositories[repoIdx]
				pkg := rpmmd.Package{
					// NOTE: for included packages, we use the plain package name, because some pipeline generators
					// are searching the depsolved package set for specific package names (such as 'kernel')
					// and fail if they are not found.
					Name: pkgName,
					// generate predictable but non-empty release/version numbers
					// NOTE: we can't use version higher than 4, because the OS pipeline's
					// GenDNF4VersionlockStageOptions() searches for packages with version "4"
					// to identify DNF4-related packages.
					Version:  strconv.Itoa(int(checksum[0]) % 5),
					Release:  fmt.Sprintf("%d.pkgset~%s^trans~%d", int(checksum[1])%9, pkgSetName, txIdx),
					Arch:     archName,
					Checksum: rpmmd.Checksum{Type: "sha256", Value: checksum},
					RepoID:   pkgRepo.Id,
					Repo:     &pkgRepo,
					CheckGPG: pkgRepo.CheckGPG != nil && *pkgRepo.CheckGPG,
				}
				pkg.Location = fmt.Sprintf("packages/%s.rpm", pkg.FullNEVRA())
				pkg.RemoteLocations = []string{
					fmt.Sprintf("%s/%s", pkg.Repo.BaseURLs[0], pkg.Location),
				}
				transactionPackages = append(transactionPackages, pkg)
			}

			for _, pkgName := range pkgSet.Exclude {
				// Generate a unique package checksum, so that the same included package name from different
				// transactions are not considered the same package. This allows us to catch changes in the default
				// package sets when generating test manifests.
				checksum := fmt.Sprintf(
					"%x",
					sha256.Sum256([]byte(fmt.Sprintf("pkgset:%s_trans:%d_exclude:%s", pkgSetName, txIdx, pkgName))),
				)
				// Assign repos to packages using round-robin across the transaction's
				// repositories. This ensures deterministic output for test manifest
				// diffing and distributes packages across repos similar to how a real
				// depsolver would source packages from multiple repositories.
				// Use package name hash for deterministic selection.
				repoIdx := int(sha256.Sum256([]byte(pkgName))[0]) % len(pkgSet.Repositories)
				pkgRepo := pkgSet.Repositories[repoIdx]
				pkg := rpmmd.Package{
					Name: fmt.Sprintf("exclude:%s", pkgName),
					// generate predictable but non-empty release/version numbers
					Version:  strconv.Itoa(int(checksum[0]) % 9),
					Release:  fmt.Sprintf("%d.pkgset~%s^trans~%d", int(checksum[1])%9, pkgSetName, txIdx),
					Arch:     archName,
					Checksum: rpmmd.Checksum{Type: "sha256", Value: checksum},
					RepoID:   pkgRepo.Id,
					Repo:     &pkgRepo,
					CheckGPG: pkgRepo.CheckGPG != nil && *pkgRepo.CheckGPG,
				}
				pkg.Location = fmt.Sprintf("packages/%s.rpm", pkg.FullNEVRA())
				pkg.RemoteLocations = []string{
					fmt.Sprintf("%s/%s", pkg.Repo.BaseURLs[0], pkg.Location),
				}
				transactionPackages = append(transactionPackages, pkg)
			}

			// generate pseudo packages for the config of each transaction
			var setRepoNames []string
			for _, setRepo := range pkgSet.Repositories {
				setRepoNames = append(setRepoNames, setRepo.Name)
			}
			configPackageName := fmt.Sprintf("pkgset:%s_trans:%d_repos:%s", pkgSetName, txIdx, strings.Join(setRepoNames, "+"))
			if pkgSet.InstallWeakDeps {
				configPackageName += "+weak"
			}
			configPkgChecksum := fmt.Sprintf("%x", sha256.Sum256([]byte(configPackageName)))
			depsolveConfigPackage := rpmmd.Package{
				Name: configPackageName,
				// generate predictable but non-empty release/version numbers
				Version:  strconv.Itoa(int(configPkgChecksum[0]) % 9),
				Release:  strconv.Itoa(int(configPkgChecksum[1])%9) + ".fk1",
				Arch:     archName,
				Checksum: rpmmd.Checksum{Type: "sha256", Value: configPkgChecksum},
				RepoID:   pkgSet.Repositories[0].Id,
				Repo:     &pkgSet.Repositories[0],
			}
			depsolveConfigPackage.Location = fmt.Sprintf("packages/%s.rpm", depsolveConfigPackage.FullNEVRA())
			depsolveConfigPackage.RemoteLocations = []string{
				fmt.Sprintf("%s/%s", depsolveConfigPackage.Repo.BaseURLs[0], depsolveConfigPackage.Location),
			}
			// Since we can't make any assumptions about the package sets, we just add all the
			// GPG keys that should be imported from the tree to the first transaction's pseudo-package.
			if txIdx == 0 {
				depsolveConfigPackage.Files = gpgKeysFromTree
			}

			transactionPackages = append(transactionPackages, depsolveConfigPackage)

			// Add repo pseudo-packages only for repos not seen before
			for _, repo := range pkgSet.Repositories {
				if existingRepo, ok := reposById[repo.Id]; ok {
					if existingRepo.Hash() != repo.Hash() {
						return nil, fmt.Errorf("repo ID %q collision: different configs with same ID", repo.Id)
					}
					continue
				}
				reposById[repo.Id] = repo

				// the test repos have the form:
				//   https://rpmrepo..../el9/cs9-x86_64-rt-20240915
				// drop the date as it's not needed for this level of mocks
				baseURL := repo.BaseURLs[0]
				if idx := strings.LastIndex(baseURL, "-"); idx > 0 {
					baseURL = baseURL[:idx]
				}
				url, err := url.Parse(baseURL)
				if err != nil {
					panic(err)
				}
				url.Host = "example.com"
				url.Path = fmt.Sprintf("passed-arch:%s/passed-repo:%s", archName, url.Path)
				checksum := fmt.Sprintf("%x", sha256.Sum256([]byte(url.String())))
				transactionPackages = append(transactionPackages, rpmmd.Package{
					Name: url.String(),
					// generate predictable but non-empty release/version numbers
					Version:         strconv.Itoa(int(checksum[0]) % 9),
					Release:         strconv.Itoa(int(checksum[1])%9) + ".fk1",
					Arch:            archName,
					Location:        url.Path,
					RemoteLocations: []string{url.String()},
					Checksum:        rpmmd.Checksum{Type: "sha256", Value: checksum},
					RepoID:          repo.Id,
					Repo:            &repo,
				})
			}

			// The packages list in each transaction result is sorted by full NEVRA, as a real depsolver would do.
			slices.SortFunc(transactionPackages, func(a, b rpmmd.Package) int {
				return cmp.Compare(a.FullNEVRA(), b.FullNEVRA())
			})
			transactions = append(transactions, transactionPackages)
		}

		// Sort the list of repos by ID, as a real depsolver would do.
		allRepos := make([]rpmmd.RepoConfig, 0, len(reposById))
		for _, repo := range reposById {
			allRepos = append(allRepos, repo)
		}
		slices.SortFunc(allRepos, func(a, b rpmmd.RepoConfig) int {
			return cmp.Compare(a.Id, b.Id)
		})

		var sbomDoc *sbom.Document
		if generateSBOM {
			var err error
			sbomDoc, err = sbom.NewDocument(
				sbom.StandardTypeSpdx,
				json.RawMessage(fmt.Sprintf(`{"sbom-for":"%s"}`, pkgSetName)),
			)
			if err != nil {
				return nil, err
			}
		}

		depsolvedSets[pkgSetName] = depsolvednf.DepsolveResult{
			Transactions: transactions,
			Repos:        allRepos,
			SBOM:         sbomDoc,
		}
	}

	return depsolvedSets, nil
}

var OSTreeResolve = mockOSTreeResolve

func mockOSTreeResolve(commitSource ostree.SourceSpec) ostree.CommitSpec {
	checksum := fmt.Sprintf("%x", sha256.Sum256([]byte(commitSource.URL+commitSource.Ref)))
	spec := ostree.CommitSpec{
		Ref:      commitSource.Ref,
		URL:      commitSource.URL,
		Checksum: checksum,
	}
	if commitSource.RHSM {
		spec.Secrets = "org.osbuild.rhsm.consumer"
	}
	return spec
}
