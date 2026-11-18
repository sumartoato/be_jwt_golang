package pasien

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/example/be_jwt_golang/backend/internal/auth"
	"github.com/example/be_jwt_golang/backend/internal/branchauth"
	"github.com/example/be_jwt_golang/backend/internal/httpx"
)

// PagePath is the menu item that grants access to these endpoints
// (seeded by 006_master_data_klinik_menu.sql).
const PagePath = "/master-data-klinik/pasien"

// MaxPageSize caps one request; exports ask for up to this many rows.
const MaxPageSize = 5000

type Handler struct {
	repo Repository
	now  func() time.Time
}

func NewHandler(repo Repository) *Handler {
	return &Handler{repo: repo, now: time.Now}
}

// Routes registers the patient endpoints for callers whose role can open PagePath.
func (h *Handler) Routes(mux *http.ServeMux, requireAuth func(http.Handler) http.Handler, requireMenu func(string) func(http.Handler) http.Handler) {
	guard := func(f http.HandlerFunc) http.Handler { return requireAuth(requireMenu(PagePath)(f)) }
	mux.Handle("GET /api/pasien", guard(h.List))
	mux.Handle("GET /api/pasien/options", guard(h.Options))
	mux.Handle("GET /api/pasien/kelurahan", guard(h.Kelurahan))
	mux.Handle("GET /api/pasien/{id}", guard(h.Get))
	mux.Handle("POST /api/pasien", guard(h.Create))
	mux.Handle("PUT /api/pasien/{id}", guard(h.Update))
}

// branches returns the caller's branches, read on every request.
func (h *Handler) branches(r *http.Request) ([]branchauth.Branch, error) {
	id, _ := auth.IdentityFrom(r.Context())
	list, err := h.repo.UserBranches(r.Context(), id.UserID)
	if list == nil {
		list = []branchauth.Branch{}
	}
	return list, err
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

func intParam(r *http.Request, name string, def, min, max int) int {
	v, err := strconv.Atoi(r.URL.Query().Get(name))
	if err != nil {
		return def
	}
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}

// List returns one page of the caller's patients.
//
// Query: page (from 1), size (1..5000), q (search name, No RM, NIK, address or
// Kode), sort (id, noRm, nama, tmptLahir, tglLahir, umur, alamat, rt, rw,
// cabang), dir (asc|desc), cabang (one of the caller's branches).
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	branches, err := h.branches(r)
	if err != nil {
		httpx.InternalError(w, err)
		return
	}
	q := r.URL.Query()
	codes := branchauth.Codes(branches)
	if c := strings.ToUpper(strings.TrimSpace(q.Get("cabang"))); c != "" {
		if !branchauth.Has(branches, c) {
			httpx.WriteError(w, http.StatusForbidden, "you are not assigned to branch "+c)
			return
		}
		codes = []string{c}
	}
	if len(codes) == 0 {
		httpx.WriteJSON(w, http.StatusOK, map[string]any{"rows": []Row{}, "total": 0, "unfilteredTotal": 0, "branches": branches})
		return
	}
	size := intParam(r, "size", 10, 1, MaxPageSize)
	page := intParam(r, "page", 1, 1, 1_000_000)
	sort := q.Get("sort")
	if _, ok := sortColumns[sort]; !ok {
		sort = "id"
	}
	desc := q.Get("dir") != "asc"
	if q.Get("sort") == "" {
		desc = true // newest first, like the original screen
	}
	res, err := h.repo.List(r.Context(), ListParams{
		Branches: codes,
		Search:   strings.TrimSpace(q.Get("q")),
		Sort:     sort,
		Desc:     desc,
		Limit:    size,
		Offset:   (page - 1) * size,
	})
	if err != nil {
		httpx.InternalError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{
		"rows": res.Rows, "total": res.Total, "unfilteredTotal": res.UnfilteredTotal, "branches": branches,
	})
}

func (h *Handler) Options(w http.ResponseWriter, r *http.Request) {
	o, err := h.repo.Options(r.Context())
	if err != nil {
		httpx.InternalError(w, err)
		return
	}
	branches, err := h.branches(r)
	if err != nil {
		httpx.InternalError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{
		"gender": o.Gender, "golDarah": o.GolDarah, "agama": o.Agama, "statusKwn": o.StatusKwn,
		"jamkes": o.Jamkes, "branches": branches,
	})
}

// Kelurahan searches kelurahan (with kecamatan, kab/kota and provinsi) by name.
func (h *Handler) Kelurahan(w http.ResponseWriter, r *http.Request) {
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	if len([]rune(q)) < 2 {
		httpx.WriteJSON(w, http.StatusOK, map[string]any{"kelurahan": []Kelurahan{}})
		return
	}
	list, err := h.repo.SearchKelurahan(r.Context(), q, 30)
	if err != nil {
		httpx.InternalError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"kelurahan": list})
}

// load fetches a patient the caller may see; other branches look like "not found".
func (h *Handler) load(w http.ResponseWriter, r *http.Request, branches []branchauth.Branch) (Pasien, bool) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil || id <= 0 {
		httpx.WriteError(w, http.StatusNotFound, "pasien not found")
		return Pasien{}, false
	}
	p, err := h.repo.Get(r.Context(), id)
	if errors.Is(err, ErrNotFound) || (err == nil && !branchauth.Has(branches, p.Cabang)) {
		httpx.WriteError(w, http.StatusNotFound, "pasien not found")
		return Pasien{}, false
	}
	if err != nil {
		httpx.InternalError(w, err)
		return Pasien{}, false
	}
	return p, true
}

func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	branches, err := h.branches(r)
	if err != nil {
		httpx.InternalError(w, err)
		return
	}
	if p, ok := h.load(w, r, branches); ok {
		httpx.WriteJSON(w, http.StatusOK, p)
	}
}

// decode reads and validates the body; the branch must be one of the caller's.
func (h *Handler) decode(w http.ResponseWriter, r *http.Request, branches []branchauth.Branch) (Input, bool) {
	var in Input
	if !httpx.DecodeJSON(w, r, &in) {
		return in, false
	}
	if errs := in.Normalize(h.now()); len(errs) > 0 {
		httpx.WriteValidation(w, errs)
		return in, false
	}
	if !branchauth.Has(branches, in.Cabang) {
		httpx.WriteError(w, http.StatusForbidden, "you are not assigned to branch "+in.Cabang)
		return in, false
	}
	return in, true
}

func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	branches, err := h.branches(r)
	if err != nil {
		httpx.InternalError(w, err)
		return
	}
	in, ok := h.decode(w, r, branches)
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

// Update edits a patient of one of the caller's branches; moving it to another
// branch is only allowed into one of the caller's branches too.
func (h *Handler) Update(w http.ResponseWriter, r *http.Request) {
	branches, err := h.branches(r)
	if err != nil {
		httpx.InternalError(w, err)
		return
	}
	current, ok := h.load(w, r, branches)
	if !ok {
		return
	}
	in, ok := h.decode(w, r, branches)
	if !ok {
		return
	}
	if err := h.repo.Update(r.Context(), current.ID, in, user(r)); err != nil {
		writeRepoError(w, err)
		return
	}
	h.respond(w, r, current.ID, http.StatusOK)
}

func (h *Handler) respond(w http.ResponseWriter, r *http.Request, id, status int) {
	p, err := h.repo.Get(r.Context(), id)
	if err != nil {
		writeRepoError(w, err)
		return
	}
	httpx.WriteJSON(w, status, p)
}

func writeRepoError(w http.ResponseWriter, err error) {
	var ref InvalidRefError
	switch {
	case errors.Is(err, ErrNotFound):
		httpx.WriteError(w, http.StatusNotFound, "pasien not found")
	case errors.As(err, &ref):
		httpx.WriteValidation(w, httpx.FieldErrors{ref.Field: "Choose a value from the list"})
	case errors.Is(err, ErrNoRMExhaust):
		httpx.WriteError(w, http.StatusConflict, err.Error())
	default:
		httpx.InternalError(w, err)
	}
}
