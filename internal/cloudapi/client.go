package cloudapi

import (
	"crypto/tls"
	"crypto/x509"
	"io/ioutil"
	"net/http"
	"strings"
)

type OsbuildClient struct {
	osbuildURL string
	cert       string
	key        string
	ca         string
}

func NewOsbuildClient(osbuildURL string, cert *string, key *string, ca *string) *OsbuildClient {
	client := &OsbuildClient{}
	client.osbuildURL = osbuildURL

	if cert != nil {
		client.cert = *cert
	}
	if key != nil {
		client.key = *key
	}
	if ca != nil {
		client.ca = *ca
	}

	return client
}

func (c *OsbuildClient) Get() (*Client, error) {
	return NewClient(c.osbuildURL, c.ConfigureClient)
}

func (c *OsbuildClient) ConfigureClient(client *Client) error {
	// set up client certificate authentication
	if strings.HasPrefix(client.Server, "https") {
		// Load client cert
		cert, err := tls.LoadX509KeyPair(c.cert, c.key)
		if err != nil {
			return err
		}

		var tlsConfig *tls.Config
		caCert, err := ioutil.ReadFile(c.ca)
		if err != nil {
			return err
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
		client.Client = &http.Client{Transport: transport}
	}
	return nil
}
