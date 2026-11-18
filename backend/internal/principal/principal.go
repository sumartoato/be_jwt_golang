// Package principal serves Master Data Klinik > Principal: drug principals
// (refprincipalobat, created with the Obat tables in 021). There is no delete:
// drugs refer to their principal.
package principal

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
const PagePath = "/master-data-klinik/principal"

var ErrNotFound = errors.New("principal not found")

// Principal is a refprincipalobat row.
type Principal struct {
	ID   int    `json:"id"`
	Nama string `json:"nama"`
}

// Input is the editable part of refprincipalobat.
type Input struct {
	Nama string `json:"nama"`
}

// Normalize trims the name and validates it against the column size (varchar(300)).
func (in *Input) Normalize() httpx.FieldErrors {
	in.Nama = strings.TrimSpace(in.Nama)
	switch {
	case in.Nama == "":
		return httpx.FieldErrors{"nama": "Nama is required"}
	case utf8.RuneCountInString(in.Nama) > 300:
		return httpx.FieldErrors{"nama": "Nama must be at most 300 characters"}
	}
	return nil
}

type Repository interface {
	List(ctx context.Context) ([]Principal, error)
	Create(ctx context.Context, in Input) (Principal, error)
	Update(ctx context.Context, id int, in Input) (Principal, error)
}

type PostgresRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresRepository(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{pool: pool}
}

func (p *PostgresRepository) List(ctx context.Context) ([]Principal, error) {
	rows, err := p.pool.Query(ctx, `SELECT id, COALESCE(principal, '') FROM refprincipalobat ORDER BY id`)
	if err != nil {
		return nil, err
	}
	list, err := pgx.CollectRows(rows, func(r pgx.CollectableRow) (Principal, error) {
		var x Principal
		err := r.Scan(&x.ID, &x.Nama)
		return x, err
	})
	if list == nil {
		list = []Principal{}
	}
	return list, err
}

func (p *PostgresRepository) Create(ctx context.Context, in Input) (Principal, error) {
	x := Principal{Nama: in.Nama}
	err := p.pool.QueryRow(ctx, `INSERT INTO refprincipalobat (principal) VALUES ($1) RETURNING id`, in.Nama).Scan(&x.ID)
	return x, err
}

func (p *PostgresRepository) Update(ctx context.Context, id int, in Input) (Principal, error) {
	tag, err := p.pool.Exec(ctx, `UPDATE refprincipalobat SET principal = $2 WHERE id = $1`, id, in.Nama)
	if err != nil {
		return Principal{}, err
	}
	if tag.RowsAffected() == 0 {
		return Principal{}, ErrNotFound
	}
	return Principal{ID: id, Nama: in.Nama}, nil
}

// ---------- HTTP ----------

type Handler struct {
	repo Repository
}

func NewHandler(repo Repository) *Handler {
	return &Handler{repo: repo}
}

// Routes registers the principal endpoints for callers whose role can open PagePath.
func (h *Handler) Routes(mux *http.ServeMux, requireAuth func(http.Handler) http.Handler, requireMenu func(string) func(http.Handler) http.Handler) {
	guard := func(f http.HandlerFunc) http.Handler { return requireAuth(requireMenu(PagePath)(f)) }
	mux.Handle("GET /api/principal", guard(h.List))
	mux.Handle("POST /api/principal", guard(h.Create))
	mux.Handle("PUT /api/principal/{id}", guard(h.Update))
}

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	list, err := h.repo.List(r.Context())
	if err != nil {
		httpx.InternalError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"principal": list})
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
		httpx.WriteError(w, http.StatusNotFound, "principal not found")
		return
	}
	in, ok := decode(w, r)
	if !ok {
		return
	}
	x, err := h.repo.Update(r.Context(), id, in)
	if errors.Is(err, ErrNotFound) {
		httpx.WriteError(w, http.StatusNotFound, "principal not found")
		return
	}
	if err != nil {
		httpx.InternalError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, x)
}
