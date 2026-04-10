package check_test

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"math/big"
	"testing"
	"time"

	"github.com/osbuild/blueprint/pkg/blueprint"
	check "github.com/osbuild/images/cmd/check-host-config/check"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func generateTestCert(t *testing.T, cn string, serial *big.Int) string {
	t.Helper()
	privKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	template := x509.Certificate{
		SerialNumber:          serial,
		Subject:               pkix.Name{CommonName: cn},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(365 * 24 * time.Hour),
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}
	certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, &privKey.PublicKey, privKey)
	require.NoError(t, err)
	return string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER}))
}

func TestCACertsCheck(t *testing.T) {
	const bundlePath = "/etc/pki/ca-trust/extracted/pem/tls-ca-bundle.pem"

	tests := []struct {
		name       string
		config     func(t *testing.T) *blueprint.CACustomization
		mockExists map[string]bool          // anchor path -> exists
		mockGrep   map[GrepInput]GrepResult // (pattern, filename) -> (found, err)
		wantErr    error
	}{
		{
			name:    "skip when no certs",
			config:  func(t *testing.T) *blueprint.CACustomization { return nil },
			wantErr: check.ErrCheckSkipped,
		},
		{
			name: "pass when anchor exists and CN in bundle",
			config: func(t *testing.T) *blueprint.CACustomization {
				return &blueprint.CACustomization{
					PEMCerts: []string{generateTestCert(t, "Test CA Certificate", big.NewInt(1234567890))},
				}
			},
			mockExists: map[string]bool{
				"/etc/pki/ca-trust/source/anchors/499602d2.pem": true,
			},
			mockGrep: map[GrepInput]GrepResult{
				{Pattern: "Test CA Certificate", Filename: bundlePath}: {Found: true},
			},
		},
		{
			name: "pass when multiple certs",
			config: func(t *testing.T) *blueprint.CACustomization {
				return &blueprint.CACustomization{
					PEMCerts: []string{
						generateTestCert(t, "First CA Certificate", big.NewInt(1111111111)),
						generateTestCert(t, "Second CA Certificate", big.NewInt(2222222222)),
					},
				}
			},
			mockExists: map[string]bool{
				"/etc/pki/ca-trust/source/anchors/423a35c7.pem": true,
				"/etc/pki/ca-trust/source/anchors/84746b8e.pem": true,
			},
			mockGrep: map[GrepInput]GrepResult{
				{Pattern: "First CA Certificate", Filename: bundlePath}:  {Found: true},
				{Pattern: "Second CA Certificate", Filename: bundlePath}: {Found: true},
			},
		},
		{
			name: "fail when anchor missing",
			config: func(t *testing.T) *blueprint.CACustomization {
				return &blueprint.CACustomization{
					PEMCerts: []string{generateTestCert(t, "Missing Anchor Test", big.NewInt(9999999999))},
				}
			},
			mockExists: map[string]bool{
				"/etc/pki/ca-trust/source/anchors/2540be3ff.pem": false,
			},
			mockGrep: map[GrepInput]GrepResult{
				{Pattern: "Missing Anchor Test", Filename: bundlePath}: {Found: true},
			},
			wantErr: check.ErrCheckFailed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			installMockExists(t, tt.mockExists)
			installMockGrep(t, tt.mockGrep)

			chk, found := check.FindCheckByName("cacerts")
			require.True(t, found, "cacerts check not found")
			config := buildConfig(&blueprint.Customizations{
				CACerts: tt.config(t),
			})

			err := chk.Func(chk.Meta, config)
			if tt.wantErr != nil {
				require.Error(t, err)
				assert.True(t, errors.Is(err, tt.wantErr))
			} else {
				require.NoError(t, err)
			}
		})
	}
}
