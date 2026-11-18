// Package distributor serves Master Data Klinik > Distributor: drug
// distributors (refdistributorobat, created with the Obat tables in 021).
// There is no delete: drugs refer to their distributor.
package distributor

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/example/be_jwt_golang/backend/internal/auth"
	"github.com/example/be_jwt_golang/backend/internal/httpx"
)

// PagePath is the menu item that grants access to these endpoints
// (seeded by 006_master_data_klinik_menu.sql).
const PagePath = "/master-data-klinik/distributor"

var ErrNotFound = errors.New("distributor not found")

// Distributor is a refdistributorobat row.
type Distributor struct {
	ID        int        `json:"id"`
	Nama      string     `json:"nama"`
	Alamat    string     `json:"alamat"`
	Telp      string     `json:"telp"`
	Fax       string     `json:"fax"`
	CP        string     `json:"cp"`     // contact person
	TelpCP    string     `json:"telpCp"` // contact person's phone
	UpdatedBy string     `json:"updatedBy"`
	UpdatedAt *time.Time `json:"updatedAt"`
}

// Input is the editable part of refdistributorobat.
type Input struct {
	Nama   string `json:"nama"`
	Alamat string `json:"alamat"`
	Telp   string `json:"telp"`
	Fax    string `json:"fax"`
	CP     string `json:"cp"`
	TelpCP string `json:"telpCp"`
}

// Normalize trims text and validates against the refdistributorobat column sizes.
func (in *Input) Normalize() httpx.FieldErrors {
	for _, f := range []*string{&in.Nama, &in.Alamat, &in.Telp, &in.Fax, &in.CP, &in.TelpCP} {
		*f = strings.TrimSpace(*f)
	}
	errs := httpx.FieldErrors{}
	maxLen := func(field, label, v string, n int) {
		if _, done := errs[field]; !done && utf8.RuneCountInString(v) > n {
			errs[field] = fmt.Sprintf("%s must be at most %d characters", label, n)
		}
	}
	if in.Nama == "" {
		errs["nama"] = "Nama is required"
	}
	maxLen("nama", "Nama", in.Nama, 300)
	maxLen("alamat", "Alamat", in.Alamat, 200)
	maxLen("telp", "Telp", in.Telp, 15)
	maxLen("fax", "Fax", in.Fax, 25)
	maxLen("cp", "Kontak Person", in.CP, 40)
	maxLen("telpCp", "Telp KP", in.TelpCP, 25)
	return errs
}

type Repository interface {
	List(ctx context.Context) ([]Distributor, error)
	Get(ctx context.Context, id int) (Distributor, error)
	Create(ctx context.Context, in Input, user string) (int, error)
	Update(ctx context.Context, id int, in Input, user string) error
}

type PostgresRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresRepository(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{pool: pool}
}

const columns = `id, COALESCE(distributor, ''), COALESCE(alamat, ''), COALESCE(telp, ''), COALESCE(fax, ''),
	COALESCE(cp, ''), COALESCE(telp_cp, ''), COALESCE(update_user, create_user, ''), COALESCE(update_date, create_date)`

func scan(r pgx.CollectableRow) (Distributor, error) {
	var d Distributor
	err := r.Scan(&d.ID, &d.Nama, &d.Alamat, &d.Telp, &d.Fax, &d.CP, &d.TelpCP, &d.UpdatedBy, &d.UpdatedAt)
	return d, err
}

func (p *PostgresRepository) List(ctx context.Context) ([]Distributor, error) {
	rows, err := p.pool.Query(ctx, `SELECT `+columns+` FROM refdistributorobat ORDER BY id`)
	if err != nil {
		return nil, err
	}
	list, err := pgx.CollectRows(rows, scan)
	if list == nil {
		list = []Distributor{}
	}
	return list, err
}

func (p *PostgresRepository) Get(ctx context.Context, id int) (Distributor, error) {
	rows, err := p.pool.Query(ctx, `SELECT `+columns+` FROM refdistributorobat WHERE id = $1`, id)
	if err != nil {
		return Distributor{}, err
	}
	list, err := pgx.CollectRows(rows, scan)
	if err != nil {
		return Distributor{}, err
	}
	if len(list) == 0 {
		return Distributor{}, ErrNotFound
	}
	return list[0], nil
}

func nullIfEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func (p *PostgresRepository) Create(ctx context.Context, in Input, user string) (int, error) {
	var id int
	err := p.pool.QueryRow(ctx,
		`INSERT INTO refdistributorobat (distributor, alamat, telp, fax, cp, telp_cp, create_user, create_date)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, now()) RETURNING id`,
		in.Nama, nullIfEmpty(in.Alamat), nullIfEmpty(in.Telp), nullIfEmpty(in.Fax), nullIfEmpty(in.CP), nullIfEmpty(in.TelpCP), user,
	).Scan(&id)
	return id, err
}

func (p *PostgresRepository) Update(ctx context.Context, id int, in Input, user string) error {
	tag, err := p.pool.Exec(ctx,
		`UPDATE refdistributorobat SET distributor = $2, alamat = $3, telp = $4, fax = $5, cp = $6, telp_cp = $7,
		        update_user = $8, update_date = now()
		 WHERE id = $1`,
		id, in.Nama, nullIfEmpty(in.Alamat), nullIfEmpty(in.Telp), nullIfEmpty(in.Fax), nullIfEmpty(in.CP), nullIfEmpty(in.TelpCP), user)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// ---------- HTTP ----------

type Handler struct {
	repo Repository
}

func NewHandler(repo Repository) *Handler {
	return &Handler{repo: repo}
}

// Routes registers the distributor endpoints for callers whose role can open PagePath.
func (h *Handler) Routes(mux *http.ServeMux, requireAuth func(http.Handler) http.Handler, requireMenu func(string) func(http.Handler) http.Handler) {
	guard := func(f http.HandlerFunc) http.Handler { return requireAuth(requireMenu(PagePath)(f)) }
	mux.Handle("GET /api/distributor", guard(h.List))
	mux.Handle("POST /api/distributor", guard(h.Create))
	mux.Handle("PUT /api/distributor/{id}", guard(h.Update))
}

func user(r *http.Request) string {
	id, _ := auth.IdentityFrom(r.Context())
	u := []rune(id.Email)
	if len(u) > 20 {
		u = u[:20]
	}
	return string(u)
}

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	list, err := h.repo.List(r.Context())
	if err != nil {
		httpx.InternalError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"distributor": list})
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
		httpx.InternalError(w, err)
		return
	}
	h.respond(w, r, id, http.StatusCreated)
}

func (h *Handler) Update(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil || id <= 0 {
		httpx.WriteError(w, http.StatusNotFound, "distributor not found")
		return
	}
	in, ok := decode(w, r)
	if !ok {
		return
	}
	if err := h.repo.Update(r.Context(), id, in, user(r)); err != nil {
		if errors.Is(err, ErrNotFound) {
			httpx.WriteError(w, http.StatusNotFound, "distributor not found")
		} else {
			httpx.InternalError(w, err)
		}
		return
	}
	h.respond(w, r, id, http.StatusOK)
}

func (h *Handler) respond(w http.ResponseWriter, r *http.Request, id, status int) {
	d, err := h.repo.Get(r.Context(), id)
	if errors.Is(err, ErrNotFound) {
		httpx.WriteError(w, http.StatusNotFound, "distributor not found")
		return
	}
	if err != nil {
		httpx.InternalError(w, err)
		return
	}
	httpx.WriteJSON(w, status, d)
}
