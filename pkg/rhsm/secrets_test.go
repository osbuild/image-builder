package rhsm

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var VALID_REPO = `[jws]
name = Red Hat JBoss Web Server
baseurl = https://cdn.redhat.com/content/dist/middleware/jws/1.0/$basearch/os
enabled = 0
gpgcheck = 1
gpgkey = file://
sslverify = 1
sslcacert = /etc/rhsm/ca/redhat-uep.pem
sslclientkey = /etc/pki/entitlement/123-key.pem
sslclientcert = /etc/pki/entitlement/456.pem
metadata_expire = 86400
enabled_metadata = 0

[rhel-atomic]
name = Red Hat Container Development Kit
baseurl = https://cdn.redhat.com/content/dist/rhel/atomic/7/7Server/$basearch/os
enabled = 0
gpgcheck = 1
gpgkey = http://
sslverify = 1
sslcacert = /etc/rhsm/ca/redhat-uep.pem
sslclientkey = /etc/pki/entitlement/789-key.pem
sslclientcert = /etc/pki/entitlement/101112.pem
metadata_expire = 86400
enabled_metadata = 0
`

// SATELLITE_REPO is a redhat.repo from a Satellite-registered host: the baseurl is
// templated on $releasever with a literal arch, and sslcacert is the Katello CA
// rather than the default redhat-uep.pem.
var SATELLITE_REPO = `[rhel-10-for-x86_64-baseos-rpms]
name = Red Hat Enterprise Linux 10 for x86_64 - BaseOS (RPMs)
baseurl = https://satellite.example.com/pulp/content/dist/rhel10/$releasever/x86_64/baseos/os
enabled = 1
gpgcheck = 1
sslverify = 1
sslcacert = /etc/rhsm/ca/katello-server-ca.pem
sslclientkey = /etc/pki/entitlement/517534911145439618-key.pem
sslclientcert = /etc/pki/entitlement/517534911145439618.pem
metadata_expire = 1
enabled_metadata = 1
`

func TestParseRepoFile(t *testing.T) {
	input := []byte(VALID_REPO)
	repoFileContent, err := parseRepoFile(input)
	require.NoError(t, err, "Failed to parse the .repo file")
	subscriptions := Subscriptions{
		available: repoFileContent,
	}
	secrets, err := subscriptions.GetSecretsForBaseurl([]string{"https://cdn.redhat.com/content/dist/middleware/jws/1.0/x86_64/os"})
	require.NoError(t, err, "Failed to get secrets for a baseurl")
	assert.Equal(t, secrets.SSLCACert, "/etc/rhsm/ca/redhat-uep.pem", "Unexpected path to the CA certificate")
	assert.Equal(t, secrets.SSLClientCert, "/etc/pki/entitlement/456.pem", "Unexpected path to the client cert")
	assert.Equal(t, secrets.SSLClientKey, "/etc/pki/entitlement/123-key.pem", "Unexpected path to the client key")
}

// no fallback secrets set, so a miss surfaces as an error rather than the
// fallback CA, isolating these cases to the matching behaviour.
func TestGetSecretsForBaseurlMatching(t *testing.T) {
	repoFileContent, err := parseRepoFile([]byte(SATELLITE_REPO))
	require.NoError(t, err, "Failed to parse the .repo file")
	subscriptions := Subscriptions{available: repoFileContent}

	base := "https://satellite.example.com/pulp/content/dist/rhel10"
	for _, tc := range []struct {
		name      string
		url       string
		wantMatch bool
	}{
		{"rolling", base + "/10/x86_64/baseos/os", true},
		{"point release", base + "/10.2/x86_64/baseos/os", true},
		{"wrong path structure", base + "/10/x86_64/appstream/os", false},
		{"different host", "https://cdn.redhat.com/content/dist/rhel10/10/x86_64/baseos/os", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			secrets, err := subscriptions.GetSecretsForBaseurl([]string{tc.url})
			if !tc.wantMatch {
				assert.Error(t, err, "expected no match for %q", tc.url)
				return
			}
			require.NoError(t, err, "expected a match for %q", tc.url)
			assert.Equal(t, "/etc/rhsm/ca/katello-server-ca.pem", secrets.SSLCACert)
			assert.Equal(t, "/etc/pki/entitlement/517534911145439618.pem", secrets.SSLClientCert)
			assert.Equal(t, "/etc/pki/entitlement/517534911145439618-key.pem", secrets.SSLClientKey)
		})
	}
}

// A point-release request must resolve to the matching subscription's CA, not the
// global fallback CA, even when a fallback is set.
func TestGetSecretsForBaseurlPointReleasePrefersSubscriptionCA(t *testing.T) {
	repoFileContent, err := parseRepoFile([]byte(SATELLITE_REPO))
	require.NoError(t, err, "Failed to parse the .repo file")
	subscriptions := Subscriptions{
		available: repoFileContent,
		secrets: &RHSMSecrets{
			SSLCACert:     "/etc/rhsm/ca/redhat-uep.pem",
			SSLClientKey:  "/etc/pki/entitlement/fallback-key.pem",
			SSLClientCert: "/etc/pki/entitlement/fallback.pem",
		},
	}
	secrets, err := subscriptions.GetSecretsForBaseurl(
		[]string{"https://satellite.example.com/pulp/content/dist/rhel10/10.2/x86_64/baseos/os"})
	require.NoError(t, err, "Failed to get secrets for a point-release baseurl")
	assert.Equal(t, "/etc/rhsm/ca/katello-server-ca.pem", secrets.SSLCACert,
		"must prefer the subscription's Katello CA over the redhat-uep.pem fallback")
}

func TestGetSecretsForBaseurlFallback(t *testing.T) {
	repoFileContent, err := parseRepoFile([]byte(SATELLITE_REPO))
	require.NoError(t, err, "Failed to parse the .repo file")
	subscriptions := Subscriptions{
		available: repoFileContent,
		secrets: &RHSMSecrets{
			SSLCACert:     "/etc/rhsm/ca/redhat-uep.pem",
			SSLClientKey:  "/etc/pki/entitlement/fallback-key.pem",
			SSLClientCert: "/etc/pki/entitlement/fallback.pem",
		},
	}
	secrets, err := subscriptions.GetSecretsForBaseurl([]string{"https://unrelated.example.com/some/path"})
	require.NoError(t, err, "expected fallback secrets to be returned")
	assert.Equal(t, "/etc/pki/entitlement/fallback-key.pem", secrets.SSLClientKey)
}

// mirrors the cases in osbuild's test_util_rhsm.py::TestUrlMatching.
func TestBaseurlToRegex(t *testing.T) {
	for _, tc := range []struct {
		name      string
		baseurl   string
		url       string
		wantMatch bool
	}{
		{"basearch", "https://cdn.redhat.com/1.0/$basearch/os", "https://cdn.redhat.com/1.0/x86_64/os/Packages/test.rpm", true},
		{"releasever and basearch", "https://cdn.redhat.com/$releasever/repo/$basearch/os", "https://cdn.redhat.com/9/repo/x86_64/os/test.rpm", true},
		{"point release releasever", "https://cdn.redhat.com/$releasever/repo/$basearch/os", "https://cdn.redhat.com/9.8/repo/x86_64/os/test.rpm", true},
		{"wrong path structure", "https://cdn.redhat.com/$releasever/repo/$basearch/os", "https://cdn.redhat.com/9/different/x86_64/os/test.rpm", false},
		{"different host", "https://cdn.redhat.com/1.0/$basearch/os", "https://other.host.com/1.0/x86_64/os/test.rpm", false},
		{"uuid variable", "https://cdn.redhat.com/$uuid/content", "https://cdn.redhat.com/abc-123-def/content/test.rpm", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			re, err := baseurlToRegex(tc.baseurl)
			require.NoError(t, err)
			assert.Equal(t, tc.wantMatch, re.MatchString(tc.url))
		})
	}
}
