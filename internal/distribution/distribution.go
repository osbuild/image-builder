package distribution

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/labstack/echo/v4"
)

var DistributionNotFound = errors.New("Distribution not available")
var RepoSourceError = errors.New("Repository must always have one of these properties: baseurl, metalink")
var MajorMinorError = errors.New("Unable to get major and minor version for distribution")

type DistributionItem struct {
	Description      string  `json:"description"`
	Name             string  `json:"name"`
	ComposerName     *string `json:"composer_name"`
	RestrictedAccess bool    `json:"restricted_access"`

	// NoPackageList is set to true for distributions that don't have their
	// packages defined in /distributions. This is useful for distributions
	// that are not visible in the UI and their package lists are huge.
	// This is very useful for Fedora.
	NoPackageList bool `json:"no_package_list"`
}

type DistributionFile struct {
	ModulePlatformID string           `json:"module_platform_id"`
	Distribution     DistributionItem `json:"distribution"`
	ArchX86          *Architecture    `json:"x86_64,omitempty"`
	Aarch64          *Architecture    `json:"aarch64,omitempty"`
	OscapDatastream  string           `json:"oscap_datastream"`
}

type Architecture struct {
	ImageTypes   []string     `json:"image_types"`
	Repositories []Repository `json:"repositories"`

	// not part of distro.json, loaded dynamically in ReadDistribution
	Packages map[string][]Package
}

type Repository struct {
	Id            string   `json:"id"`
	Baseurl       *string  `json:"baseurl"`
	Metalink      *string  `json:"metalink"`
	GpgKey        *string  `json:"gpgkey"`
	CheckGpg      *bool    `json:"check_gpg"`
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

func (dist DistributionFile) RHELMajorMinor() (int, int, error) {
	splitHyphen := strings.Split(dist.Distribution.Name, "-")
	if len(splitHyphen) != 2 || splitHyphen[0] != "rhel" {
		return 0, 0, MajorMinorError
	}

	version := splitHyphen[1]
	if len(version) < 2 || len(version) > 4 {
		return 0, 0, MajorMinorError
	}

	if strings.Contains(version, ".") {
		splitDot := strings.Split(version, ".")
		major, err := strconv.Atoi(splitDot[0])
		if err != nil {
			return 0, 0, fmt.Errorf("%w: %w", MajorMinorError, err)
		}
		minor, err := strconv.Atoi(splitDot[1])
		if err != nil {
			return major, 0, fmt.Errorf("%w: %w", MajorMinorError, err)
		}
		return major, minor, nil
	}

	major, err := strconv.Atoi(version[:1])
	if err != nil {
		return 0, 0, fmt.Errorf("%w: %w", MajorMinorError, err)
	}
	minor, err := strconv.Atoi(version[1:])
	if err != nil {
		return major, 0, fmt.Errorf("%w: %w", MajorMinorError, err)
	}
	return major, minor, nil
}

func (dist DistributionFile) Architecture(arch string) (*Architecture, error) {
	switch arch {
	case "x86_64":
		return dist.ArchX86, nil
	case "aarch64":
		return dist.Aarch64, nil
	default:
		return nil, echo.NewHTTPError(http.StatusBadRequest, "Architecture not supported")
	}
}

func (arch Architecture) FindPackages(search string) []Package {
	if arch.Packages == nil {
		return nil
	}

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

func (arch Architecture) validate() error {
	for _, r := range arch.Repositories {
		sourceSet := false
		if r.Baseurl != nil {
			sourceSet = true
		}

		if r.Metalink != nil {
			if sourceSet {
				return RepoSourceError
			}
			sourceSet = true
		}

		if !sourceSet {
			return RepoSourceError
		}
	}

	return nil
}

func allDistributions(distsDir string) ([]string, error) {
	files, err := os.ReadDir(distsDir)
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

func readDistribution(distsDir, distroIn string) (d DistributionFile, err error) {
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

	if err = d.ArchX86.validate(); err != nil {
		return
	}

	if !d.Distribution.NoPackageList {
		var x86Pkgs map[string][]Package
		x86Pkgs, err = readPackages(d.ArchX86.Repositories, "x86_64", distsDir, distroIn)
		if err != nil {
			return
		}
		d.ArchX86.Packages = x86Pkgs

		var aarch64 map[string][]Package
		aarch64, err = readPackages(d.Aarch64.Repositories, "aarch64", distsDir, distroIn)
		if err != nil {
			return
		}
		d.Aarch64.Packages = aarch64
	}

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
