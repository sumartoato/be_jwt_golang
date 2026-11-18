package dokter

import (
	"errors"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/example/be_jwt_golang/backend/internal/auth"
	"github.com/example/be_jwt_golang/backend/internal/branchauth"
	"github.com/example/be_jwt_golang/backend/internal/httpx"
)

// PagePath is the menu item that grants access to these endpoints
// (seeded by 006_master_data_klinik_menu.sql).
const PagePath = "/master-data-klinik/dokter"

type Handler struct {
	repo Repository
	now  func() time.Time
}

func NewHandler(repo Repository) *Handler {
	return &Handler{repo: repo, now: time.Now}
}

// Routes registers the doctor endpoints for callers whose role can open PagePath.
func (h *Handler) Routes(mux *http.ServeMux, requireAuth func(http.Handler) http.Handler, requireMenu func(string) func(http.Handler) http.Handler) {
	guard := func(f http.HandlerFunc) http.Handler { return requireAuth(requireMenu(PagePath)(f)) }
	mux.Handle("GET /api/dokter", guard(h.List))
	mux.Handle("GET /api/dokter/options", guard(h.Options))
	mux.Handle("POST /api/dokter", guard(h.Create))
	mux.Handle("PUT /api/dokter/{id}", guard(h.Update))
	mux.Handle("GET /api/dokter/{id}/tindakan", guard(h.Tindakan))
	mux.Handle("PATCH /api/dokter/{id}/tindakan", guard(h.ChangeTindakan))
	mux.Handle("GET /api/dokter/{id}/cabang", guard(h.Cabang))
	mux.Handle("PATCH /api/dokter/{id}/cabang", guard(h.ChangeCabang))
}

func (h *Handler) userBranches(r *http.Request) ([]branchauth.Branch, error) {
	id, _ := auth.IdentityFrom(r.Context())
	list, err := h.repo.UserBranches(r.Context(), id.UserID)
	if list == nil {
		list = []branchauth.Branch{}
	}
	return list, err
}

func user(r *http.Request) string {
	id, _ := auth.IdentityFrom(r.Context())
	u := []rune(id.Email)
	if len(u) > 20 {
		u = u[:20]
	}
	return string(u)
}

func pathID(w http.ResponseWriter, r *http.Request) (int, bool) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil || id <= 0 {
		httpx.WriteError(w, http.StatusNotFound, "dokter not found")
		return 0, false
	}
	return id, true
}

// List returns every doctor (with all practice branches) and every branch name.
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	list, err := h.repo.List(r.Context())
	if err != nil {
		httpx.InternalError(w, err)
		return
	}
	branches, err := h.repo.AllBranches(r.Context())
	if err != nil {
		httpx.InternalError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"dokter": list, "branches": branches})
}

func (h *Handler) Options(w http.ResponseWriter, r *http.Request) {
	o, err := h.repo.Options(r.Context())
	if err != nil {
		httpx.InternalError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, o)
}

func (h *Handler) decode(w http.ResponseWriter, r *http.Request) (Input, bool) {
	var in Input
	if !httpx.DecodeJSON(w, r, &in) {
		return in, false
	}
	if errs := in.Normalize(h.now()); len(errs) > 0 {
		httpx.WriteValidation(w, errs)
		return in, false
	}
	return in, true
}

func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	in, ok := h.decode(w, r)
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
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	in, ok := h.decode(w, r)
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
	d, err := h.repo.Get(r.Context(), id)
	if err != nil {
		writeRepoError(w, err)
		return
	}
	httpx.WriteJSON(w, status, d)
}

// Tindakan returns the doctor's procedures and the ones that can be added.
func (h *Handler) Tindakan(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	if _, err := h.repo.Get(r.Context(), id); err != nil {
		writeRepoError(w, err)
		return
	}
	assigned, available, err := h.repo.Tindakan(r.Context(), id)
	if err != nil {
		httpx.InternalError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"assigned": assigned, "available": available})
}

type intChange struct {
	Add    []int `json:"add"`
	Remove []int `json:"remove"`
}

// ChangeTindakan adds and removes procedures of a doctor in one transaction.
func (h *Handler) ChangeTindakan(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	var body intChange
	if !httpx.DecodeJSON(w, r, &body) {
		return
	}
	if len(body.Add)+len(body.Remove) > 5000 {
		httpx.WriteValidation(w, httpx.FieldErrors{"tindakan": "Too many procedures in one change"})
		return
	}
	if _, err := h.repo.Get(r.Context(), id); err != nil {
		writeRepoError(w, err)
		return
	}
	if err := h.repo.ChangeTindakan(r.Context(), id, body.Add, body.Remove); err != nil {
		writeRepoError(w, err)
		return
	}
	h.Tindakan(w, r)
}

// Cabang returns the doctor's practice branches, the branches that can be
// added, and which branches the caller may change.
func (h *Handler) Cabang(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	d, err := h.repo.Get(r.Context(), id)
	if err != nil {
		writeRepoError(w, err)
		return
	}
	all, err := h.repo.AllBranches(r.Context())
	if err != nil {
		httpx.InternalError(w, err)
		return
	}
	mine, err := h.userBranches(r)
	if err != nil {
		httpx.InternalError(w, err)
		return
	}
	has := map[string]bool{}
	for _, c := range d.CabangPraktek {
		has[c] = true
	}
	assigned, available := []branchauth.Branch{}, []branchauth.Branch{}
	for _, b := range all {
		if has[b.Kode] {
			assigned = append(assigned, b)
		} else {
			available = append(available, b)
		}
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{
		"assigned": assigned, "available": available, "editable": branchauth.Codes(mine),
	})
}

type codeChange struct {
	Add    []string `json:"add"`
	Remove []string `json:"remove"`
}

func cleanCodes(in []string) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, c := range in {
		c = strings.ToUpper(strings.TrimSpace(c))
		if c != "" && !seen[c] {
			seen[c] = true
			out = append(out, c)
		}
	}
	sort.Strings(out)
	return out
}

// ChangeCabang adds and removes practice branches. Every branch in the change
// must be one of the caller's branches.
func (h *Handler) ChangeCabang(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	var body codeChange
	if !httpx.DecodeJSON(w, r, &body) {
		return
	}
	add, remove := cleanCodes(body.Add), cleanCodes(body.Remove)
	mine, err := h.userBranches(r)
	if err != nil {
		httpx.InternalError(w, err)
		return
	}
	for _, c := range append(append([]string{}, add...), remove...) {
		if !branchauth.Has(mine, c) {
			httpx.WriteError(w, http.StatusForbidden, "you are not assigned to branch "+c)
			return
		}
	}
	if _, err := h.repo.Get(r.Context(), id); err != nil {
		writeRepoError(w, err)
		return
	}
	if err := h.repo.ChangeCabang(r.Context(), id, add, remove); err != nil {
		writeRepoError(w, err)
		return
	}
	h.Cabang(w, r)
}

func writeRepoError(w http.ResponseWriter, err error) {
	var ref InvalidRefError
	switch {
	case errors.Is(err, ErrNotFound):
		httpx.WriteError(w, http.StatusNotFound, "dokter not found")
	case errors.As(err, &ref):
		httpx.WriteValidation(w, httpx.FieldErrors{ref.Field: "Choose a value from the list"})
	default:
		httpx.InternalError(w, err)
	}
}
