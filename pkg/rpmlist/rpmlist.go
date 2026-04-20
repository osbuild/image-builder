package rpmlist

import (
	"bytes"
	"encoding/json"

	"github.com/osbuild/images/pkg/rpmmd"
)

// rpmlist item entry expected by koji
type kojiRpmListEntry struct {
	Name        string `json:"name"`
	Version     string `json:"version"`
	Release     string `json:"release"`
	Epoch       uint   `json:"epoch"`
	Arch        string `json:"arch"`
	BuildTime   int64  `json:"buildtime"`
	Size        uint64 `json:"size"`
	PayloadHash string `json:"payloadhash,omitempty"`
}

func PackageToKojiRpmListEntry(p rpmmd.Package) kojiRpmListEntry {
	hash := p.Checksum.Value
	if hash == "" {
		hash = p.HeaderChecksum.Value
	}
	buildTime := int64(0)
	if !p.BuildTime.IsZero() {
		buildTime = p.BuildTime.Unix()
	}
	return kojiRpmListEntry{
		Name:        p.Name,
		Version:     p.Version,
		Release:     p.Release,
		Epoch:       p.Epoch,
		Arch:        p.Arch,
		BuildTime:   buildTime,
		PayloadHash: hash,
		Size:        p.DownloadSize,
	}
}

// EncodePackages returns JSON in a format that Koji expects for the given package list
// it expects a list of dictionaries, e.g. https://riscv-koji.fedoraproject.org/koji/taskinfo?taskID=136875
// https://github.com/koji-project/koji/blob/67cc2cdbac97d3af96ac2b2a4a34b5fe84a3435e/plugins/builder/kiwi.py#L263
func EncodePackages(packages rpmmd.PackageList) (*bytes.Buffer, error) {
	var buf bytes.Buffer
	if len(packages) == 0 {
		buf.Write([]byte("[]\n"))
		return &buf, nil
	}
	out := make([]kojiRpmListEntry, 0, len(packages))
	for _, p := range packages {
		out = append(out, PackageToKojiRpmListEntry(p))
	}
	enc := json.NewEncoder(&buf)
	if err := enc.Encode(out); err != nil {
		return nil, err
	}
	return &buf, nil
}
