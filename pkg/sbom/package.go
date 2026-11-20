package sbom

import "github.com/osbuild/image-builder/pkg/rpmmd"

// Package is the subset of an RPM record an SBOM is built from. It
// omits fields not needed by the SBOM.
type Package struct {
	Name     string
	Epoch    uint
	Version  string
	Release  string
	Arch     string
	Vendor   string
	URL      string
	Checksum rpmmd.Checksum
}

// PackageFromRPMMD copies the SBOM fields from an rpmmd.Package and
// drops the rest.
func PackageFromRPMMD(p rpmmd.Package) Package {
	return Package{
		Name:     p.Name,
		Epoch:    p.Epoch,
		Version:  p.Version,
		Release:  p.Release,
		Arch:     p.Arch,
		Vendor:   p.Vendor,
		URL:      p.URL,
		Checksum: p.Checksum,
	}
}
