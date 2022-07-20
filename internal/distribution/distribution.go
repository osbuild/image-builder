package distribution

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/labstack/echo/v4"
)

var DistributionNotFound = errors.New("Distribution not available")

type DistributionItem struct {
	Description      string `json:"description"`
	Name             string `json:"name"`
	RestrictedAccess bool   `json:"restrictedAccess"`
}

type DistributionFile struct {
	ModulePlatformID string           `json:"module_platform_id"`
	Distribution     DistributionItem `json:"distribution"`
	ArchX86          *Architecture    `json:"x86_64,omitempty"`
}

type Architecture struct {
	ImageTypes   []string     `json:"image_types"`
	Repositories []Repository `json:"repositories"`

	// not part of distro.json, loaded dynamically in ReadDistribution
	Packages map[string][]Package
}

type Repository struct {
	Id            string   `json:"id"`
	Baseurl       string   `json:"baseurl"`
	Rhsm          bool     `json:"rhsm"`
	ImageTypeTags []string `json:"image_type_tags"`
}

type Package struct {
	Name    string `json:"name"`
	Summary string `json:"summary"`
}

type PackagesFile struct {
	Data []Package `json:"data"`
}

// entitlement is required if access to the repository is gated by
// Red Hat Subscription Manager
func (repo Repository) NeedsEntitlement() bool {
	return repo.Rhsm
}

// entitlement is required for a distro if it is for any of its
// repositories
func (dist DistributionFile) NeedsEntitlement() bool {
	for _, repo := range dist.ArchX86.Repositories {
		if repo.NeedsEntitlement() {
			return true
		}
	}
	return false
}

func (dist DistributionFile) IsRestricted() bool {
	return dist.Distribution.RestrictedAccess
}

func (dist DistributionFile) Architecture(arch string) (*Architecture, error) {
	switch arch {
	case "x86_64":
		return dist.ArchX86, nil
	default:
		return nil, echo.NewHTTPError(http.StatusBadRequest, "Architecture not supported")
	}
}

func (arch Architecture) FindPackages(search string) []Package {
	var pkgs []Package
	for _, r := range arch.Repositories {
		// Ignore repositories that do not apply to all for now
		if len(r.ImageTypeTags) > 0 {
			continue
		}

		ps := arch.Packages[r.Id]
		for _, p := range ps {
			if strings.Contains(p.Name, search) {
				pkgs = append(pkgs, p)
			}
		}
	}
	return pkgs
}

func allDistributions(distsDir string) ([]string, error) {
	files, err := ioutil.ReadDir(distsDir)
	if err != nil {
		return nil, err
	}
	var ds []string
	for _, f := range files {
		ds = append(ds, f.Name())
	}
	return ds, nil
}

func validDistribution(distsDir, distro string) (string, error) {
	allDistros, err := allDistributions(distsDir)
	if err != nil {
		return "", err
	}

	for _, d := range allDistros {
		if distro == d {
			return d, nil
		}
	}
	return "", DistributionNotFound
}

func ReadDistribution(distsDir, distroIn string) (d DistributionFile, err error) {
	distro, err := validDistribution(distsDir, distroIn)
	if err != nil {
		return
	}
	p, err := filepath.EvalSymlinks(filepath.Join(distsDir, distro))
	if err != nil {
		return
	}
	f, err := os.Open(filepath.Clean(filepath.Join(p, fmt.Sprintf("%s.json", filepath.Base(p)))))
	if err != nil {
		return
	}
	defer func() {
		err := f.Close()
		if err != nil {
			fmt.Printf("Error closing file: %v", err)
		}
	}()
	err = json.NewDecoder(f).Decode(&d)
	if err != nil {
		return
	}

	x86Pkgs, err := readPackages(d.ArchX86.Repositories, "x86_64", distsDir, distroIn)
	if err != nil {
		return
	}
	d.ArchX86.Packages = x86Pkgs

	return
}

func readPackages(repos []Repository, archName, distsDir, distroIn string) (map[string][]Package, error) {
	pkgs := make(map[string][]Package)
	for _, r := range repos {
		p, err := filepath.EvalSymlinks(filepath.Join(distsDir, distroIn))
		if err != nil {
			return nil, err
		}
		f, err := os.Open(filepath.Clean(filepath.Join(distsDir, distroIn, fmt.Sprintf("%s-%s-%s-packages.json", filepath.Base(p), archName, r.Id))))
		if err != nil {
			return nil, err
		}

		var ps []Package
		err = json.NewDecoder(f).Decode(&ps)
		if err != nil {
			return nil, err
		}

		pkgs[r.Id] = ps
	}

	return pkgs, nil
}

func RepositoriesForArch(distsDir, distro, arch string, is_entitled bool) ([]Repository, error) {
	distribution, err := ReadDistribution(distsDir, distro)
	if err != nil {
		return nil, err
	}

	if !is_entitled && distribution.NeedsEntitlement() {
		return nil, echo.NewHTTPError(http.StatusUnauthorized, "users organization is not entitled for distribution")
	}

	switch arch {
	case "x86_64":
		return distribution.ArchX86.Repositories, nil
	default:
		return nil, echo.NewHTTPError(http.StatusBadRequest, "Architecture not supported")
	}
}

func FindPackages(distsDir, distro, arch, search string, is_entitled bool) ([]Package, error) {
	repos, err := RepositoriesForArch(distsDir, distro, arch, is_entitled)
	if err != nil {
		return nil, err
	}

	var pkgs []Package
	for _, r := range repos {
		// Ignore repositories that do not apply to all for now
		if len(r.ImageTypeTags) > 0 {
			continue
		}
		p, err := filepath.EvalSymlinks(filepath.Join(distsDir, distro))
		if err != nil {
			return nil, err
		}
		f, err := os.Open(filepath.Clean(filepath.Join(distsDir, distro, fmt.Sprintf("%s-%s-%s-packages.json", filepath.Base(p), arch, r.Id))))
		if err != nil {
			return nil, err
		}

		var ps []Package
		err = json.NewDecoder(f).Decode(&ps)
		if err != nil {
			return nil, err
		}
		for _, p := range ps {
			if strings.Contains(p.Name, search) {
				pkgs = append(pkgs, p)
			}
		}
	}
	return pkgs, nil
}
