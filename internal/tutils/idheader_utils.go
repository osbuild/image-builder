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
		"internal": {%s}
	}
}`

var internalContent string = "\n			\"org_id\" : \"%s\"\n"

func getBase64Header(accountNumber string, internalContent string) string {
	return base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf(completeIdHeader, accountNumber, internalContent)))
}

// returns a base64 encoded string of the idHeader
func GetCompleteBas64Header(accountNumber string, orgId string) string {
	return getBase64Header(accountNumber, fmt.Sprintf(internalContent, orgId))
}

// returns a base64 encoded string of the idHeader without the orgId
func GetBas64HeaderWithoutOrgId(accountNumber string) string {
	return getBase64Header(accountNumber, "")
}
