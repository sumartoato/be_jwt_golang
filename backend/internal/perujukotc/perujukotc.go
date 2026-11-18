// Package perujukotc serves Master Data Klinik > Penjual OTC: the people who
// refer over-the-counter sales (refperujukotc). There is no delete: sales
// transactions refer to them.
package perujukotc

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/example/be_jwt_golang/backend/internal/httpx"
)

// PagePath is the menu item that grants access to these endpoints
// (seeded by 006_master_data_klinik_menu.sql).
const PagePath = "/master-data-klinik/penjual-otc"

var ErrNotFound = errors.New("perujuk OTC not found")

// Perujuk is a refperujukotc row.
type Perujuk struct {
	ID   int    `json:"id"`
	Nama string `json:"nama"`
}

// Input is the editable part of refperujukotc.
type Input struct {
	Nama string `json:"nama"`
}

// Normalize trims the name and validates it against the column size (varchar(50)).
func (in *Input) Normalize() httpx.FieldErrors {
	in.Nama = strings.TrimSpace(in.Nama)
	switch {
	case in.Nama == "":
		return httpx.FieldErrors{"nama": "Nama is required"}
	case utf8.RuneCountInString(in.Nama) > 50:
		return httpx.FieldErrors{"nama": "Nama must be at most 50 characters"}
	}
	return nil
}

type Repository interface {
	List(ctx context.Context) ([]Perujuk, error)
	Create(ctx context.Context, in Input) (Perujuk, error)
	Update(ctx context.Context, id int, in Input) (Perujuk, error)
}

type PostgresRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresRepository(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{pool: pool}
}

func (p *PostgresRepository) List(ctx context.Context) ([]Perujuk, error) {
	rows, err := p.pool.Query(ctx, `SELECT id, COALESCE(nama, '') FROM refperujukotc ORDER BY id`)
	if err != nil {
		return nil, err
	}
	list, err := pgx.CollectRows(rows, func(r pgx.CollectableRow) (Perujuk, error) {
		var x Perujuk
		err := r.Scan(&x.ID, &x.Nama)
		return x, err
	})
	if list == nil {
		list = []Perujuk{}
	}
	return list, err
}

func (p *PostgresRepository) Create(ctx context.Context, in Input) (Perujuk, error) {
	x := Perujuk{Nama: in.Nama}
	err := p.pool.QueryRow(ctx, `INSERT INTO refperujukotc (nama) VALUES ($1) RETURNING id`, in.Nama).Scan(&x.ID)
	return x, err
}

func (p *PostgresRepository) Update(ctx context.Context, id int, in Input) (Perujuk, error) {
	tag, err := p.pool.Exec(ctx, `UPDATE refperujukotc SET nama = $2 WHERE id = $1`, id, in.Nama)
	if err != nil {
		return Perujuk{}, err
	}
	if tag.RowsAffected() == 0 {
		return Perujuk{}, ErrNotFound
	}
	return Perujuk{ID: id, Nama: in.Nama}, nil
}

// ---------- HTTP ----------

type Handler struct {
	repo Repository
}

func NewHandler(repo Repository) *Handler {
	return &Handler{repo: repo}
}

// Routes registers the endpoints for callers whose role can open PagePath.
func (h *Handler) Routes(mux *http.ServeMux, requireAuth func(http.Handler) http.Handler, requireMenu func(string) func(http.Handler) http.Handler) {
	guard := func(f http.HandlerFunc) http.Handler { return requireAuth(requireMenu(PagePath)(f)) }
	mux.Handle("GET /api/perujuk-otc", guard(h.List))
	mux.Handle("POST /api/perujuk-otc", guard(h.Create))
	mux.Handle("PUT /api/perujuk-otc/{id}", guard(h.Update))
}

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	list, err := h.repo.List(r.Context())
	if err != nil {
		httpx.InternalError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"perujuk": list})
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
	x, err := h.repo.Create(r.Context(), in)
	if err != nil {
		httpx.InternalError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, x)
}

func (h *Handler) Update(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil || id <= 0 {
		httpx.WriteError(w, http.StatusNotFound, "perujuk OTC not found")
		return
	}
	in, ok := decode(w, r)
	if !ok {
		return
	}
	x, err := h.repo.Update(r.Context(), id, in)
	if errors.Is(err, ErrNotFound) {
		httpx.WriteError(w, http.StatusNotFound, "perujuk OTC not found")
		return
	}
	if err != nil {
		httpx.InternalError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, x)
}
