package jamkes

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/example/be_jwt_golang/backend/internal/auth"
	"github.com/example/be_jwt_golang/backend/internal/httpx"
)

// PagePath is the menu item that grants access to these endpoints
// (seeded by 006_master_data_klinik_menu.sql).
const PagePath = "/master-data-klinik/jaminan-kesehatan"

const maxTextLen = 5000 // per PKS term or list item

type Handler struct {
	repo Repository
}

func NewHandler(repo Repository) *Handler {
	return &Handler{repo: repo}
}

// Routes registers the insurer endpoints for callers whose role can open PagePath.
func (h *Handler) Routes(mux *http.ServeMux, requireAuth func(http.Handler) http.Handler, requireMenu func(string) func(http.Handler) http.Handler) {
	guard := func(f http.HandlerFunc) http.Handler { return requireAuth(requireMenu(PagePath)(f)) }
	mux.Handle("GET /api/jamkes", guard(h.List))
	mux.Handle("POST /api/jamkes", guard(h.Create))
	mux.Handle("PUT /api/jamkes/{id}", guard(h.Update))
	mux.Handle("GET /api/jamkes/{id}/pks", guard(h.Terms))
	mux.Handle("PUT /api/jamkes/{id}/pks", guard(h.SaveTerms))
	mux.Handle("GET /api/jamkes/{id}/items", guard(h.Items))
	mux.Handle("PUT /api/jamkes/{id}/items", guard(h.SaveItems))
}

func user(r *http.Request) string {
	id, _ := auth.IdentityFrom(r.Context())
	u := []rune(id.Email)
	if len(u) > 20 {
		u = u[:20]
	}
	return string(u)
}

// loadID parses the path id and checks the insurer exists.
func (h *Handler) loadID(w http.ResponseWriter, r *http.Request) (int, bool) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil || id <= 0 {
		httpx.WriteError(w, http.StatusNotFound, "jaminan kesehatan not found")
		return 0, false
	}
	if _, err := h.repo.Get(r.Context(), id); err != nil {
		writeRepoError(w, err)
		return 0, false
	}
	return id, true
}

// List returns every insurer and the insurer types.
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	list, err := h.repo.List(r.Context())
	if err != nil {
		httpx.InternalError(w, err)
		return
	}
	types, err := h.repo.Types(r.Context())
	if err != nil {
		httpx.InternalError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"jamkes": list, "types": types})
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
		httpx.WriteError(w, http.StatusNotFound, "jaminan kesehatan not found")
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
	j, err := h.repo.Get(r.Context(), id)
	if err != nil {
		writeRepoError(w, err)
		return
	}
	httpx.WriteJSON(w, status, j)
}

func (h *Handler) Terms(w http.ResponseWriter, r *http.Request) {
	id, ok := h.loadID(w, r)
	if !ok {
		return
	}
	terms, err := h.repo.Terms(r.Context(), id)
	if err != nil {
		httpx.InternalError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"terms": terms})
}

// SaveTerms replaces the insurer's PKS terms (an empty value removes a term).
func (h *Handler) SaveTerms(w http.ResponseWriter, r *http.Request) {
	id, ok := h.loadID(w, r)
	if !ok {
		return
	}
	var body struct {
		Terms []TermInput `json:"terms"`
	}
	if !httpx.DecodeJSON(w, r, &body) {
		return
	}
	seen := map[int]bool{}
	for i := range body.Terms {
		t := &body.Terms[i]
		t.Value = strings.TrimSpace(t.Value)
		if seen[t.DetailID] {
			httpx.WriteValidation(w, httpx.FieldErrors{"terms": "Each term can appear only once"})
			return
		}
		seen[t.DetailID] = true
		if utf8.RuneCountInString(t.Value) > maxTextLen {
			httpx.WriteValidation(w, httpx.FieldErrors{"terms": "A term must be at most 5000 characters"})
			return
		}
	}
	if err := h.repo.SaveTerms(r.Context(), id, body.Terms); err != nil {
		writeRepoError(w, err)
		return
	}
	h.Terms(w, r)
}

func (h *Handler) Items(w http.ResponseWriter, r *http.Request) {
	id, ok := h.loadID(w, r)
	if !ok {
		return
	}
	items, err := h.repo.Items(r.Context(), id)
	if err != nil {
		httpx.InternalError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"items": items})
}

// SaveItems replaces the insurer's numbered list; empty entries are dropped.
func (h *Handler) SaveItems(w http.ResponseWriter, r *http.Request) {
	id, ok := h.loadID(w, r)
	if !ok {
		return
	}
	var body struct {
		Items []ItemInput `json:"items"`
	}
	if !httpx.DecodeJSON(w, r, &body) {
		return
	}
	if len(body.Items) > 500 {
		httpx.WriteValidation(w, httpx.FieldErrors{"items": "At most 500 items"})
		return
	}
	items := []ItemInput{}
	for _, it := range body.Items {
		it.Value = strings.TrimSpace(it.Value)
		if it.Value == "" {
			continue
		}
		if utf8.RuneCountInString(it.Value) > maxTextLen {
			httpx.WriteValidation(w, httpx.FieldErrors{"items": "An item must be at most 5000 characters"})
			return
		}
		items = append(items, it)
	}
	if err := h.repo.SaveItems(r.Context(), id, items); err != nil {
		writeRepoError(w, err)
		return
	}
	h.Items(w, r)
}

func writeRepoError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrNotFound):
		httpx.WriteError(w, http.StatusNotFound, "jaminan kesehatan not found")
	case errors.Is(err, ErrUnknownType):
		httpx.WriteValidation(w, httpx.FieldErrors{"idType": "Choose a value from the list"})
	case errors.Is(err, ErrUnknownTerm), errors.Is(err, ErrDuplicateKey):
		httpx.WriteValidation(w, httpx.FieldErrors{"terms": "Unknown or repeated PKS term"})
	default:
		httpx.InternalError(w, err)
	}
}
