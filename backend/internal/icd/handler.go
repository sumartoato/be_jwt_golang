package icd

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/example/be_jwt_golang/backend/internal/auth"
	"github.com/example/be_jwt_golang/backend/internal/httpx"
)

// PagePath is the menu item that grants access to these endpoints
// (seeded by 006_master_data_klinik_menu.sql).
const PagePath = "/master-data-klinik/icd"

// MaxPageSize caps one request; exports ask for up to this many rows.
const MaxPageSize = 5000

type Handler struct {
	repo Repository
}

func NewHandler(repo Repository) *Handler {
	return &Handler{repo: repo}
}

// Routes registers the ICD endpoints for callers whose role can open PagePath.
func (h *Handler) Routes(mux *http.ServeMux, requireAuth func(http.Handler) http.Handler, requireMenu func(string) func(http.Handler) http.Handler) {
	guard := func(f http.HandlerFunc) http.Handler { return requireAuth(requireMenu(PagePath)(f)) }
	mux.Handle("GET /api/icd", guard(h.List))
	mux.Handle("GET /api/icd/options", guard(h.Options))
	mux.Handle("POST /api/icd", guard(h.Create))
	mux.Handle("PUT /api/icd/{id}", guard(h.Update))
}

func user(r *http.Request) string {
	id, _ := auth.IdentityFrom(r.Context())
	u := []rune(id.Email)
	if len(u) > 20 {
		u = u[:20]
	}
	return string(u)
}

func intParam(r *http.Request, name string, def, lo, hi int) int {
	v, err := strconv.Atoi(r.URL.Query().Get(name))
	if err != nil {
		return def
	}
	return min(max(v, lo), hi)
}

// List returns one page. Query: page, size (1..5000), q (code, name or type),
// sort (id, kode, nama, jenis), dir (asc|desc).
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	size := intParam(r, "size", 10, 1, MaxPageSize)
	page := intParam(r, "page", 1, 1, 1_000_000)
	sort := q.Get("sort")
	if _, ok := sortColumns[sort]; !ok {
		sort = "id"
	}
	res, err := h.repo.List(r.Context(), ListParams{
		Search: strings.TrimSpace(q.Get("q")),
		Sort:   sort,
		Desc:   q.Get("dir") == "desc",
		Limit:  size,
		Offset: (page - 1) * size,
	})
	if err != nil {
		httpx.InternalError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, res)
}

func (h *Handler) Options(w http.ResponseWriter, r *http.Request) {
	jenis, err := h.repo.Jenis(r.Context())
	if err != nil {
		httpx.InternalError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"jenis": jenis})
}

func decode(w http.ResponseWriter, r *http.Request) (Input, bool) {
	var in Input
	if !httpx.DecodeJSON(w, r, &in) {
		return in, false
	}
	if errs := in.Normalize(); len(errs) > 0 {
		httpx.WriteValidation(w, errs)
		return in, false
	}
	return in, true
}

func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	in, ok := decode(w, r)
	if !ok {
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
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil || id <= 0 {
		httpx.WriteError(w, http.StatusNotFound, "icd not found")
		return
	}
	in, ok := decode(w, r)
	if !ok {
		return
	}
	if err := h.repo.Update(r.Context(), id, in, user(r)); err != nil {
		writeRepoError(w, err)
		return
	}
	h.respond(w, r, id, http.StatusOK)
}

func (h *Handler) respond(w http.ResponseWriter, r *http.Request, id, status int) {
	x, err := h.repo.Get(r.Context(), id)
	if err != nil {
		writeRepoError(w, err)
		return
	}
	httpx.WriteJSON(w, status, x)
}

func writeRepoError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrNotFound):
		httpx.WriteError(w, http.StatusNotFound, "icd not found")
	case errors.Is(err, ErrDuplicate):
		httpx.WriteJSON(w, http.StatusConflict, map[string]any{
			"error": "kode ICD already exists", "errors": httpx.FieldErrors{"kode": "Kode ICD-X is already used"},
		})
	case errors.Is(err, ErrUnknownJenis):
		httpx.WriteValidation(w, httpx.FieldErrors{"idJenis": "Choose a value from the list"})
	default:
		httpx.InternalError(w, err)
	}
}
