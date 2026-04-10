package check

import (
	"crypto/x509"
	"encoding/pem"
	"log"
	"strings"

	"github.com/osbuild/images/internal/buildconfig"
)

func init() {
	RegisterCheck(Metadata{
		Name:                   "cacerts",
		RequiresBlueprint:      true,
		RequiresCustomizations: true,
	}, cacertsCheck)
}

func cacertsCheck(meta *Metadata, config *buildconfig.BuildConfig) error {
	cacerts := config.Blueprint.Customizations.CACerts
	if cacerts == nil || len(cacerts.PEMCerts) == 0 {
		return Skip("no CA certs to check")
	}

	// Check all CA certs
	checkedCount := 0
	for i, pemCert := range cacerts.PEMCerts {
		if pemCert == "" {
			log.Printf("Skipping empty CA cert at index %d\n", i)
			continue
		}
		checkedCount++

		log.Printf("Parsing CA cert %d\n", i+1)
		block, _ := pem.Decode([]byte(pemCert))
		if block == nil {
			return Fail("failed to decode PEM certificate at index", i)
		}

		if block.Type != "CERTIFICATE" {
			return Fail("PEM block is not a CERTIFICATE at index", i, "got:", block.Type)
		}

		cert, err := x509.ParseCertificate(block.Bytes)
		if err != nil {
			return Fail("failed to parse certificate at index", i, "error:", err)
		}

		// Extract serial number (format as hex, lowercase)
		serial := strings.ToLower(cert.SerialNumber.Text(16))
		log.Printf("Extracting serial from CA cert %d: %s\n", i+1, serial)

		// Extract CN from certificate subject
		cn := cert.Subject.CommonName
		if cn == "" {
			// Fallback: try to extract from Subject.String() if CommonName is empty
			// Subject.String() format: "CN=value,OU=...,O=..."
			subjectStr := cert.Subject.String()
			if _, after, cnOk := strings.Cut(subjectStr, "CN="); cnOk {
				cnPart := after
				// CN value might be followed by , or end of string
				if before, _, pOk := strings.Cut(cnPart, ","); pOk {
					cn = before
				} else {
					cn = cnPart
				}
				cn = strings.TrimSpace(cn)
			}
		}

		if cn == "" {
			return Fail("failed to extract CN from CA cert subject at index", i)
		}

		// Check anchor file
		anchorPath := "/etc/pki/ca-trust/source/anchors/" + serial + ".pem"
		log.Printf("Checking CA cert %d anchor file serial '%s'\n", i+1, serial)
		if !Exists(anchorPath) {
			return Fail("file missing for cert", i+1, "at", anchorPath)
		}

		// Check extracted CA cert file
		log.Printf("Checking extracted CA cert %d file named '%s'\n", i+1, cn)
		bundlePath := "/etc/pki/ca-trust/extracted/pem/tls-ca-bundle.pem"
		found, err := Grep(cn, bundlePath)
		if err != nil {
			return Fail("extracted CA cert not found in the bundle for cert", i+1, "cn:", cn, "error:", err)
		}
		if !found {
			log.Printf("Pattern not found in file: %s\n", bundlePath)
			return Fail("extracted CA cert not found in the bundle for cert", i+1, "cn:", cn)
		}
		log.Printf("Pattern found in %s\n", bundlePath)
	}

	if checkedCount == 0 {
		return Skip("all CA certs were empty")
	}

	return Pass()
}
