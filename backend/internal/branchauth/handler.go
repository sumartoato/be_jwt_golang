package branchauth

import (
	"context"
	"net/http"

	"github.com/example/be_jwt_golang/backend/internal/auth"
	"github.com/example/be_jwt_golang/backend/internal/httpx"
)

// MineHandler serves GET /api/me/branches: the branches the logged-in user is
// mapped to (refbranchauth), for the home page. It needs a login but no menu,
// because it only reveals the caller's own mapping.
func MineHandler(forUser func(ctx context.Context, userID string) ([]Branch, error)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, _ := auth.IdentityFrom(r.Context())
		list, err := forUser(r.Context(), id.UserID)
		if err != nil {
			httpx.InternalError(w, err)
			return
		}
		if list == nil {
			list = []Branch{}
		}
		httpx.WriteJSON(w, http.StatusOK, map[string]any{"branches": list})
	}
}
