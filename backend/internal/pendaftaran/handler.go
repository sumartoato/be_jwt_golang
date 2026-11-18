package pendaftaran

import (
	"errors"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/example/be_jwt_golang/backend/internal/auth"
	"github.com/example/be_jwt_golang/backend/internal/branchauth"
	"github.com/example/be_jwt_golang/backend/internal/httpx"
)

// PagePath is the menu item that grants access to these endpoints
// (seeded by 007_front_office_klinik_menu.sql).
const PagePath = "/front-office-klinik/pendaftaran-pasien"

// MaxPageSize caps one request; exports ask for up to this many rows.
const MaxPageSize = 5000

type Handler struct {
	repo Repository
	now  func() time.Time
}

func NewHandler(repo Repository) *Handler {
	return &Handler{repo: repo, now: time.Now}
}

// Routes registers the registration endpoints for callers whose role can open PagePath.
func (h *Handler) Routes(mux *http.ServeMux, requireAuth func(http.Handler) http.Handler, requireMenu func(string) func(http.Handler) http.Handler) {
	guard := func(f http.HandlerFunc) http.Handler { return requireAuth(requireMenu(PagePath)(f)) }
	mux.Handle("GET /api/pendaftaran", guard(h.List))
	mux.Handle("GET /api/pendaftaran/options", guard(h.Options))
	mux.Handle("GET /api/pendaftaran/pasien", guard(h.SearchPasien))
	mux.Handle("POST /api/pendaftaran", guard(h.Create))
	mux.Handle("PUT /api/pendaftaran/{id}", guard(h.Update))
	mux.Handle("POST /api/pendaftaran/{id}/cancel", guard(h.Cancel))
}

func (h *Handler) branches(r *http.Request) ([]branchauth.Branch, error) {
	id, _ := auth.IdentityFrom(r.Context())
	list, err := h.repo.UserBranches(r.Context(), id.UserID)
	if list == nil {
		list = []branchauth.Branch{}
	}
	return list, err
}

// user returns the caller's email cut to n characters (the column size).
func user(r *http.Request, n int) string {
	id, _ := auth.IdentityFrom(r.Context())
	u := []rune(id.Email)
	if len(u) > n {
		u = u[:n]
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

func optInt(r *http.Request, name string) *int {
	v, err := strconv.Atoi(r.URL.Query().Get(name))
	if err != nil || v <= 0 {
		return nil
	}
	return &v
}

func validDate(s string) bool {
	_, err := time.Parse("2006-01-02", s)
	return err == nil
}

// List returns one page of registrations at the caller's branches.
//
// Query: from, to (YYYY-MM-DD, default today in the clinic's time zone),
// page, size (1..5000), q, sort, dir, cabang, pelayanan, status.
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
	today := ClinicToday(h.now())
	from, to := q.Get("from"), q.Get("to")
	if from == "" {
		from = today
	}
	if to == "" {
		to = from
	}
	if !validDate(from) || !validDate(to) || to < from {
		httpx.WriteValidation(w, httpx.FieldErrors{"from": "Use a valid date range (YYYY-MM-DD, from ≤ to)"})
		return
	}
	if len(codes) == 0 {
		httpx.WriteJSON(w, http.StatusOK, map[string]any{"rows": []Row{}, "total": 0, "unfilteredTotal": 0, "from": from, "to": to})
		return
	}
	size := intParam(r, "size", 10, 1, MaxPageSize)
	page := intParam(r, "page", 1, 1, 1_000_000)
	sort := q.Get("sort")
	desc := q.Get("dir") != "asc"
	if _, ok := sortColumns[sort]; !ok {
		sort, desc = "id", true // newest first, like the original screen
	}
	res, err := h.repo.List(r.Context(), ListParams{
		Branches: codes, From: from, To: to, Search: strings.TrimSpace(q.Get("q")),
		IDJenis: optInt(r, "pelayanan"), IDStatus: optInt(r, "status"),
		Sort: sort, Desc: desc, Limit: size, Offset: (page - 1) * size,
	})
	if err != nil {
		httpx.InternalError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{
		"rows": res.Rows, "total": res.Total, "unfilteredTotal": res.UnfilteredTotal, "from": from, "to": to,
	})
}

// Options returns the services, visit statuses, the caller's branches and today's clinic date.
func (h *Handler) Options(w http.ResponseWriter, r *http.Request) {
	services, err := h.repo.Services(r.Context())
	if err != nil {
		httpx.InternalError(w, err)
		return
	}
	statuses, err := h.repo.Statuses(r.Context())
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
		"pelayanan": services, "status": statuses, "branches": branches, "today": ClinicToday(h.now()),
	})
}

// SearchPasien pages the patients of the caller's branches for the registration form.
func (h *Handler) SearchPasien(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	search := PasienSearch{Q: strings.TrimSpace(q.Get("q")), By: q.Get("by"), Key: strings.TrimSpace(q.Get("key"))}
	if search.By != "" && !slices.Contains(PasienSearchFields, search.By) {
		httpx.WriteError(w, http.StatusBadRequest, "by must be one of "+strings.Join(PasienSearchFields, ", "))
		return
	}
	if search.By == "umur" {
		age, err := strconv.Atoi(search.Key)
		switch {
		case search.Key == "":
			search.By = ""
		case err != nil || age < 0 || age > 150:
			httpx.WriteError(w, http.StatusBadRequest, "umur must be a whole number of years (0-150)")
			return
		default:
			search.Age = age
		}
	}
	branches, err := h.branches(r)
	if err != nil {
		httpx.InternalError(w, err)
		return
	}
	if len(branches) == 0 {
		httpx.WriteJSON(w, http.StatusOK, PasienPage{Rows: []PasienHit{}})
		return
	}
	size := intParam(r, "size", 5, 1, 100)
	page := intParam(r, "page", 1, 1, 1_000_000)
	res, err := h.repo.SearchPasien(r.Context(), branchauth.Codes(branches), search, size, (page-1)*size)
	if err != nil {
		httpx.InternalError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, res)
}

// Create registers a visit. The date may be today or earlier (clinic time).
func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	var in NewInput
	if !httpx.DecodeJSON(w, r, &in) {
		return
	}
	in.Cabang = strings.ToUpper(strings.TrimSpace(in.Cabang))
	in.Tanggal = strings.TrimSpace(in.Tanggal)
	today := ClinicToday(h.now())
	if in.Tanggal == "" {
		in.Tanggal = today
	}
	errs := httpx.FieldErrors{}
	switch {
	case !validDate(in.Tanggal):
		errs["tanggal"] = "Tanggal must be a date (YYYY-MM-DD)"
	case in.Tanggal > today:
		errs["tanggal"] = "Tanggal cannot be in the future"
	}
	if in.IDJenis <= 0 {
		errs["idJenis"] = "Choose a Pelayanan"
	}
	if in.Cabang == "" {
		errs["cabang"] = "Choose a Cabang"
	}
	if in.IDPas <= 0 {
		errs["idPas"] = "Choose a patient"
	}
	if len(errs) > 0 {
		httpx.WriteValidation(w, errs)
		return
	}
	branches, err := h.branches(r)
	if err != nil {
		httpx.InternalError(w, err)
		return
	}
	if !branchauth.Has(branches, in.Cabang) {
		httpx.WriteError(w, http.StatusForbidden, "you are not assigned to branch "+in.Cabang)
		return
	}
	id, err := h.repo.Create(r.Context(), in, user(r, 20))
	if err != nil {
		writeRepoError(w, err)
		return
	}
	h.respond(w, r, id, http.StatusCreated)
}

// load fetches a registration of one of the caller's branches; others look like "not found".
func (h *Handler) load(w http.ResponseWriter, r *http.Request) (Row, bool) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil || id <= 0 {
		httpx.WriteError(w, http.StatusNotFound, "pendaftaran not found")
		return Row{}, false
	}
	branches, err := h.branches(r)
	if err != nil {
		httpx.InternalError(w, err)
		return Row{}, false
	}
	row, err := h.repo.Get(r.Context(), id)
	if errors.Is(err, ErrNotFound) || (err == nil && !branchauth.Has(branches, row.Cabang)) {
		httpx.WriteError(w, http.StatusNotFound, "pendaftaran not found")
		return Row{}, false
	}
	if err != nil {
		httpx.InternalError(w, err)
		return Row{}, false
	}
	return row, true
}

func (h *Handler) Update(w http.ResponseWriter, r *http.Request) {
	row, ok := h.load(w, r)
	if !ok {
		return
	}
	var in EditInput
	if !httpx.DecodeJSON(w, r, &in) {
		return
	}
	if in.IDJenis <= 0 {
		httpx.WriteValidation(w, httpx.FieldErrors{"idJenis": "Choose a Pelayanan"})
		return
	}
	if err := h.repo.Update(r.Context(), row.ID, in, user(r, 20)); err != nil {
		writeRepoError(w, err)
		return
	}
	h.respond(w, r, row.ID, http.StatusOK)
}

func (h *Handler) Cancel(w http.ResponseWriter, r *http.Request) {
	row, ok := h.load(w, r)
	if !ok {
		return
	}
	var body struct {
		Reason string `json:"reason"`
	}
	if !httpx.DecodeJSON(w, r, &body) {
		return
	}
	body.Reason = strings.TrimSpace(body.Reason)
	switch {
	case body.Reason == "":
		httpx.WriteValidation(w, httpx.FieldErrors{"reason": "Enter the reason for cancelling"})
		return
	case utf8.RuneCountInString(body.Reason) > 100:
		httpx.WriteValidation(w, httpx.FieldErrors{"reason": "The reason must be at most 100 characters"})
		return
	}
	if err := h.repo.Cancel(r.Context(), row.ID, body.Reason, user(r, 30)); err != nil {
		writeRepoError(w, err)
		return
	}
	h.respond(w, r, row.ID, http.StatusOK)
}

func (h *Handler) respond(w http.ResponseWriter, r *http.Request, id, status int) {
	row, err := h.repo.Get(r.Context(), id)
	if err != nil {
		writeRepoError(w, err)
		return
	}
	httpx.WriteJSON(w, status, row)
}

func writeRepoError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrNotFound):
		httpx.WriteError(w, http.StatusNotFound, "pendaftaran not found")
	case errors.Is(err, ErrNotEditable):
		httpx.WriteError(w, http.StatusConflict, err.Error())
	case errors.Is(err, ErrUnknownPasien):
		httpx.WriteValidation(w, httpx.FieldErrors{"idPas": "The patient does not exist"})
	case errors.Is(err, ErrUnknownService):
		httpx.WriteValidation(w, httpx.FieldErrors{"idJenis": "Choose an active Pelayanan"})
	default:
		httpx.InternalError(w, err)
	}
}
