package auth

import (
	"context"
	"net/http"
	"strings"
)

type ctxKey struct{}

// Identity is the authenticated user extracted from the JWT.
type Identity struct {
	UserID string
	Email  string
}

func IdentityFrom(ctx context.Context) (Identity, bool) {
	id, ok := ctx.Value(ctxKey{}).(Identity)
	return id, ok
}

// Middleware requires a valid "Authorization: Bearer <token>" header.
func (m *TokenManager) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token, found := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
		token = strings.TrimSpace(token)
		if !found || token == "" {
			writeError(w, http.StatusUnauthorized, "missing bearer token")
			return
		}
		claims, err := m.Parse(token)
		if err != nil {
			writeError(w, http.StatusUnauthorized, "invalid or expired token")
			return
		}
		ctx := context.WithValue(r.Context(), ctxKey{}, Identity{UserID: claims.Subject, Email: claims.Email})
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
