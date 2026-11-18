package branch

import (
	"errors"
	"net/http"
	"strings"

	"github.com/example/be_jwt_golang/backend/internal/httpx"
)

// PagePath is the menu item that grants access to these endpoints
// (seeded by 012_management_menu.sql).
const PagePath = "/management/cabang"

type Handler struct {
	repo Repository
}

func NewHandler(repo Repository) *Handler {
	return &Handler{repo: repo}
}

// Routes registers the branch endpoints for callers whose role can open PagePath.
func (h *Handler) Routes(mux *http.ServeMux, requireAuth func(http.Handler) http.Handler, requireMenu func(string) func(http.Handler) http.Handler) {
	guard := func(f http.HandlerFunc) http.Handler { return requireAuth(requireMenu(PagePath)(f)) }
	mux.Handle("GET /api/branches", guard(h.List))
	mux.Handle("POST /api/branches", guard(h.Create))
	mux.Handle("PUT /api/branches/{kode}", guard(h.Update))
}

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	branches, err := h.repo.List(r.Context())
	if err != nil {
		httpx.InternalError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"branches": branches})
}

func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	in, ok := decode(w, r, true)
	if !ok {
		return
	}
	b, err := h.repo.Create(r.Context(), in)
	if err != nil {
		writeRepoError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, b)
}

func (h *Handler) Update(w http.ResponseWriter, r *http.Request) {
	// Kode is the primary key and cannot change; a kode in the body is ignored.
	kode := strings.ToUpper(strings.TrimSpace(r.PathValue("kode")))
	in, ok := decode(w, r, false)
	if !ok {
		return
	}
	b, err := h.repo.Update(r.Context(), kode, in)
	if err != nil {
		writeRepoError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, b)
}

func decode(w http.ResponseWriter, r *http.Request, isCreate bool) (Input, bool) {
	var in Input
	if !httpx.DecodeJSON(w, r, &in) {
		return in, false
	}
	if errs := in.Normalize(isCreate); len(errs) > 0 {
		httpx.WriteValidation(w, errs)
		return in, false
	}
	return in, true
}

func writeRepoError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrNotFound):
		httpx.WriteError(w, http.StatusNotFound, "branch not found")
	case errors.Is(err, ErrCodeTaken):
		httpx.WriteJSON(w, http.StatusConflict, map[string]any{
			"error": "branch code already exists", "errors": httpx.FieldErrors{"kode": "Kode Cabang is already used"},
		})
	default:
		httpx.InternalError(w, err)
	}
}
