package tindakan

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/example/be_jwt_golang/backend/internal/auth"
	"github.com/example/be_jwt_golang/backend/internal/branchauth"
	"github.com/example/be_jwt_golang/backend/internal/httpx"
)

// PagePath is the menu item that grants access to these endpoints
// (seeded by 006_master_data_klinik_menu.sql).
const PagePath = "/master-data-klinik/tindakan"

type Handler struct {
	repo Repository
}

func NewHandler(repo Repository) *Handler {
	return &Handler{repo: repo}
}

// Routes registers the procedure endpoints for callers whose role can open PagePath.
func (h *Handler) Routes(mux *http.ServeMux, requireAuth func(http.Handler) http.Handler, requireMenu func(string) func(http.Handler) http.Handler) {
	guard := func(f http.HandlerFunc) http.Handler { return requireAuth(requireMenu(PagePath)(f)) }
	mux.Handle("GET /api/tindakan", guard(h.List))
	mux.Handle("GET /api/tindakan/options", guard(h.Options))
	mux.Handle("POST /api/tindakan", guard(h.Create))
	mux.Handle("PUT /api/tindakan/{id}", guard(h.Update))
	mux.Handle("PUT /api/tindakan/{id}/price/{cabang}", guard(h.SetPrice))
}

// branches returns the caller's branches, read on every request.
func (h *Handler) branches(r *http.Request) ([]branchauth.Branch, error) {
	id, _ := auth.IdentityFrom(r.Context())
	return h.repo.UserBranches(r.Context(), id.UserID)
}

// user is what goes into create_user/update_user (varchar(20)).
func user(r *http.Request) string {
	id, _ := auth.IdentityFrom(r.Context())
	u := []rune(id.Email)
	if len(u) > 20 {
		u = u[:20]
	}
	return string(u)
}

// List returns every procedure with prices at the caller's branches, and those branches.
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	branches, err := h.branches(r)
	if err != nil {
		httpx.InternalError(w, err)
		return
	}
	list, err := h.repo.List(r.Context(), branchauth.Codes(branches))
	if err != nil {
		httpx.InternalError(w, err)
		return
	}
	if branches == nil {
		branches = []branchauth.Branch{} // JSON [] rather than null
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"tindakan": list, "branches": branches})
}

func (h *Handler) Options(w http.ResponseWriter, r *http.Request) {
	jenis, err := h.repo.Jenis(r.Context())
	if err != nil {
		httpx.InternalError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"jenis": jenis})
}

func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	var in Input
	if !decode(w, r, &in) {
		return
	}
	id, err := h.repo.Create(r.Context(), in, user(r))
	if err != nil {
		writeRepoError(w, err)
		return
	}
	h.respond(w, r, id, http.StatusCreated)
}

func (h *Handler) Update(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	var in Input
	if !decode(w, r, &in) {
		return
	}
	if err := h.repo.Update(r.Context(), id, in, user(r)); err != nil {
		writeRepoError(w, err)
		return
	}
	h.respond(w, r, id, http.StatusOK)
}

// SetPrice creates or updates the procedure's prices at one branch.
// Only branches the caller is mapped to are allowed.
func (h *Handler) SetPrice(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	cabang := strings.ToUpper(strings.TrimSpace(r.PathValue("cabang")))
	branches, err := h.branches(r)
	if err != nil {
		httpx.InternalError(w, err)
		return
	}
	if !branchauth.Has(branches, cabang) {
		httpx.WriteError(w, http.StatusForbidden, "you are not assigned to branch "+cabang)
		return
	}
	var in PriceInput
	if !httpx.DecodeJSON(w, r, &in) {
		return
	}
	if errs := in.Validate(); len(errs) > 0 {
		httpx.WriteValidation(w, errs)
		return
	}
	if err := h.repo.UpsertPrice(r.Context(), id, cabang, in); err != nil {
		writeRepoError(w, err)
		return
	}
	h.respond(w, r, id, http.StatusOK)
}

// respond answers with the procedure as the caller sees it (prices at their branches).
func (h *Handler) respond(w http.ResponseWriter, r *http.Request, id, status int) {
	branches, err := h.branches(r)
	if err != nil {
		httpx.InternalError(w, err)
		return
	}
	t, err := h.repo.Get(r.Context(), id, branchauth.Codes(branches))
	if err != nil {
		writeRepoError(w, err)
		return
	}
	httpx.WriteJSON(w, status, t)
}

func decode(w http.ResponseWriter, r *http.Request, in *Input) bool {
	if !httpx.DecodeJSON(w, r, in) {
		return false
	}
	if errs := in.Normalize(); len(errs) > 0 {
		httpx.WriteValidation(w, errs)
		return false
	}
	return true
}

func pathID(w http.ResponseWriter, r *http.Request) (int, bool) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil || id <= 0 {
		httpx.WriteError(w, http.StatusNotFound, "tindakan not found")
		return 0, false
	}
	return id, true
}

func writeRepoError(w http.ResponseWriter, err error) {
	if errors.Is(err, ErrNotFound) {
		httpx.WriteError(w, http.StatusNotFound, "tindakan not found")
		return
	}
	httpx.InternalError(w, err)
}
