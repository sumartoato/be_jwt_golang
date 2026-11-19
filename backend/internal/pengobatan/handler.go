package pengobatan

import (
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/example/be_jwt_golang/backend/internal/auth"
	"github.com/example/be_jwt_golang/backend/internal/branchauth"
	"github.com/example/be_jwt_golang/backend/internal/httpx"
	"github.com/example/be_jwt_golang/backend/internal/pendaftaran"
)

// PagePath is the menu item that grants access to these endpoints
// (seeded by 008_transaksi_klinik_menu.sql).
const PagePath = "/transaksi-klinik/pengobatan"

// MaxPageSize caps one request; exports ask for up to this many rows.
const MaxPageSize = 5000

type Handler struct {
	repo Repository
	now  func() time.Time
}

func NewHandler(repo Repository) *Handler {
	return &Handler{repo: repo, now: time.Now}
}

// Routes registers the endpoints for callers whose role can open PagePath.
func (h *Handler) Routes(mux *http.ServeMux, requireAuth func(http.Handler) http.Handler, requireMenu func(string) func(http.Handler) http.Handler) {
	guard := func(f http.HandlerFunc) http.Handler { return requireAuth(requireMenu(PagePath)(f)) }
	mux.Handle("GET /api/pengobatan", guard(h.List))
	mux.Handle("GET /api/pengobatan/options", guard(h.Options))
	mux.Handle("GET /api/pengobatan/pendaftaran", guard(h.Pendaftaran))
	mux.Handle("GET /api/pengobatan/dokter", guard(h.Dokter))
	mux.Handle("GET /api/pengobatan/obat", guard(h.Obat))
}

func (h *Handler) branches(r *http.Request) ([]branchauth.Branch, error) {
	id, _ := auth.IdentityFrom(r.Context())
	list, err := h.repo.UserBranches(r.Context(), id.UserID)
	if list == nil {
		list = []branchauth.Branch{}
	}
	return list, err
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

// List returns one page of sales at the caller's branches, newest first.
//
// Query: from, to (YYYY-MM-DD, default today in the clinic's time zone),
// page, size (1..5000), q, sort, dir, cabang, nama, pelayanan,
// statusKunjungan, pembayaran, status (open, close, cancel).
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
	from, to := q.Get("from"), q.Get("to")
	if from == "" {
		from = pendaftaran.ClinicToday(h.now())
	}
	if to == "" {
		to = from
	}
	if !validDate(from) || !validDate(to) || to < from {
		httpx.WriteValidation(w, httpx.FieldErrors{"from": "Use a valid date range (YYYY-MM-DD, from ≤ to)"})
		return
	}
	status := strings.ToLower(strings.TrimSpace(q.Get("status")))
	if status != "" && status != StatusOpen && status != StatusClose && status != StatusCancel {
		httpx.WriteValidation(w, httpx.FieldErrors{"status": "Use open, close or cancel"})
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
		Branches: codes, From: from, To: to,
		Search: strings.TrimSpace(q.Get("q")), Nama: strings.TrimSpace(q.Get("nama")),
		IDJenisRawat: optInt(r, "pelayanan"), IDStatusKunjung: optInt(r, "statusKunjungan"),
		IDJenisBayar: optInt(r, "pembayaran"), Status: status,
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

// Options returns the filter choices, the caller's branches and today's clinic date.
func (h *Handler) Options(w http.ResponseWriter, r *http.Request) {
	jenisRawat, err := h.repo.JenisRawat(r.Context())
	if err != nil {
		httpx.InternalError(w, err)
		return
	}
	statusKunjungan, err := h.repo.StatusKunjungan(r.Context())
	if err != nil {
		httpx.InternalError(w, err)
		return
	}
	jenisBayar, err := h.repo.JenisBayar(r.Context())
	if err != nil {
		httpx.InternalError(w, err)
		return
	}
	proses, err := h.repo.ProsesStatus(r.Context())
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
		"pelayanan": jenisRawat, "statusKunjungan": statusKunjungan, "pembayaran": jenisBayar, "proses": proses,
		"branches": branches, "today": pendaftaran.ClinicToday(h.now()),
	})
}

// Pendaftaran pages the registrations of one branch that a treatment can still
// be recorded for (the form's "Data Pendaftaran" lookup), newest first.
//
// Query: cabang (required, one of the caller's branches), from, to (YYYY-MM-DD,
// default yesterday and today in the clinic's time zone), nama, pelayanan,
// status (3 DAFTAR, 4 PROSES or 5 PAYMENT), q, sort, dir, page, size (1..100).
func (h *Handler) Pendaftaran(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	cabang, ok := h.lookupBranch(w, r)
	if !ok {
		return
	}
	now := h.now()
	from, to := q.Get("from"), q.Get("to")
	if to == "" {
		to = pendaftaran.ClinicToday(now)
	}
	if from == "" {
		from = pendaftaran.ClinicToday(now.AddDate(0, 0, -1))
	}
	if !validDate(from) || !validDate(to) || to < from {
		httpx.WriteValidation(w, httpx.FieldErrors{"from": "Use a valid date range (YYYY-MM-DD, from ≤ to)"})
		return
	}
	status := optInt(r, "status")
	if status != nil && !slices.Contains(BillableStatuses, *status) {
		httpx.WriteValidation(w, httpx.FieldErrors{"status": "Use DAFTAR (3), PROSES (4) or PAYMENT (5)"})
		return
	}
	size := intParam(r, "size", 5, 1, 100)
	page := intParam(r, "page", 1, 1, 1_000_000)
	sort := q.Get("sort")
	desc := q.Get("dir") != "asc"
	if _, ok := regSortColumns[sort]; !ok {
		sort, desc = "id", true
	}
	res, err := h.repo.Pendaftaran(r.Context(), RegParams{
		Branch: cabang, From: from, To: to, Nama: strings.TrimSpace(q.Get("nama")),
		IDJenisRawat: optInt(r, "pelayanan"), IDStatus: status, Search: strings.TrimSpace(q.Get("q")),
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

// lookupBranch reads the required cabang of a form lookup and checks that the
// caller is mapped to it. It writes the error response and returns false otherwise.
func (h *Handler) lookupBranch(w http.ResponseWriter, r *http.Request) (string, bool) {
	cabang := strings.ToUpper(strings.TrimSpace(r.URL.Query().Get("cabang")))
	if cabang == "" {
		httpx.WriteValidation(w, httpx.FieldErrors{"cabang": "Choose a branch first"})
		return "", false
	}
	branches, err := h.branches(r)
	if err != nil {
		httpx.InternalError(w, err)
		return "", false
	}
	if !branchauth.Has(branches, cabang) {
		httpx.WriteError(w, http.StatusForbidden, "you are not assigned to branch "+cabang)
		return "", false
	}
	return cabang, true
}

// Dokter lists the active doctors mapped to one of the caller's branches (the
// form's Dokter PJ lookup). Query: cabang (required).
func (h *Handler) Dokter(w http.ResponseWriter, r *http.Request) {
	cabang, ok := h.lookupBranch(w, r)
	if !ok {
		return
	}
	list, err := h.repo.Dokter(r.Context(), cabang)
	if err != nil {
		httpx.InternalError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"rows": list})
}

// Obat lists the drugs sold at one of the caller's branches, with that branch's
// price and stock (the form's "Data Obat" lookup). Query: cabang (required).
func (h *Handler) Obat(w http.ResponseWriter, r *http.Request) {
	cabang, ok := h.lookupBranch(w, r)
	if !ok {
		return
	}
	list, err := h.repo.Obat(r.Context(), cabang)
	if err != nil {
		httpx.InternalError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"rows": list})
}
