package auth

import (
	"context"
	"net/http"
	"strings"

	"github.com/charlesduarte/net-control/backend/internal/platform/httpserver"
)

type principalKey struct{}

func Authenticate(tokens *TokenManager, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		scheme, token, ok := strings.Cut(r.Header.Get("Authorization"), " ")
		if !ok || !strings.EqualFold(scheme, "Bearer") || token == "" {
			httpserver.WriteError(w, r, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required")
			return
		}
		principal, err := tokens.Parse(token)
		if err != nil {
			httpserver.WriteError(w, r, http.StatusUnauthorized, "UNAUTHORIZED", "Invalid or expired token")
			return
		}
		ctx := context.WithValue(r.Context(), principalKey{}, principal)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func RequirePermission(code string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		principal, ok := PrincipalFromContext(r.Context())
		if !ok {
			httpserver.WriteError(w, r, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required")
			return
		}
		for _, permission := range principal.Permissions {
			if permission == code {
				next.ServeHTTP(w, r)
				return
			}
		}
		httpserver.WriteError(w, r, http.StatusForbidden, "FORBIDDEN", "Permission denied")
	})
}

func PrincipalFromContext(ctx context.Context) (Principal, bool) {
	principal, ok := ctx.Value(principalKey{}).(Principal)
	return principal, ok
}
