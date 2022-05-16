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
		"account_number": "%s",
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
		"internal": {"org_id": "%s"}
	}
}`

var idHeaderWithoutEntitlements = `{
	"identity": {
		"account_number": "%s",
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
		"internal": {"org_id": "%s"}
	}
}`

func getBase64Header(header string, accountNumber string, orgId string) string {
	return base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf(header, accountNumber, orgId)))
}

// returns a base64 encoded string of the idHeader
func GetCompleteBas64Header(accountNumber string, orgId string) string {
	return getBase64Header(completeIdHeader, accountNumber, orgId)
}

// returns a base64 encoded string of the idHeader without the orgId
func GetBase64HeaderWithoutEntitlements(accountNumber string, orgId string) string {
	return getBase64Header(idHeaderWithoutEntitlements, accountNumber, orgId)
}
