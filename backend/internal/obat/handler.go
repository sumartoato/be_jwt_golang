package obat

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
const PagePath = "/master-data-klinik/obat"

type Handler struct {
	repo Repository
}

func NewHandler(repo Repository) *Handler {
	return &Handler{repo: repo}
}

// Routes registers the drug endpoints for callers whose role can open PagePath.
func (h *Handler) Routes(mux *http.ServeMux, requireAuth func(http.Handler) http.Handler, requireMenu func(string) func(http.Handler) http.Handler) {
	guard := func(f http.HandlerFunc) http.Handler { return requireAuth(requireMenu(PagePath)(f)) }
	mux.Handle("GET /api/obat", guard(h.List))
	mux.Handle("GET /api/obat/options", guard(h.Options))
	mux.Handle("POST /api/obat", guard(h.Create))
	mux.Handle("PUT /api/obat/{id}", guard(h.Update))
	mux.Handle("PUT /api/obat/{id}/stock/{cabang}", guard(h.SetStock))
}

// branches returns the caller's branches (refbranchauth), read on every
// request so a new mapping applies at once.
func (h *Handler) branches(r *http.Request) ([]Branch, []string, error) {
	id, _ := auth.IdentityFrom(r.Context())
	list, err := h.repo.UserBranches(r.Context(), id.UserID)
	if err != nil {
		return nil, nil, err
	}
	return list, branchauth.Codes(list), nil
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

// List returns every drug with stock at the caller's branches, and those branches.
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	branches, codes, err := h.branches(r)
	if err != nil {
		httpx.InternalError(w, err)
		return
	}
	list, err := h.repo.List(r.Context(), codes)
	if err != nil {
		httpx.InternalError(w, err)
		return
	}
	if branches == nil {
		branches = []Branch{}
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"obat": list, "branches": branches})
}

func (h *Handler) Options(w http.ResponseWriter, r *http.Request) {
	o, err := h.repo.Options(r.Context())
	if err != nil {
		httpx.InternalError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, o)
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

// SetStock creates or updates the drug's stock and prices at one branch.
// Only branches the caller is mapped to are allowed.
func (h *Handler) SetStock(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	cabang := strings.ToUpper(strings.TrimSpace(r.PathValue("cabang")))
	branches, _, err := h.branches(r)
	if err != nil {
		httpx.InternalError(w, err)
		return
	}
	if !branchauth.Has(branches, cabang) {
		httpx.WriteError(w, http.StatusForbidden, "you are not assigned to branch "+cabang)
		return
	}
	var in StockInput
	if !httpx.DecodeJSON(w, r, &in) {
		return
	}
	if errs := in.Validate(); len(errs) > 0 {
		httpx.WriteValidation(w, errs)
		return
	}
	if err := h.repo.UpsertStock(r.Context(), id, cabang, in); err != nil {
		writeRepoError(w, err)
		return
	}
	h.respond(w, r, id, http.StatusOK)
}

// respond answers with the drug as the caller sees it (stock at their branches).
func (h *Handler) respond(w http.ResponseWriter, r *http.Request, id, status int) {
	_, codes, err := h.branches(r)
	if err != nil {
		httpx.InternalError(w, err)
		return
	}
	o, err := h.repo.Get(r.Context(), id, codes)
	if err != nil {
		writeRepoError(w, err)
		return
	}
	httpx.WriteJSON(w, status, o)
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
		httpx.WriteError(w, http.StatusNotFound, "obat not found")
		return 0, false
	}
	return id, true
}

func writeRepoError(w http.ResponseWriter, err error) {
	var ref InvalidRefError
	switch {
	case errors.Is(err, ErrNotFound):
		httpx.WriteError(w, http.StatusNotFound, "obat not found")
	case errors.As(err, &ref):
		httpx.WriteValidation(w, httpx.FieldErrors{ref.Field: "Choose a value from the list"})
	default:
		httpx.InternalError(w, err)
	}
}
