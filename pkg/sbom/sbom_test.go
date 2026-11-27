package sbom_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/osbuild/image-builder/pkg/rpmmd"
	"github.com/osbuild/image-builder/pkg/sbom"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var testInputs = sbom.Inputs{
	Creator: sbom.Creator{
		Tool: sbom.ToolInfo{
			Name:    "image-builder",
			Version: "1.2.3",
		},
	},
	Image: sbom.Image{
		Name:     "fedora-44-qcow2-x86_64",
		FileName: "fedora-44-qcow2-x86_64.qcow2",
		SHA256:   "60303ae22b998861bce3b28f33eec1be758a213c86c93c076dbe9f558c11c752",
	},
}

type spdxChecksum struct {
	Algorithm     string `json:"algorithm"`
	ChecksumValue string `json:"checksumValue"`
}

type spdxExternalRef struct {
	ReferenceType    string `json:"referenceType"`
	ReferenceLocator string `json:"referenceLocator"`
}

type spdxPackage struct {
	DownloadLocation string            `json:"downloadLocation"`
	FilesAnalyzed    *bool             `json:"filesAnalyzed"`
	SPDXID           string            `json:"SPDXID"`
	Name             string            `json:"name"`
	VersionInfo      string            `json:"versionInfo"`
	PackageFileName  string            `json:"packageFileName"`
	Supplier         string            `json:"supplier"`
	Homepage         string            `json:"homepage"`
	Checksums        []spdxChecksum    `json:"checksums"`
	ExternalRefs     []spdxExternalRef `json:"externalRefs"`
}

type spdxRelationship struct {
	Element string `json:"spdxElementId"`
	Related string `json:"relatedSpdxElement"`
	Type    string `json:"relationshipType"`
}

type spdxDoc struct {
	SPDXID            string `json:"SPDXID"`
	DataLicense       string `json:"dataLicense"`
	SPDXVersion       string `json:"spdxVersion"`
	Name              string `json:"name"`
	DocumentNamespace string `json:"documentNamespace"`
	CreationInfo      struct {
		Creators []string `json:"creators"`
		Created  string   `json:"created"`
	} `json:"creationInfo"`
	Packages      []spdxPackage      `json:"packages"`
	Relationships []spdxRelationship `json:"relationships"`
}

func writeSPDX(t *testing.T, s *sbom.Document) spdxDoc {
	t.Helper()

	var buf bytes.Buffer
	require.NoError(t, s.Write(&buf, sbom.FormatSPDX23JSON))

	var out spdxDoc
	require.NoError(t, json.Unmarshal(buf.Bytes(), &out))
	return out
}

func packageNamed(t *testing.T, doc spdxDoc, name string) spdxPackage {
	t.Helper()

	for _, p := range doc.Packages {
		if p.Name == name {
			return p
		}
	}
	t.Fatalf("package %q not in document", name)
	return spdxPackage{}
}

func assertDocumentNamespace(t *testing.T, name, namespace string) {
	t.Helper()
	prefix := "https://osbuild.org/spdxdocs/" + name + "-"
	require.True(t, strings.HasPrefix(namespace, prefix), "namespace %q", namespace)
	_, err := uuid.Parse(strings.TrimPrefix(namespace, prefix))
	require.NoError(t, err)
}

func bashRPM() sbom.Package {
	return sbom.Package{
		Name: "bash", Version: "5.2.26", Release: "3.fc40", Arch: "x86_64",
		Vendor: "Fedora Project", URL: "https://www.gnu.org/software/bash/",
		Checksum: rpmmd.Checksum{Type: "sha256", Value: strings.Repeat("a", 64)},
	}
}

func TestSPDXImageRoot(t *testing.T) {
	for _, withPackages := range []bool{false, true} {
		t.Run(fmt.Sprintf("packages=%t", withPackages), func(t *testing.T) {
			in := testInputs
			if withPackages {
				other := bashRPM()
				other.Name = "coreutils"
				in.Packages = []sbom.Package{bashRPM(), other, bashRPM()}
			}
			s, err := sbom.Generate(in)
			require.NoError(t, err)
			out := writeSPDX(t, s)
			assert.Equal(t, "SPDX-2.3", out.SPDXVersion)
			assert.Equal(t, "SPDXRef-DOCUMENT", out.SPDXID)
			assert.Equal(t, "CC0-1.0", out.DataLicense)
			_, err = time.Parse(time.RFC3339, out.CreationInfo.Created)
			require.NoError(t, err)
			seen := make(map[string]bool)
			for _, pkg := range out.Packages {
				assert.Regexp(t, `^SPDXRef-[a-zA-Z0-9.-]+$`, pkg.SPDXID)
				assert.False(t, seen[pkg.SPDXID], "duplicate SPDXID %s", pkg.SPDXID)
				seen[pkg.SPDXID] = true
				assert.NotEmpty(t, pkg.DownloadLocation)
				require.NotNil(t, pkg.FilesAnalyzed)
				assert.False(t, *pkg.FilesAnalyzed)
			}
			assert.Equal(t, in.Image.Name, out.Name)
			image := packageNamed(t, out, in.Image.Name)
			assert.Equal(t, in.Image.FileName, image.PackageFileName)
			assert.Equal(t, []spdxChecksum{{"SHA256", in.Image.SHA256}}, image.Checksums)
			want := []spdxRelationship{{"SPDXRef-DOCUMENT", image.SPDXID, "DESCRIBES"}}
			if withPackages {
				for _, name := range []string{"bash", "coreutils"} {
					rpm := packageNamed(t, out, name)
					want = append(want, spdxRelationship{image.SPDXID, rpm.SPDXID, "CONTAINS"})
				}
			}
			// Counts catch duplicate nodes as well as duplicate relationships.
			assert.Len(t, out.Packages, len(want))
			assert.ElementsMatch(t, want, out.Relationships)
		})
	}
}

func TestSPDXDocumentNamespaceAndCreators(t *testing.T) {
	for _, tt := range []struct{ name, escaped string }{
		{testInputs.Image.Name, testInputs.Image.Name},
		{"my image", "my%20image"}, {"100% image", "100%25%20image"},
		{"image#1", "image%231"}, {"café-image", "caf%C3%A9-image"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			in := testInputs
			in.Image.Name = tt.name
			for _, org := range []string{"", "Red Hat"} {
				in.Creator.Organization = org
				s, err := sbom.Generate(in)
				require.NoError(t, err)
				out := writeSPDX(t, s)
				assert.Equal(t, tt.name, out.Name)
				assertDocumentNamespace(t, tt.escaped, out.DocumentNamespace)
				creators := []string{"Tool: image-builder-1.2.3"}
				supplier := ""
				if org != "" {
					supplier = "Organization: " + org
					creators = append(creators, supplier)
				}
				assert.Subset(t, out.CreationInfo.Creators, creators)
				assert.Len(t, out.CreationInfo.Creators, len(creators)+1)
				image := packageNamed(t, out, tt.name)
				assert.Equal(t, supplier, image.Supplier)
				again, err := sbom.Generate(in)
				require.NoError(t, err)
				second := writeSPDX(t, again)
				assert.NotEqual(t, out.DocumentNamespace, second.DocumentNamespace)
				assert.Equal(t, image.SPDXID, packageNamed(t, second, tt.name).SPDXID)
			}
		})
	}
}

func TestSPDXRPMPackageFields(t *testing.T) {
	redhat := sbom.Package{
		Name: "openssl-libs", Epoch: 1, Version: "3.0.7", Release: "18.el9_2", Arch: "aarch64",
		Vendor: "Red Hat, Inc.", URL: "https://www.openssl.org/",
		Checksum: rpmmd.Checksum{Type: "sha256", Value: strings.Repeat("b", 64)},
	}
	for _, tt := range []struct {
		pkg                      sbom.Package
		filename, purl, supplier string
	}{
		{bashRPM(), "bash-5.2.26-3.fc40.x86_64.rpm", "pkg:rpm/fedora/bash@5.2.26-3.fc40?arch=x86_64", ""},
		{redhat, "openssl-libs-3.0.7-18.el9_2.aarch64.rpm", "pkg:rpm/redhat/openssl-libs@3.0.7-18.el9_2?arch=aarch64&epoch=1", "Organization: Red Hat"},
	} {
		t.Run(tt.pkg.Name, func(t *testing.T) {
			in := testInputs
			in.Packages = []sbom.Package{tt.pkg}
			s, err := sbom.Generate(in)
			require.NoError(t, err)
			rpm := packageNamed(t, writeSPDX(t, s), tt.pkg.Name)
			assert.Equal(t, tt.filename, rpm.PackageFileName)
			assert.Equal(t, tt.supplier, rpm.Supplier)
			assert.Equal(t, tt.pkg.URL, rpm.Homepage)
			assert.Equal(t, tt.pkg.Version+"-"+tt.pkg.Release, rpm.VersionInfo)
			assert.Equal(t, []spdxChecksum{{"SHA256", tt.pkg.Checksum.Value}}, rpm.Checksums)
			assert.Contains(t, rpm.ExternalRefs, spdxExternalRef{"purl", tt.purl})
		})
	}
}

func TestSPDXRPMChecksumAlgorithm(t *testing.T) {
	for _, tt := range []struct {
		typ    string
		length int
	}{
		{"md5", 32}, {"sha1", 40}, {"sha256", 64}, {"sha384", 96}, {"sha512", 128},
	} {
		t.Run(tt.typ, func(t *testing.T) {
			in := testInputs
			p := bashRPM()
			p.Checksum = rpmmd.Checksum{Type: tt.typ, Value: strings.Repeat("b", tt.length)}
			in.Packages = []sbom.Package{p}
			s, err := sbom.Generate(in)
			require.NoError(t, err)
			rpm := packageNamed(t, writeSPDX(t, s), p.Name)
			assert.Equal(t, []spdxChecksum{{strings.ToUpper(tt.typ), p.Checksum.Value}}, rpm.Checksums)
		})
	}
}

func TestGenerateRejectsInvalidPackages(t *testing.T) {
	for _, tt := range []struct {
		name   string
		change func(*sbom.Package)
		want   string
	}{
		{"name", func(p *sbom.Package) { p.Name = "" }, "missing name"},
		{"blank name", func(p *sbom.Package) { p.Name = " \t" }, "missing name"},
		{"version", func(p *sbom.Package) { p.Version = "" }, "missing version"},
		{"release", func(p *sbom.Package) { p.Release = "" }, "missing release"},
		{"arch", func(p *sbom.Package) { p.Arch = "" }, "missing arch"},
		{"checksum", func(p *sbom.Package) { p.Checksum.Value = "" }, "missing checksum"},
		{"type", func(p *sbom.Package) { p.Checksum.Type = "" }, "missing checksum type"},
		{"unsupported", func(p *sbom.Package) { p.Checksum.Type = "sha224" }, `unsupported checksum type "sha224"`},
		{"short", func(p *sbom.Package) { p.Checksum.Value = "aa" }, "invalid sha256 checksum"},
		{"long", func(p *sbom.Package) { p.Checksum.Value += "aa" }, "invalid sha256 checksum"},
		{"not hex", func(p *sbom.Package) { p.Checksum.Value = strings.Repeat("z", 64) }, "invalid sha256 checksum"},
		{"uppercase", func(p *sbom.Package) { p.Checksum.Value = strings.Repeat("A", 64) }, "invalid sha256 checksum"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			p := bashRPM()
			tt.change(&p)
			in := testInputs
			in.Packages = []sbom.Package{p}
			s, err := sbom.Generate(in)
			assert.ErrorContains(t, err, tt.want)
			assert.Nil(t, s)
		})
	}
	in := testInputs
	in.Packages = []sbom.Package{bashRPM(), bashRPM()}
	_, err := sbom.Generate(in)
	require.NoError(t, err)
	conflict := bashRPM()
	conflict.Checksum.Value = strings.Repeat("b", 64)
	in.Packages = []sbom.Package{bashRPM(), conflict}
	const want = `duplicate package "pkg:rpm/fedora/bash@5.2.26-3.fc40?arch=x86_64"`
	s, err := sbom.Generate(in)
	assert.ErrorContains(t, err, want)
	assert.Nil(t, s)
}

func TestGenerateInvalidInputs(t *testing.T) {
	for _, tt := range []struct {
		name   string
		change func(*sbom.Inputs)
		want   string
	}{
		{"image name", func(in *sbom.Inputs) { in.Image.Name = "" }, "image name is missing"},
		{"tool name", func(in *sbom.Inputs) { in.Creator.Tool.Name = "" }, "tool name is missing"},
		{"tool version", func(in *sbom.Inputs) { in.Creator.Tool.Version = "" }, "tool version is missing"},
		{"missing digest", func(in *sbom.Inputs) { in.Image.SHA256 = "" }, "image sha256 is missing"},
		{"short digest", func(in *sbom.Inputs) { in.Image.SHA256 = "aa" }, "invalid image sha256"},
		{"uppercase digest", func(in *sbom.Inputs) { in.Image.SHA256 = strings.Repeat("A", 64) }, "invalid image sha256"},
		{"nonhex digest", func(in *sbom.Inputs) { in.Image.SHA256 = strings.Repeat("z", 64) }, "invalid image sha256"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			in := testInputs
			tt.change(&in)
			s, err := sbom.Generate(in)
			assert.ErrorContains(t, err, tt.want)
			assert.Nil(t, s)
		})
	}
}

type errorWriter struct{ err error }

func (w errorWriter) Write([]byte) (int, error) { return 0, w.err }

func TestDocumentWriteErrors(t *testing.T) {
	s, err := sbom.Generate(testInputs)
	require.NoError(t, err)
	var buf bytes.Buffer
	assert.ErrorContains(t, s.Write(&buf, "foo"), `unsupported SBOM format "foo"`)
	assert.Zero(t, buf.Len())
	writeErr := errors.New("writer failed")
	assert.ErrorIs(t, s.Write(errorWriter{writeErr}, sbom.FormatSPDX23JSON), writeErr)
}

func TestSPDXPackagesSortedByPURL(t *testing.T) {
	for _, tt := range []struct {
		name     string
		packages []sbom.Package
		purls    []string
	}{
		{
			name: "unsorted input",
			packages: []sbom.Package{
				{
					Name:    "openssl-libs",
					Epoch:   1,
					Version: "3.0.7",
					Release: "18.el9_2",
					Arch:    "aarch64",
					Vendor:  "Red Hat, Inc.",
					URL:     "https://www.openssl.org/",
					Checksum: rpmmd.Checksum{
						Type:  "sha256",
						Value: strings.Repeat("b", 64),
					},
				},
				{
					Name:    "bash",
					Version: "5.2.26",
					Release: "3.fc40",
					Arch:    "x86_64",
					Vendor:  "Fedora Project",
					URL:     "https://www.gnu.org/software/bash/",
					Checksum: rpmmd.Checksum{
						Type:  "sha256",
						Value: strings.Repeat("a", 64),
					},
				},
				{
					Name:    "zsh",
					Version: "1.0",
					Release: "1.al8",
					Arch:    "x86_64",
					Vendor:  "AlmaLinux",
					URL:     "https://example.test/zsh",
					Checksum: rpmmd.Checksum{
						Type:  "sha256",
						Value: strings.Repeat("c", 64),
					},
				},
			},
			purls: []string{
				"pkg:rpm/almalinux/zsh@1.0-1.al8?arch=x86_64",
				"pkg:rpm/fedora/bash@5.2.26-3.fc40?arch=x86_64",
				"pkg:rpm/redhat/openssl-libs@3.0.7-18.el9_2?arch=aarch64&epoch=1",
			},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			in := testInputs
			in.Packages = tt.packages
			s, err := sbom.Generate(in)
			require.NoError(t, err)
			var got []string
			for _, pkg := range writeSPDX(t, s).Packages {
				for _, ref := range pkg.ExternalRefs {
					if ref.ReferenceType == "purl" {
						got = append(got, ref.ReferenceLocator)
					}
				}
			}
			assert.Equal(t, tt.purls, got)
		})
	}
}
