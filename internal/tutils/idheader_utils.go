package tutils

import (
	"encoding/base64"
	"fmt"
)

var completeIdHeader string = `{
	"entitlements": {
		"insights": {
			"is_entitled": true
		},
		"rhel": {
			"is_entitled": true
		},
		"smart_management": {
			"is_entitled": true
		},
		"openshift": {
			"is_entitled": true
		},
		"hybrid": {
			"is_entitled": true
		},
		"migrations": {
			"is_entitled": true
		},
		"ansible": {
			"is_entitled": true
		}
	},
	"identity": {
		"type": "User",
		"user": {
		 	"username": "user",
		 	"email": "user@user.user",
		 	"first_name": "user",
		 	"last_name": "user",
		 	"is_active": true,
		 	"is_org_admin": true,
		 	"is_internal": true,
		 	"locale": "en-US"
		},
		"internal": {"org_id": "%s"},
		"account_number": "000000"
	}
}`

var idHeaderWithoutEntitlements = `{
	"identity": {
		"type": "User",
		"user": {
			"username": "user",
			"email": "user@user.user",
			"first_name": "user",
			"last_name": "user",
			"is_active": true,
			"is_org_admin": true,
			"is_internal": true,
			"locale": "en-US"
		},
		"internal": {"org_id": "%s"},
		"account_number": "000000"
	}
}`

var fedoraHeader = `{
	"user": "%s"
}`

func getBase64Header(header string, orgId string) string {
	return base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf(header, orgId)))
}

// returns a base64 encoded string of the idHeader
func GetCompleteBase64Header(orgId string) string {
	return getBase64Header(completeIdHeader, orgId)
}

// returns a base64 encoded string of the idHeader without the orgId
func GetBase64HeaderWithoutEntitlements(orgId string) string {
	return getBase64Header(idHeaderWithoutEntitlements, orgId)
}
