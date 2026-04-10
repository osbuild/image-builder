package main

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"math/big"
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/osbuild/blueprint/pkg/blueprint"
	"github.com/osbuild/images/cmd/check-host-config/check"
	"github.com/osbuild/images/internal/buildconfig"
)

func TestShouldRunOn(t *testing.T) {
	tests := []struct {
		name      string
		osRelease *check.OSRelease
		runOn     []string
		want      bool
	}{
		{
			name:      "nil osRelease returns true",
			osRelease: nil,
			runOn:     []string{"fedora-39"},
			want:      true,
		},
		{
			name:      "empty runOn returns true",
			osRelease: &check.OSRelease{ID: "fedora", VersionID: "39"},
			runOn:     nil,
			want:      true,
		},
		{
			name:      "empty runOn slice returns true",
			osRelease: &check.OSRelease{ID: "fedora", VersionID: "39"},
			runOn:     []string{},
			want:      true,
		},
		{
			name:      "inclusion matches current ID",
			osRelease: &check.OSRelease{ID: "Fedora", VersionID: "39"},
			runOn:     []string{"fedora-39"},
			want:      true,
		},
		{
			name:      "inclusion does not match",
			osRelease: &check.OSRelease{ID: "fedora", VersionID: "39"},
			runOn:     []string{"rhel-9"},
			want:      false,
		},
		{
			name:      "exclusion matches current ID",
			osRelease: &check.OSRelease{ID: "fedora", VersionID: "39"},
			runOn:     []string{"!fedora-39"},
			want:      false,
		},
		{
			name:      "exclusion match is case-insensitive",
			osRelease: &check.OSRelease{ID: "RHEL", VersionID: "9.0"},
			runOn:     []string{"!rhel-9.0"},
			want:      false,
		},
		{
			name:      "exclusion does not match, no inclusions",
			osRelease: &check.OSRelease{ID: "fedora", VersionID: "39"},
			runOn:     []string{"!rhel-9"},
			want:      true,
		},
		{
			name:      "exclusions does not match, no inclusions",
			osRelease: &check.OSRelease{ID: "fedora", VersionID: "39"},
			runOn:     []string{"!rhel-9", "!centos-9"},
			want:      true,
		},
		{
			name:      "exclusion does not match, inclusion matches",
			osRelease: &check.OSRelease{ID: "fedora", VersionID: "39"},
			runOn:     []string{"fedora-39", "!rhel-9"},
			want:      true,
		},
		{
			name:      "multiple inclusions, one matches",
			osRelease: &check.OSRelease{ID: "rhel", VersionID: "9.0"},
			runOn:     []string{"fedora-39", "rhel-9.0", "centos-9"},
			want:      true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := shouldRunOn(tt.osRelease, tt.runOn)
			if got != tt.want {
				t.Errorf("shouldRunOn() = %v, want %v", got, tt.want)
			}
		})
	}
}

// generateSmokeCACert returns a CA cert PEM (serial 1, CN "Smoke Test CA").
func generateSmokeCACert(t *testing.T) string {
	t.Helper()
	privKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	template := x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "Smoke Test CA"},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(999 * 24 * time.Hour),
		KeyUsage:              x509.KeyUsageCertSign,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		IsCA:                  true,
	}
	certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, &privKey.PublicKey, privKey)
	if err != nil {
		t.Fatal(err)
	}
	return string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER}))
}

// This is a happy-path smoke test that is supposed to be executed in a
// temporary Fedora container. It is ran on our CI/CD. To run it locally (in
// podman), execute `make host-check-test`.
//
// Tests which require running services or commands which are not relevant for
// Fedora environment (dnf modularity, OpenSCAP) are not supported in the smoke test.
//
//nolint:gosec // G303: Temporary files need to be consistently named
func TestSmokeAll(t *testing.T) {
	if os.Getenv("OSBUILD_TEST_CONTAINER") != "true" {
		t.Skip("Not running in container, skipping host check test")
	}

	// Prepare the container environment (cleanup not needed)

	// cacerts
	smokeCACertPEM := generateSmokeCACert(t)
	anchorsDir := "/etc/pki/ca-trust/source/anchors"
	if err := os.MkdirAll(anchorsDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(anchorsDir+"/1.pem", []byte(smokeCACertPEM), 0644); err != nil {
		t.Fatal(err)
	}
	if err := exec.Command("update-ca-trust", "extract").Run(); err != nil {
		t.Fatal(err)
	}

	// directories
	if err := os.Mkdir("/tmp/dir", 0700); err != nil {
		t.Fatal(err)
	}

	// files
	if err := os.WriteFile("/tmp/dir/file", []byte("data"), 0600); err != nil {
		t.Fatal(err)
	}

	// hostname: use current hostname and ensure /etc/hostname exists so the check's fallback works
	smokeHostname, err := os.Hostname()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile("/etc/hostname", []byte(smokeHostname+"\n"), 0644); err != nil {
		t.Fatal(err)
	}

	// users: create a dedicated user with UID 1000 for the users check
	if out, err := exec.Command("useradd", "-u", "1000", "-m", "smokeuser").CombinedOutput(); err != nil {
		t.Fatalf("useradd smokeuser: %v, output: %s", err, out)
	}

	tests := []struct {
		chk  string
		name string
		c    blueprint.Customizations
	}{
		{
			chk:  "kernel",
			name: "params",
			c: blueprint.Customizations{
				Kernel: &blueprint.KernelCustomization{
					Name:   "kernel",
					Append: "root=",
				},
			},
		},
		{
			chk:  "kernel",
			name: "package",
			c: blueprint.Customizations{
				Kernel: &blueprint.KernelCustomization{
					Name: "kernel-debug",
				},
			},
		},
		{
			chk: "directories",
			c: blueprint.Customizations{
				Directories: []blueprint.DirectoryCustomization{
					{Path: "/tmp/dir"},
					{Path: "/tmp/dir", Mode: "0700"},
					{Path: "/tmp/dir", Mode: "0700", User: "root", Group: "root"},
					{Path: "/tmp/dir", Mode: "0700", User: 0, Group: 0},
				},
			},
		},
		{
			chk: "files",
			c: blueprint.Customizations{
				Files: []blueprint.FileCustomization{
					{Path: "/tmp/dir/file"},
					{Path: "/tmp/dir/file", Data: "data"},
					{Path: "/tmp/dir/file", Mode: "0600"},
					{Path: "/tmp/dir/file", Mode: "0600", User: "root", Group: "root"},
					{Path: "/tmp/dir/file", Mode: "0600", User: 0, Group: 0},
				},
			},
		},
		{
			chk: "users",
			c: blueprint.Customizations{
				User: []blueprint.UserCustomization{
					{Name: "root"},
					{Name: "smokeuser"},
				},
			},
		},
		{
			chk: "cacerts",
			c: blueprint.Customizations{
				CACerts: &blueprint.CACustomization{
					PEMCerts: []string{smokeCACertPEM},
				},
			},
		},
		{
			chk: "hostname",
			c: blueprint.Customizations{
				Hostname: &smokeHostname,
			},
		},
	}

	for _, tt := range tests {
		name := tt.chk
		if tt.name != "" {
			name += "/" + tt.name
		}

		t.Run(name, func(t *testing.T) {
			chk := check.MustFindCheckByName(tt.chk)
			config := &buildconfig.BuildConfig{
				Blueprint: &blueprint.Blueprint{
					Customizations: &tt.c,
				},
			}
			err := chk.Func(chk.Meta, config)
			if errors.Is(err, check.ErrCheckSkipped) {
				t.Logf("Check %s skipped", chk.Meta.Name)
				return
			} else if err != nil {
				t.Fatalf("Check %s failed: %v", chk.Meta.Name, err)
			}
		})
	}
}
