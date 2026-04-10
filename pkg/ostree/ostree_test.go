package ostree

import (
	"crypto/tls"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/osbuild/images/pkg/ostree/test_mtls_server"
	"github.com/osbuild/images/pkg/rhsm"
)

func TestOstreeResolveRef(t *testing.T) {
	goodRef := "5330bb1b8820944567f519de66ad6354c729b6b490dea1c5a7ba320c9f147c58"
	badRef := "<html>not a ref</html>"

	handler := http.NewServeMux()
	handler.HandleFunc("/refs/heads/rhel/8/x86_64/edge", func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	})
	handler.HandleFunc("/refs/heads/test_forbidden", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "", http.StatusForbidden)
	})
	handler.HandleFunc("/refs/heads/get_bad_ref", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, badRef)
	})

	handler.HandleFunc("/refs/heads/test_redir", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/refs/heads/valid/ostree/ref", http.StatusFound)
	})
	handler.HandleFunc("/refs/heads/valid/ostree/ref", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, goodRef)
	})

	handler.HandleFunc("/mirrorlist", func(w http.ResponseWriter, r *http.Request) {
		scheme := "http"
		if r.TLS != nil {
			scheme = "https"
		}
		fmt.Fprintf(w, "%s://%s", scheme, r.Host)
	})

	srv := httptest.NewServer(handler)
	defer srv.Close()

	mTLSSrv, err := test_mtls_server.NewMTLSServer(handler)
	srv2 := mTLSSrv.Server
	require.NoError(t, err)
	defer srv2.Close()
	subs := &rhsm.Subscriptions{
		Consumer: &rhsm.ConsumerSecrets{
			ConsumerKey:  mTLSSrv.ClientKeyPath,
			ConsumerCert: mTLSSrv.ClientCrtPath,
		},
	}

	type srvConfig struct {
		Srv  *httptest.Server
		RHSM bool
		Subs *rhsm.Subscriptions
	}
	srvConfs := []srvConfig{
		{
			Srv:  srv,
			RHSM: false,
			Subs: nil,
		},
		{
			Srv:  srv2,
			RHSM: true,
			Subs: subs,
		},
	}

	type input struct {
		location string
		ref      string
	}

	for _, srvConf := range srvConfs {
		validCases := map[input]string{
			{srvConf.Srv.URL, "test_redir"}:                                       goodRef,
			{srvConf.Srv.URL, "valid/ostree/ref"}:                                 goodRef,
			{"mirrorlist=" + srvConf.Srv.URL + "/mirrorlist", "valid/ostree/ref"}: goodRef,
		}
		for in, expOut := range validCases {
			url, out, err := resolveRef(SourceSpec{
				in.location,
				in.ref,
				srvConf.RHSM,
				&MTLS{mTLSSrv.CAPath, mTLSSrv.ClientCrtPath, mTLSSrv.ClientKeyPath},
				"",
			})
			require.NoError(t, err)
			assert.Equal(t, expOut, out)

			expectedURL := in.location
			if strings.HasPrefix(in.location, "mirrorlist=") {
				expectedURL = srvConf.Srv.URL
			}
			assert.Equal(t, expectedURL, url)
		}

		errCases := map[input]string{
			{"not-a-url", "a-bad-ref"}:              "error sending request to ostree repository \"not-a-url/refs/heads/a-bad-ref\": Get \"not-a-url/refs/heads/a-bad-ref\": unsupported protocol scheme \"\"",
			{"http://0.0.0.0:10/repo", "whatever"}:  "error sending request to ostree repository \"http://0.0.0.0:10/repo/refs/heads/whatever\": Get \"http://0.0.0.0:10/repo/refs/heads/whatever\": dial tcp 0.0.0.0:10: connect: connection refused",
			{srvConf.Srv.URL, "rhel/8/x86_64/edge"}: fmt.Sprintf("ostree repository \"%s/refs/heads/rhel/8/x86_64/edge\" returned status: 404 Not Found", srvConf.Srv.URL),
			{srvConf.Srv.URL, "test_forbidden"}:     fmt.Sprintf("ostree repository \"%s/refs/heads/test_forbidden\" returned status: 403 Forbidden", srvConf.Srv.URL),
			{srvConf.Srv.URL, "get_bad_ref"}:        fmt.Sprintf("ostree repository \"%s/refs/heads/get_bad_ref\" returned invalid reference", srvConf.Srv.URL),
		}
		for in, expMsg := range errCases {
			url, _, err := resolveRef(SourceSpec{
				in.location,
				in.ref,
				srvConf.RHSM,
				&MTLS{mTLSSrv.CAPath, mTLSSrv.ClientCrtPath, mTLSSrv.ClientKeyPath},
				"",
			})
			assert.EqualError(t, err, expMsg)
			assert.Equal(t, url, "")
		}
	}
}

func TestVerifyRef(t *testing.T) {
	cases := map[string]bool{
		"a_perfectly_valid_ref": true,
		"another/valid/ref":     true,
		"this-one-has/all.the/_valid-/characters/even/_numbers_42": true,
		"rhel/8/aarch64/edge": true,
		"1337":                true,
		"1337/but/also/more":  true,
		"_good_start/ref":     true,
		"/bad/ref":            false,
		"invalid)characters":  false,
		"this/was/doing/fine/until/the/very/end/": false,
		"-bad_start/ref":     false,
		".another/bad/start": false,
		"how/about/now?":     false,
	}

	for in, expOut := range cases {
		assert.Equal(t, expOut, verifyRef(in), in)
	}
}

func TestValidate(t *testing.T) {

	type testCase struct {
		options ImageOptions
		valid   bool
	}

	cases := map[string]testCase{
		"empty": {
			options: ImageOptions{
				ImageRef:   "",
				ParentRef:  "",
				URL:        "",
				ContentURL: "",
			},
			valid: true,
		},
		"fedora-ref-valid": {
			options: ImageOptions{
				ImageRef:   "fedora/39/x86_64/iot",
				ParentRef:  "",
				URL:        "",
				ContentURL: "",
			},
			valid: true,
		},
		"fedora-ref-invalid": {
			options: ImageOptions{
				ImageRef:   "fedora/39/x86_64/ιοτ",
				ParentRef:  "",
				URL:        "",
				ContentURL: "",
			},
			valid: false,
		},
		"fedora-parent-valid": {
			options: ImageOptions{
				ImageRef:   "fedora/39/x86_64/iot",
				ParentRef:  "fedora/39/x86_64/iot",
				URL:        "https://repo.example.com",
				ContentURL: "",
			},
			valid: true,
		},
		"fedora-parent-invalid": {
			options: ImageOptions{
				ImageRef:   "fedora/39/x86_64/iot",
				ParentRef:  "-bad",
				URL:        "https://repo.example.com",
				ContentURL: "",
			},
			valid: false,
		},
		"parent-without-url": {
			options: ImageOptions{
				ParentRef:  "parent/ref/without/a/URL",
				ContentURL: "",
			},
			valid: false,
		},
		"bad-url": {
			options: ImageOptions{
				URL:        "this-is-not-a-url",
				ContentURL: "",
			},
			valid: false,
		},
		"bad-content-url": {
			options: ImageOptions{
				ContentURL: "http;;//where-is-my-shift-key.com",
			},
			valid: false,
		},
		"checksum-ref": {
			options: ImageOptions{
				ImageRef: "c70e4ceff1726cb986eafd0230e2e1b0e5ebe590d0498a9f7c370c8ec3797deb",
			},
			valid: false,
		},
		"checksum-parent": {
			options: ImageOptions{
				ImageRef:  "the-ref",
				ParentRef: "c70e4ceff1726cb986eafd0230e2e1b0e5ebe590d0498a9f7c370c8ec3797deb",
			},
			valid: false,
		},
	}

	for name, testCase := range cases {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, testCase.valid, testCase.options.Validate() == nil)
		})
	}

}

func TestClientForRefProxy(t *testing.T) {
	ss := SourceSpec{
		Proxy: "foo:1234",
	}
	client, err := httpClientForRef("https", ss)
	assert.NoError(t, err)

	proxy, err := client.Transport.(*http.Transport).Proxy(&http.Request{})
	assert.NoError(t, err)
	assert.Equal(t, "foo", proxy.Hostname())
	assert.Equal(t, "1234", proxy.Port())
}

func TestClientForRefClientCertKey(t *testing.T) {
	ss := SourceSpec{
		MTLS: &MTLS{
			ClientCert: "test_mtls_server/client.crt",
			ClientKey:  "test_mtls_server/client.key",
		},
	}
	client, err := httpClientForRef("https", ss)
	assert.NoError(t, err)

	tlsConf := client.Transport.(*http.Transport).TLSClientConfig
	assert.NoError(t, err)
	expectedCert, err := tls.LoadX509KeyPair("test_mtls_server/client.crt", "test_mtls_server/client.key")
	assert.NoError(t, err)
	assert.Equal(t, 1, len(tlsConf.Certificates))
	assert.Equal(t, expectedCert, tlsConf.Certificates[0])
	// no RootCAs got added
	assert.Nil(t, tlsConf.RootCAs)
}

func TestClientForRefCA(t *testing.T) {
	ss := SourceSpec{
		MTLS: &MTLS{
			CA: "test_mtls_server/ca.crt",
		},
	}
	client, err := httpClientForRef("https", ss)
	assert.NoError(t, err)

	tlsConf := client.Transport.(*http.Transport).TLSClientConfig
	assert.NoError(t, err)
	// the RootCAs is a x590.CertPool which provides almost no
	// introspection (only Subjects() which is deprecated). So
	// we do just chech that *something* got added and hope for the
	// best here.
	assert.NotNil(t, tlsConf.RootCAs)
	// no certificates got added
	assert.Equal(t, 0, len(tlsConf.Certificates))
}
