package osbuild

import (
	"encoding/pem"
	"testing"

	"github.com/osbuild/image-builder/v73/pkg/cert"
	"github.com/stretchr/testify/assert"
)

// taken from osbuild:test/data/certs/cert{1,2}.pem
const exampleCert = `
-----BEGIN CERTIFICATE-----
MIIDhTCCAm2gAwIBAgIUVya7VJ3O8W8SqwuEa0BZ4HSsXvAwDQYJKoZIhvcNAQEL
BQAwUTELMAkGA1UEBhMCREUxDzANBgNVBAgMBkJlcmxpbjEPMA0GA1UEBwwGQmVy
bGluMQwwCgYDVQQKDANPcmcxEjAQBgNVBAMMCWxvY2FsaG9zdDAgFw0yNDA4MjYx
MDQyNDBaGA8yMTI0MDgwMjEwNDI0MFowUTELMAkGA1UEBhMCREUxDzANBgNVBAgM
BkJlcmxpbjEPMA0GA1UEBwwGQmVybGluMQwwCgYDVQQKDANPcmcxEjAQBgNVBAMM
CWxvY2FsaG9zdDCCASIwDQYJKoZIhvcNAQEBBQADggEPADCCAQoCggEBAJnGjlvN
O3F/Z7Lr/r+6Xp2DosnNwoPHhG2e61KnFzgZfaxbklal5ORpuV/gLIg7lrbpdZe7
WvK+16RanL6fLitis/tYVFyvz1MXqBYYrEoFGvVg9fOiis7hjpdZcpNDH9SngoAN
O0Wvv4T6LQS0cC7ZAFZjvmJ+RiZEbzRkNG5pUddZXbotE6htNfLgA5L1wIBgllrM
4DVkG0yNKmzqPNzfPTbdUgWCfjaQShHy1GP8KNEwFxM31F2wvQxsEb77o1S44Out
mlsi83tti6P7KjDk7w2j2zZO1X0xI8pflv3TBkJT1Am8vnk6rVnNO4pCpop3+kma
pDUEzBQmSQA5R1ECAwEAAaNTMFEwHQYDVR0OBBYEFDxFcFgPEsgsDixfKxB0uYGN
aJmzMB8GA1UdIwQYMBaAFDxFcFgPEsgsDixfKxB0uYGNaJmzMA8GA1UdEwEB/wQF
MAMBAf8wDQYJKoZIhvcNAQELBQADggEBAFih4lUbLlhKwIAV9x3/W7Mih8xUEdZr
olquZgaHedFet+ByAHvoES3pec7AVYTOD53mjgyZubD6INnVHzKyS4AG9ydD73o4
cmm3DKxBaesvlHeTn0MOKsoM8QCxeyFJmiUPpgDBok/PFnbGR9+JcsrlGJAnsSKD
vWpiwYcBauZ9nnK5yDe5M9XNFPkNDZzbKvWU7Sw3ziMT/+bRJse5vTrYcyOnNGgy
gZNz2nimKy1U8XZVAVwOV0rdGEFrfMln8DkRW86rGK/EncaVsl0SSP/rmjQgiX8Q
3CZraQGujJP932HSwUfdCX9yh+rTjE3MEnbqMoLzJa4BXB2aDQWtywU=
-----END CERTIFICATE-----
-----BEGIN CERTIFICATE-----
MIIDhTCCAm2gAwIBAgIUBFUPUz3MqjWa0bINLdvSfNDQ3iQwDQYJKoZIhvcNAQEL
BQAwUTELMAkGA1UEBhMCREUxDzANBgNVBAgMBkJlcmxpbjEPMA0GA1UEBwwGQmVy
bGluMQwwCgYDVQQKDANPcmcxEjAQBgNVBAMMCWxvY2FsaG9zdDAgFw0yNDA4MjYx
MDQyNDVaGA8yMTI0MDgwMjEwNDI0NVowUTELMAkGA1UEBhMCREUxDzANBgNVBAgM
BkJlcmxpbjEPMA0GA1UEBwwGQmVybGluMQwwCgYDVQQKDANPcmcxEjAQBgNVBAMM
CWxvY2FsaG9zdDCCASIwDQYJKoZIhvcNAQEBBQADggEPADCCAQoCggEBAN/SEvmx
m9UHhP/rXQrx82SmIft940HoJtM9Bsbp1Pj8Nk7WA+M/WgTVz0J/08uU8pZgzj0U
pfaawxfB2lo++GBa9vCeqQzIo6YKQf0Rg50+KJ1oQ3+ZIDxIr2ou38dg5KyG1D+A
XnGIg7fKu3RczCYbgudB2Yq2LvQdLTSb+PE08lczc7e9bGobHLfqCilFS71Q2BEa
B14xfrz3LF1Cf0r6qL6SXg2+vyjsMy1sGwGY1XGqAgvT8rgm+ZfVknFLyzhly6Cf
eVNg+JWapR5iDVIG6F64Ayj63wd52aveYD3JEljpzF3I8lq4qQ8oI3z0S/t4Dx01
ErexkyAFhIlWVvsCAwEAAaNTMFEwHQYDVR0OBBYEFDccfxp07zFNvuWqvSTgFmju
JrfCMB8GA1UdIwQYMBaAFDccfxp07zFNvuWqvSTgFmjuJrfCMA8GA1UdEwEB/wQF
MAMBAf8wDQYJKoZIhvcNAQELBQADggEBAK2sJWg9Eg2Ekv7TgQeln7lGWNl2EYS5
uDc1qhgkOos3QSonvSU9n90DECY+m3wcHBWF6OSB1RFSQmROdi3tKMZUgM9gnjG/
Us8D8HxS0+FLYX0wHlx/anCBzy+tKMDKq+tQzuZZGpKFhDZf527Uqj+R8DVMcqKV
oqdILvb3asYiISCXDu2Fj9HfqPHKKER6hboQ2LvcLq+t4FOtvGiTxdiDaOhSJWcL
0Izcs3Wi0RUNTIULtLFukFvYCTFoxYSLP178xXSf3N8Z9o6ezb7aUI86d6rLmwga
GHY+cBQvEsjW85elwClvspptKd1Xx4z8geH0+Y0qaIGBn1cyze5k5fk=
-----END CERTIFICATE-----
`

func TestNewCAFileNodes(t *testing.T) {
	certs, err := cert.ParseCerts(exampleCert)
	assert.Nil(t, err)

	files, err := NewCAFileNodes(exampleCert)
	assert.Nil(t, err)
	assert.Equal(t, 2, len(certs), len(files))
	expectedPaths := []string{
		"/etc/pki/ca-trust/source/anchors/5726bb549dcef16f12ab0b846b4059e074ac5ef0.pem",
		"/etc/pki/ca-trust/source/anchors/4550f533dccaa359ad1b20d2ddbd27cd0d0de24.pem",
	}
	for i, c := range certs {
		assert.Equal(t, expectedPaths[i], files[i].Path())
		assert.Equal(t, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: c.Raw}), files[i].Data())
	}
}
