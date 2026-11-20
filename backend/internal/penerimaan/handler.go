package penerimaan

import (
	"errors"
	"net/http"
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
const PagePath = "/transaksi-klinik/penerimaan-barang"

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
	mux.Handle("GET /api/penerimaan-barang", guard(h.List))
	mux.Handle("GET /api/penerimaan-barang/options", guard(h.Options))
	mux.Handle("GET /api/penerimaan-barang/distributor", guard(h.Distributors))
	mux.Handle("GET /api/penerimaan-barang/obat", guard(h.Obat))
	mux.Handle("POST /api/penerimaan-barang", guard(h.Save))
	mux.Handle("GET /api/penerimaan-barang/detail/{id}", guard(h.Detail))
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

func validDate(s string) bool {
	_, err := time.Parse("2006-01-02", s)
	return err == nil
}

// List returns one page of goods receipts at the caller's branches, newest first.
//
// Query: from, to (YYYY-MM-DD of the entry date, default today), cabang, page,
// size (1..5000), q, sort (id, noGr, tglTrans, tglGr, supplier, total, cabang),
// dir, noGr, supplier (refsupplier id; 0 = none).
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
	if len(codes) == 0 {
		httpx.WriteJSON(w, http.StatusOK, map[string]any{"rows": []Row{}, "total": 0, "unfilteredTotal": 0, "from": from, "to": to})
		return
	}
	var supplier *int
	if v, err := strconv.Atoi(q.Get("supplier")); err == nil && v >= 0 {
		supplier = &v
	}
	size := intParam(r, "size", 10, 1, MaxPageSize)
	page := intParam(r, "page", 1, 1, 1_000_000)
	sort := q.Get("sort")
	desc := q.Get("dir") != "asc"
	if !Sortable(sort) {
		sort, desc = "id", true // newest first, like the legacy list
	}
	res, err := h.repo.List(r.Context(), ListParams{
		Branches: codes, From: from, To: to,
		Search: strings.TrimSpace(q.Get("q")), NoGR: strings.TrimSpace(q.Get("noGr")), IDSupplier: supplier,
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

// Options returns the suppliers, the caller's branches and today's clinic date.
func (h *Handler) Options(w http.ResponseWriter, r *http.Request) {
	suppliers, err := h.repo.Suppliers(r.Context())
	if err != nil {
		httpx.InternalError(w, err)
		return
	}
	branches, err := h.branches(r)
	if err != nil {
		httpx.InternalError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"suppliers": suppliers, "branches": branches, "today": pendaftaran.ClinicToday(h.now())})
}

// Distributors lists the distributors for the form's "Data Distributor" lookup.
func (h *Handler) Distributors(w http.ResponseWriter, r *http.Request) {
	list, err := h.repo.Distributors(r.Context())
	if err != nil {
		httpx.InternalError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"rows": list})
}

// Obat lists the drugs stocked at one of the caller's branches for the
// "Data Obat" lookup. Query: cabang (required).
func (h *Handler) Obat(w http.ResponseWriter, r *http.Request) {
	c := strings.ToUpper(strings.TrimSpace(r.URL.Query().Get("cabang")))
	if c == "" {
		httpx.WriteValidation(w, httpx.FieldErrors{"cabang": "Pilih Cabang"})
		return
	}
	branches, err := h.branches(r)
	if err != nil {
		httpx.InternalError(w, err)
		return
	}
	if !branchauth.Has(branches, c) {
		httpx.WriteError(w, http.StatusForbidden, "you are not assigned to branch "+c)
		return
	}
	list, err := h.repo.Obat(r.Context(), c)
	if err != nil {
		httpx.InternalError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"rows": list})
}

// Save records a goods receipt at one of the caller's branches and adds its
// drugs to that branch's stock; answers 201 with the new No Trans and total.
func (h *Handler) Save(w http.ResponseWriter, r *http.Request) {
	var in SaveInput
	if !httpx.DecodeJSON(w, r, &in) {
		return
	}
	in.Normalize()
	if errs := in.Validate(pendaftaran.ClinicToday(h.now())); len(errs) > 0 {
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
	id, _ := auth.IdentityFrom(r.Context())
	user := id.Email
	if at := strings.IndexByte(user, '@'); at > 0 {
		user = user[:at] // like the legacy user names ("khoza")
	}
	if u := []rune(user); len(u) > 20 { // create_user is varchar(20)
		user = string(u[:20])
	}
	res, err := h.repo.Save(r.Context(), in, user)
	var item *ItemError
	switch {
	case err == nil:
		httpx.WriteJSON(w, http.StatusCreated, res)
	case errors.Is(err, ErrUnknownDistributor):
		httpx.WriteValidation(w, httpx.FieldErrors{"idDistributor": "Distributor tidak dikenal"})
	case errors.As(err, &item):
		httpx.WriteValidation(w, httpx.FieldErrors{"obat": item.Error()})
	default:
		httpx.InternalError(w, err)
	}
}

// Detail returns a saved receipt with its lines for the View page; a receipt
// of another branch, or one that does not exist, answers 404.
func (h *Handler) Detail(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil || id <= 0 {
		httpx.WriteError(w, http.StatusNotFound, ErrNotFound.Error())
		return
	}
	d, err := h.repo.Detail(r.Context(), id)
	if errors.Is(err, ErrNotFound) {
		httpx.WriteError(w, http.StatusNotFound, err.Error())
		return
	}
	if err != nil {
		httpx.InternalError(w, err)
		return
	}
	branches, err := h.branches(r)
	if err != nil {
		httpx.InternalError(w, err)
		return
	}
	if !branchauth.Has(branches, d.Cabang) {
		httpx.WriteError(w, http.StatusNotFound, ErrNotFound.Error())
		return
	}
	httpx.WriteJSON(w, http.StatusOK, d)
}
