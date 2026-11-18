package orglokasi

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/example/be_jwt_golang/backend/internal/auth"
	"github.com/example/be_jwt_golang/backend/internal/branchauth"
	"github.com/example/be_jwt_golang/backend/internal/httpx"
)

// PagePath is the menu item that grants access to these endpoints
// (seeded by 015_master_data_satu_sehat_menu.sql).
const PagePath = "/master-data-satu-sehat/organisasi-lokasi"

type Handler struct {
	repo Repository
}

func NewHandler(repo Repository) *Handler {
	return &Handler{repo: repo}
}

// Routes registers the endpoints for callers whose role can open PagePath.
func (h *Handler) Routes(mux *http.ServeMux, requireAuth func(http.Handler) http.Handler, requireMenu func(string) func(http.Handler) http.Handler) {
	guard := func(f http.HandlerFunc) http.Handler { return requireAuth(requireMenu(PagePath)(f)) }
	mux.Handle("GET /api/organisasi-lokasi", guard(h.List))
	mux.Handle("PUT /api/organisasi-lokasi/{id}", guard(h.Update))
}

func (h *Handler) branches(r *http.Request) ([]branchauth.Branch, error) {
	id, _ := auth.IdentityFrom(r.Context())
	list, err := h.repo.UserBranches(r.Context(), id.UserID)
	if list == nil {
		list = []branchauth.Branch{}
	}
	return list, err
}

// user returns the caller's email cut to the updatedby column size (30).
func user(r *http.Request) string {
	id, _ := auth.IdentityFrom(r.Context())
	u := []rune(id.Email)
	if len(u) > 30 {
		u = u[:30]
	}
	return string(u)
}

// List returns the rows of the caller's branches, and those branches.
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	branches, err := h.branches(r)
	if err != nil {
		httpx.InternalError(w, err)
		return
	}
	rows := []Row{}
	if len(branches) > 0 {
		if rows, err = h.repo.List(r.Context(), branchauth.Codes(branches)); err != nil {
			httpx.InternalError(w, err)
			return
		}
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"rows": rows, "branches": branches})
}

// Update changes the names and SATUSEHAT ids of a row of one of the caller's
// branches; rows of other branches look like "not found".
func (h *Handler) Update(w http.ResponseWriter, r *http.Request) {
	notFound := func() { httpx.WriteError(w, http.StatusNotFound, "organisasi & lokasi not found") }
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil || id <= 0 {
		notFound()
		return
	}
	var in Input
	if !httpx.DecodeJSON(w, r, &in) {
		return
	}
	if errs := in.Normalize(); len(errs) > 0 {
		httpx.WriteValidation(w, errs)
		return
	}
	branches, err := h.branches(r)
	if err != nil {
		httpx.InternalError(w, err)
		return
	}
	row, err := h.repo.Get(r.Context(), id)
	if errors.Is(err, ErrNotFound) || (err == nil && !branchauth.Has(branches, row.Cabang)) {
		notFound()
		return
	}
	if err != nil {
		httpx.InternalError(w, err)
		return
	}
	row, err = h.repo.Update(r.Context(), id, in, user(r))
	if errors.Is(err, ErrNotFound) {
		notFound()
		return
	}
	if err != nil {
		httpx.InternalError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, row)
}
