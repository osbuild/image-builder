package identity

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
)

type identityKey int

// Internal is the "internal" field of an XRHID
type Internal struct {
	OrgID string `json:"org_id"`
}

// User is the "user" field of an XRHID
type User struct {
	Username  string `json:"username"`
	Email     string `json:"email"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Active    bool   `json:"is_active"`
	OrgAdmin  bool   `json:"is_org_admin"`
	Internal  bool   `json:"is_internal"`
	Locale    string `json:"locale"`
	UserID    string `json:"user_id"`
}

// Associate is the "associate" field of an XRHID
type Associate struct {
	Role      []string `json:"Role"`
	Email     string   `json:"email"`
	GivenName string   `json:"givenName"`
	RHatUUID  string   `json:"rhatUUID"`
	Surname   string   `json:"surname"`
}

// X509 is the "x509" field of an XRHID
type X509 struct {
	SubjectDN string `json:"subject_dn"`
	IssuerDN  string `json:"issuer_dn"`
}

// Identity is the main body of the XRHID
type Identity struct {
	AccountNumber string                 `json:"account_number"`
	OrgID         string                 `json:"org_id"`
	Internal      Internal               `json:"internal"`
	User          User                   `json:"user,omitempty"`
	System        map[string]interface{} `json:"system,omitempty"`
	Associate     Associate              `json:"associate,omitempty"`
	X509          X509                   `json:"x509,omitempty"`
	Type          string                 `json:"type"`
}

// ServiceDetails describe the services the org is entitled to
type ServiceDetails struct {
	IsEntitled bool `json:"is_entitled"`
	IsTrial    bool `json:"is_trial"`
}

// XRHID is the "identity" pricipal object set by Cloud Platform 3scale
type XRHID struct {
	Identity     Identity                  `json:"identity"`
	Entitlements map[string]ServiceDetails `json:"entitlements"`
}

// Key the key for the XRHID in that gets added to the context
const (
	Key         identityKey = iota
	IDHeaderKey identityKey = iota
)

func getErrorText(code int, reason string) string {
	return http.StatusText(code) + ": " + reason
}

func doError(w http.ResponseWriter, code int, reason string) error {
	http.Error(w, getErrorText(code, reason), code)
	return errors.New(reason)
}

// Get returns either an XRHID from the context or an empty XRHID as well as an
// ok value.  If the context does not contain an XRHID, the ok value will be
// false.
func Get(ctx context.Context) (XRHID, bool) {
	xrhid, ok := ctx.Value(Key).(XRHID)
	return xrhid, ok
}

// GetIdentityHeader returns the identity header from the given context if one
// is present as well as an ok value.  Can be used to retrieve the header and
// pass it forward to other applications.  Returns an empty string and false if
// the context does not contain an identity header.
func GetIdentityHeader(ctx context.Context) (string, bool) {
	idHeader, ok := ctx.Value(IDHeaderKey).(string)
	return idHeader, ok
}

// BasePolicy is the base policy that enforces some common checks on the XRHID.
func BasePolicy(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id, ok := Get(r.Context())
		if !ok {
			doError(w, 401, "missing identity header")
			return
		}
		if id.Identity.Type == "Associate" && id.Identity.AccountNumber == "" {
			next.ServeHTTP(w, r)
			return
		}

		if id.Identity.AccountNumber == "" || id.Identity.AccountNumber == "-1" {
			doError(w, 400, "x-rh-identity header has an invalid or missing account number")
			return
		}

		if id.Identity.OrgID == "" || id.Identity.Internal.OrgID == "" {
			doError(w, 400, "x-rh-identity header has an invalid or missing org_id")
			return
		}

		if id.Identity.Type == "" {
			doError(w, 400, "x-rh-identity header is missing type")
			return
		}
		next.ServeHTTP(w, r)
	})
}

// Extractor reads the X-Rh-Identity header and marshalls the bytes into an XRHID instance into the context.
// If the Identity is invalid, the request will be aborted.
func Extractor(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rawHeaders := r.Header["X-Rh-Identity"]

		// must have an x-rh-id header
		if len(rawHeaders) != 1 {
			doError(w, 400, "missing x-rh-identity header")
			return
		}

		// must be able to base64 decode header
		idRaw, err := base64.StdEncoding.DecodeString(rawHeaders[0])
		if err != nil {
			doError(w, 400, "unable to b64 decode x-rh-identity header")
			return
		}

		var jsonData XRHID
		err = json.Unmarshal(idRaw, &jsonData)
		if err != nil {
			doError(w, 400, "x-rh-identity header is does not contain valid JSON")
			return
		}

		topLevelOrgIDFallback(&jsonData)

		ctx := context.WithValue(r.Context(), Key, jsonData)
		ctx = context.WithValue(ctx, IDHeaderKey, rawHeaders[0])
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// if org_id is not defined at the top level, use the internal one
// https://issues.redhat.com/browse/RHCLOUD-17717
func topLevelOrgIDFallback(identity *XRHID) {
	if identity.Identity.OrgID == "" && identity.Identity.Internal.OrgID != "" {
		identity.Identity.OrgID = identity.Identity.Internal.OrgID
	}
}
