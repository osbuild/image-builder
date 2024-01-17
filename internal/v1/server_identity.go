package v1

import (
	"net/http"

	"github.com/labstack/echo/v4"
	fedora_identity "github.com/osbuild/community-gateway/oidc-authorizer/pkg/identity"
	rh_identity "github.com/redhatinsights/identity"
	"github.com/sirupsen/logrus"
)

type Identity struct {
	rhid *rh_identity.XRHID
	fid  *fedora_identity.Identity
}

// return the Identity Header if there is a valid one in the request
func (s *Server) getIdentity(ctx echo.Context) (*Identity, error) {
	if s.fedoraAuth {
		fid, ok := ctx.Request().Context().Value(fedora_identity.IDHeaderKey).(*fedora_identity.Identity)
		if !ok {
			return nil, echo.NewHTTPError(http.StatusInternalServerError, "Identity Header missing in request handler")
		}
		return &Identity{
			fid: fid,
		}, nil
	}

	rhid, ok := rh_identity.Get(ctx.Request().Context())
	if !ok {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Identity Header missing in request handler")
	}

	return &Identity{
		rhid: &rhid,
	}, nil
}

func (i *Identity) OrgID() string {
	if i.rhid != nil {
		return i.rhid.Identity.OrgID
	}
	if i.fid != nil {
		return i.fid.User
	}
	return ""
}

func (i *Identity) AccountNumber() string {
	if i.rhid != nil {
		return i.rhid.Identity.AccountNumber
	}
	return ""
}

func (i *Identity) Email() string {
	if i.rhid != nil {
		return i.rhid.Identity.User.Email
	}
	return ""
}

func (i *Identity) Type() string {
	if i.rhid != nil {
		return i.rhid.Identity.Type
	}
	return ""
}

func (i *Identity) IsEntitled(ask string) bool {
	if i.rhid != nil {
		entitled, ok := i.rhid.Entitlements[ask]
		if !ok {
			// the user's org does not have an associated EBS account number, these
			// are associated when a billing relationship exists, which is a decent
			// proxy for RHEL entitlements
			logrus.Error("RHEL entitlement not present in identity header")
			return i.AccountNumber() != ""
		}
		return entitled.IsEntitled
	}
	return false
}
