package identity

import (
	"context"
	"net/http"
)

type HeaderKey int

const (
	IDHeaderKey  HeaderKey = iota
	RawHeaderKey HeaderKey = iota
)

func Extractor(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rawHeaders := r.Header[FedoraIDHeader]

		if len(rawHeaders) != 1 {
			http.Error(w, "Missing identity header", http.StatusBadRequest)
			return
		}

		identity, err := FromBase64(rawHeaders[0])
		if err != nil {
			http.Error(w, "Identity header does not contain valid JSON", http.StatusBadRequest)
			return
		}

		if identity.User == "" {
			http.Error(w, "Identity does not contain a user", http.StatusBadRequest)
			return
		}

		ctx := context.WithValue(r.Context(), IDHeaderKey, identity)
		ctx = context.WithValue(ctx, RawHeaderKey, rawHeaders[0])
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
