//go:generate go run github.com/deepmap/oapi-codegen/cmd/oapi-codegen --package=cloudapi --generate types -o cloudapi_types.go cloudapi_types.yml

package cloudapi

import (
	"bytes"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
)

type OsbuildClient struct {
	osbuildURL string
	cert       string
	key        string
	ca         string
	pathPrefix string
	client     *http.Client
}

func NewOsbuildClient(osbuildURL string, cert *string, key *string, ca *string) (OsbuildClient, error) {
	oc := OsbuildClient{}
	oc.osbuildURL = osbuildURL

	if cert != nil {
		oc.cert = *cert
	}
	if key != nil {
		oc.key = *key
	}
	if ca != nil {
		oc.ca = *ca
	}
	oc.pathPrefix = "/api/composer/v1"

	if strings.HasPrefix(osbuildURL, "https") {
		// Load client cert
		cert, err := tls.LoadX509KeyPair(oc.cert, oc.key)
		if err != nil {
			return oc, err
		}

		var tlsConfig *tls.Config
		caCert, err := ioutil.ReadFile(oc.ca)
		if err != nil {
			return oc, err
		}
		caCertPool := x509.NewCertPool()
		caCertPool.AppendCertsFromPEM(caCert)
		tlsConfig = &tls.Config{
			MinVersion:   tls.VersionTLS12,
			Certificates: []tls.Certificate{cert},
			RootCAs:      caCertPool,
		}

		tlsConfig.BuildNameToCertificate()
		transport := &http.Transport{TLSClientConfig: tlsConfig}
		oc.client = &http.Client{Transport: transport}
	} else {
		oc.client = &http.Client{}
	}

	return oc, nil
}

func (oc *OsbuildClient) ComposeStatus(id string) (*http.Response, error) {
	req, err := http.NewRequest("GET", fmt.Sprintf("%s%s/compose/%s", oc.osbuildURL, oc.pathPrefix, id), nil)
	if err != nil {
		return nil, err
	}

	return oc.client.Do(req)
}

func (oc *OsbuildClient) Compose(compose ComposeRequest) (*http.Response, error) {
	buf, err := json.Marshal(compose)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", fmt.Sprintf("%s%s/compose", oc.osbuildURL, oc.pathPrefix), bytes.NewReader(buf))
	if err != nil {
		return nil, err
	}
	req.Header.Add("Content-Type", "application/json")

	return oc.client.Do(req)
}
