package distribution

import (
	"os"
)

// AllDistroRegistry holds all distribution that image-builder knows
// In order to access them, you need to call Available.
type AllDistroRegistry struct {
	distros map[string]*DistributionFile
}

// LoadDistroRegistry loads all distributions from distsDir
func LoadDistroRegistry(distsDir string) (*AllDistroRegistry, error) {
	files, err := os.ReadDir(distsDir)
	if err != nil {
		return nil, err
	}

	dr := &AllDistroRegistry{
		distros: make(map[string]*DistributionFile),
	}

	for _, f := range files {
		d, err := ReadDistribution(distsDir, f.Name())
		if err != nil {
			return nil, err
		}

		dr.distros[f.Name()] = &d
	}

	return dr, nil
}

// Available returns DistroRegistry. The registry contains distribution that
// need entitlement only if isEntitled is set to true. Otherwise, they are
// omitted from the registry.
func (adr *AllDistroRegistry) Available(isEntitled bool) *DistroRegistry {
	dr := &DistroRegistry{
		distros: make(map[string]*DistributionFile),
	}

	for name, d := range adr.distros {
		if !isEntitled && d.NeedsEntitlement() {
			continue
		}

		dr.distros[name] = d
	}

	return dr
}

// DistroRegistry is a storage structure for distributions, it can be only
// constructed using AllDistroRegistry.Available()
type DistroRegistry struct {
	distros map[string]*DistributionFile
}

// List returns all distribution in the registry.
func (dr DistroRegistry) List() []*DistributionFile {
	var ds []*DistributionFile

	for _, d := range dr.distros {
		ds = append(ds, d)
	}

	return ds
}

// Get returns a distribution with a specific name.
// If it's not found, DistributionNotFound is returned.
func (dr DistroRegistry) Get(name string) (*DistributionFile, error) {
	df, found := dr.distros[name]
	if !found {
		return nil, DistributionNotFound
	}

	return df, nil
}
