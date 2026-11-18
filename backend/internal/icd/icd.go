// Package icd serves Master Data Klinik > ICD: ICD-X diagnosis codes (reficd)
// and their types (refjenisicd). The ~18.5k codes are paged, searched and
// sorted in the database.
package icd

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/example/be_jwt_golang/backend/internal/httpx"
)

var (
	ErrNotFound     = errors.New("icd not found")
	ErrDuplicate    = errors.New("kode ICD already exists")
	ErrUnknownJenis = errors.New("unknown jenis ICD")
)

// ICD is a reficd row.
type ICD struct {
	ID        int        `json:"id"`
	Kode      string     `json:"kode"`
	Nama      string     `json:"nama"`
	IDJenis   int        `json:"idJenis"`
	Jenis     string     `json:"jenis"`
	UpdatedBy string     `json:"updatedBy"`
	UpdatedAt *time.Time `json:"updatedAt"`
}

// Input is the editable part of reficd.
type Input struct {
	Kode    string `json:"kode"`
	Nama    string `json:"nama"`
	IDJenis *int   `json:"idJenis"`
}

// Normalize trims text, uppercases the code and validates against the reficd column sizes.
func (in *Input) Normalize() httpx.FieldErrors {
	in.Kode = strings.ToUpper(strings.TrimSpace(in.Kode))
	in.Nama = strings.TrimSpace(in.Nama)
	errs := httpx.FieldErrors{}
	switch {
	case in.Kode == "":
		errs["kode"] = "Kode ICD-X is required"
	case utf8.RuneCountInString(in.Kode) > 10:
		errs["kode"] = "Kode ICD-X must be at most 10 characters"
	}
	switch {
	case in.Nama == "":
		errs["nama"] = "Nama Penyakit is required"
	case utf8.RuneCountInString(in.Nama) > 250:
		errs["nama"] = "Nama Penyakit must be at most 250 characters"
	}
	if in.IDJenis == nil {
		errs["idJenis"] = "Jenis ICD-X is required"
	}
	return errs
}

// sortColumns are the sortable list columns by API key (SQL is fixed, never from the request).
var sortColumns = map[string]string{
	"id":    "i.id",
	"kode":  "i.kode_icd",
	"nama":  "i.nama",
	"jenis": "j.info",
}

// ListParams selects one page of codes.
type ListParams struct {
	Search string
	Sort   string
	Desc   bool
	Limit  int
	Offset int
}

type Page struct {
	Rows            []ICD `json:"rows"`
	Total           int   `json:"total"`
	UnfilteredTotal int   `json:"unfilteredTotal"`
}

// Option is an entry of refjenisicd.
type Option struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

type Repository interface {
	List(ctx context.Context, p ListParams) (Page, error)
	Jenis(ctx context.Context) ([]Option, error)
	Get(ctx context.Context, id int) (ICD, error)
	Create(ctx context.Context, in Input, user string) (int, error)
	Update(ctx context.Context, id int, in Input, user string) error
}

type PostgresRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresRepository(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{pool: pool}
}

func escapeLike(s string) string {
	return strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(s)
}

const fromJoin = ` FROM reficd i LEFT JOIN refjenisicd j ON j.id = i.id_jenis_icd`
const selectCols = `SELECT i.id, COALESCE(i.kode_icd, ''), COALESCE(i.nama, ''), i.id_jenis_icd, COALESCE(j.info, ''),
	COALESCE(i.update_user, i.create_user, ''), COALESCE(i.update_date, i.create_date)`

func scan(r pgx.CollectableRow) (ICD, error) {
	var x ICD
	err := r.Scan(&x.ID, &x.Kode, &x.Nama, &x.IDJenis, &x.Jenis, &x.UpdatedBy, &x.UpdatedAt)
	return x, err
}

func (p *PostgresRepository) List(ctx context.Context, lp ListParams) (Page, error) {
	page := Page{Rows: []ICD{}}
	col, ok := sortColumns[lp.Sort]
	if !ok {
		col = "i.id"
	}
	dir := "ASC"
	if lp.Desc {
		dir = "DESC"
	}
	where, args := "", []any{}
	if lp.Search != "" {
		args = append(args, "%"+escapeLike(lp.Search)+"%")
		where = ` WHERE i.kode_icd ILIKE $1 OR i.nama ILIKE $1 OR j.info ILIKE $1`
	}
	q := fmt.Sprintf(`%s%s%s ORDER BY %s %s NULLS LAST, i.id %s LIMIT $%d OFFSET $%d`,
		selectCols, fromJoin, where, col, dir, dir, len(args)+1, len(args)+2)
	rows, err := p.pool.Query(ctx, q, append(args, lp.Limit, lp.Offset)...)
	if err != nil {
		return page, err
	}
	if page.Rows, err = pgx.CollectRows(rows, scan); err != nil {
		return page, err
	}
	if page.Rows == nil {
		page.Rows = []ICD{}
	}
	if err := p.pool.QueryRow(ctx, `SELECT count(*)`+fromJoin+where, args...).Scan(&page.Total); err != nil {
		return page, err
	}
	if lp.Search == "" {
		page.UnfilteredTotal = page.Total
		return page, nil
	}
	err = p.pool.QueryRow(ctx, `SELECT count(*) FROM reficd`).Scan(&page.UnfilteredTotal)
	return page, err
}

func (p *PostgresRepository) Jenis(ctx context.Context) ([]Option, error) {
	rows, err := p.pool.Query(ctx, `SELECT id, COALESCE(info, '') FROM refjenisicd ORDER BY id`)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(r pgx.CollectableRow) (Option, error) {
		var o Option
		err := r.Scan(&o.ID, &o.Name)
		return o, err
	})
}

func (p *PostgresRepository) Get(ctx context.Context, id int) (ICD, error) {
	rows, err := p.pool.Query(ctx, selectCols+fromJoin+` WHERE i.id = $1`, id)
	if err != nil {
		return ICD{}, err
	}
	list, err := pgx.CollectRows(rows, scan)
	if err != nil {
		return ICD{}, err
	}
	if len(list) == 0 {
		return ICD{}, ErrNotFound
	}
	return list[0], nil
}

func (p *PostgresRepository) Create(ctx context.Context, in Input, user string) (int, error) {
	var id int
	err := p.pool.QueryRow(ctx,
		`INSERT INTO reficd (kode_icd, nama, id_jenis_icd, create_user, create_date) VALUES ($1, $2, $3, $4, now()) RETURNING id`,
		in.Kode, in.Nama, in.IDJenis, user).Scan(&id)
	return id, translate(err)
}

func (p *PostgresRepository) Update(ctx context.Context, id int, in Input, user string) error {
	tag, err := p.pool.Exec(ctx,
		`UPDATE reficd SET kode_icd = $2, nama = $3, id_jenis_icd = $4, update_user = $5, update_date = now() WHERE id = $1`,
		id, in.Kode, in.Nama, in.IDJenis, user)
	if err != nil {
		return translate(err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func translate(err error) error {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		return err
	}
	switch pgErr.ConstraintName {
	case "reficd_kode_icd_key":
		return ErrDuplicate
	case "fk_reficd_jenis":
		return ErrUnknownJenis
	}
	return err
}
