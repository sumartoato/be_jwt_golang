// Package jamkes serves Master Data Klinik > Jaminan Kesehatan: health
// insurers (refjaminankesehatan), their cooperation-agreement (PKS) terms
// (refjaminankesehatan_detail) and their numbered item lists
// (refjaminankesehatan_detail_item).
package jamkes

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/example/be_jwt_golang/backend/internal/httpx"
)

var (
	ErrNotFound     = errors.New("jaminan kesehatan not found")
	ErrUnknownType  = errors.New("unknown jenis")
	ErrUnknownTerm  = errors.New("unknown PKS term")
	ErrDuplicateKey = errors.New("duplicate PKS term")
)

// Jamkes is a refjaminankesehatan row.
type Jamkes struct {
	ID        int        `json:"id"`
	Nama      string     `json:"nama"`
	Alamat    string     `json:"alamat"`
	Telp      string     `json:"telp"`
	Fax       string     `json:"fax"`
	IDType    *int       `json:"idType"`
	IsActive  bool       `json:"isActive"` // isactive 1
	HasLimit  bool       `json:"hasLimit"` // status_limit 1 (2 = No Limit)
	Limit     *float64   `json:"limit"`
	UpdatedBy string     `json:"updatedBy"`
	UpdatedAt *time.Time `json:"updatedAt"`
}

// Input is the editable part of refjaminankesehatan.
type Input struct {
	Nama     string   `json:"nama"`
	Alamat   string   `json:"alamat"`
	Telp     string   `json:"telp"`
	Fax      string   `json:"fax"`
	IDType   *int     `json:"idType"`
	IsActive bool     `json:"isActive"`
	HasLimit bool     `json:"hasLimit"`
	Limit    *float64 `json:"limit"`
}

// Normalize trims text and validates against the refjaminankesehatan column sizes.
func (in *Input) Normalize() httpx.FieldErrors {
	for _, f := range []*string{&in.Nama, &in.Alamat, &in.Telp, &in.Fax} {
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
	maxLen("nama", "Nama", in.Nama, 100)
	maxLen("alamat", "Alamat", in.Alamat, 100)
	maxLen("telp", "Telp", in.Telp, 15)
	maxLen("fax", "Fax", in.Fax, 15)
	if in.Limit != nil && (math.IsNaN(*in.Limit) || math.IsInf(*in.Limit, 0) || *in.Limit < 0 || *in.Limit > 1e13) {
		errs["limit"] = "Limit must be a number from 0"
	}
	if in.HasLimit && (in.Limit == nil || *in.Limit <= 0) {
		if _, set := errs["limit"]; !set {
			errs["limit"] = "Enter the limit, or choose No Limit"
		}
	}
	return errs
}

// Term is one PKS term of an insurer. Every term of the master list is returned;
// Value is "" and Saved false when the insurer has none.
type Term struct {
	DetailID    int    `json:"detailId"`
	Description string `json:"description"`
	Value       string `json:"value"`
	Shown       bool   `json:"shown"` // flag 1
	Saved       bool   `json:"saved"`
}

// TermInput sets the text of one term; an empty value removes it.
type TermInput struct {
	DetailID int    `json:"detailId"`
	Value    string `json:"value"`
	Shown    bool   `json:"shown"`
}

// Item is one entry of an insurer's numbered list.
type Item struct {
	No    int    `json:"no"` // detailid
	Value string `json:"value"`
	Shown bool   `json:"shown"`
}

// ItemInput is one entry of the list; the numbers follow the order sent.
type ItemInput struct {
	Value string `json:"value"`
	Shown bool   `json:"shown"`
}

// Option is an entry of a lookup table.
type Option struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

type Repository interface {
	List(ctx context.Context) ([]Jamkes, error)
	Types(ctx context.Context) ([]Option, error)
	Get(ctx context.Context, id int) (Jamkes, error)
	Create(ctx context.Context, in Input, user string) (int, error)
	Update(ctx context.Context, id int, in Input, user string) error
	Terms(ctx context.Context, id int) ([]Term, error)
	SaveTerms(ctx context.Context, id int, terms []TermInput) error
	Items(ctx context.Context, id int) ([]Item, error)
	SaveItems(ctx context.Context, id int, items []ItemInput) error
}

type PostgresRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresRepository(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{pool: pool}
}

const columns = `id, nama, COALESCE(alamat, ''), COALESCE(telp, ''), COALESCE(fax, ''), jamkestype,
	COALESCE(isactive, 1) = 1, COALESCE(status_limit, 2) = 1, "limit",
	COALESCE(update_user, create_user, ''), COALESCE(update_date, create_date)`

func scan(r pgx.CollectableRow) (Jamkes, error) {
	var j Jamkes
	err := r.Scan(&j.ID, &j.Nama, &j.Alamat, &j.Telp, &j.Fax, &j.IDType, &j.IsActive, &j.HasLimit, &j.Limit, &j.UpdatedBy, &j.UpdatedAt)
	return j, err
}

func (p *PostgresRepository) List(ctx context.Context) ([]Jamkes, error) {
	rows, err := p.pool.Query(ctx, `SELECT `+columns+` FROM refjaminankesehatan ORDER BY id`)
	if err != nil {
		return nil, err
	}
	list, err := pgx.CollectRows(rows, scan)
	if list == nil {
		list = []Jamkes{}
	}
	return list, err
}

func (p *PostgresRepository) Types(ctx context.Context) ([]Option, error) {
	rows, err := p.pool.Query(ctx, `SELECT id, COALESCE(nama, '') FROM refjaminankesehatantype ORDER BY id`)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(r pgx.CollectableRow) (Option, error) {
		var o Option
		err := r.Scan(&o.ID, &o.Name)
		return o, err
	})
}

func (p *PostgresRepository) Get(ctx context.Context, id int) (Jamkes, error) {
	rows, err := p.pool.Query(ctx, `SELECT `+columns+` FROM refjaminankesehatan WHERE id = $1`, id)
	if err != nil {
		return Jamkes{}, err
	}
	list, err := pgx.CollectRows(rows, scan)
	if err != nil {
		return Jamkes{}, err
	}
	if len(list) == 0 {
		return Jamkes{}, ErrNotFound
	}
	return list[0], nil
}

func nullIfEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func flags(in Input) (isactive, statusLimit int, limit float64) {
	isactive, statusLimit = 2, 2
	if in.IsActive {
		isactive = 1
	}
	if in.HasLimit {
		statusLimit = 1
	}
	if in.HasLimit && in.Limit != nil {
		limit = *in.Limit
	}
	return
}

func (p *PostgresRepository) Create(ctx context.Context, in Input, user string) (int, error) {
	active, statusLimit, limit := flags(in)
	var id int
	err := p.pool.QueryRow(ctx,
		`INSERT INTO refjaminankesehatan (nama, alamat, telp, fax, jamkestype, isactive, status_limit, "limit", create_user, create_date)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, now()) RETURNING id`,
		in.Nama, nullIfEmpty(in.Alamat), nullIfEmpty(in.Telp), nullIfEmpty(in.Fax), in.IDType, active, statusLimit, limit, user,
	).Scan(&id)
	return id, translate(err)
}

func (p *PostgresRepository) Update(ctx context.Context, id int, in Input, user string) error {
	active, statusLimit, limit := flags(in)
	tag, err := p.pool.Exec(ctx,
		`UPDATE refjaminankesehatan SET nama = $2, alamat = $3, telp = $4, fax = $5, jamkestype = $6, isactive = $7,
		        status_limit = $8, "limit" = $9, update_user = $10, update_date = now()
		 WHERE id = $1`,
		id, in.Nama, nullIfEmpty(in.Alamat), nullIfEmpty(in.Telp), nullIfEmpty(in.Fax), in.IDType, active, statusLimit, limit, user)
	if err != nil {
		return translate(err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (p *PostgresRepository) Terms(ctx context.Context, id int) ([]Term, error) {
	rows, err := p.pool.Query(ctx,
		`SELECT m.id, COALESCE(m.description, ''), COALESCE(d.value, ''), COALESCE(d.flag, 1) = 1, d.id IS NOT NULL
		 FROM refjaminankesehatan_pksmaster m
		 LEFT JOIN refjaminankesehatan_detail d ON d.detailid = m.id AND d.jamkesid = $1
		 ORDER BY m.id`, id)
	if err != nil {
		return nil, err
	}
	list, err := pgx.CollectRows(rows, func(r pgx.CollectableRow) (Term, error) {
		var t Term
		err := r.Scan(&t.DetailID, &t.Description, &t.Value, &t.Shown, &t.Saved)
		return t, err
	})
	if list == nil {
		list = []Term{}
	}
	return list, err
}

// SaveTerms replaces the insurer's terms; terms with an empty value are removed.
func (p *PostgresRepository) SaveTerms(ctx context.Context, id int, terms []TermInput) error {
	err := pgx.BeginFunc(ctx, p.pool, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `DELETE FROM refjaminankesehatan_detail WHERE jamkesid = $1`, id); err != nil {
			return err
		}
		for _, t := range terms {
			if t.Value == "" {
				continue
			}
			if _, err := tx.Exec(ctx,
				`INSERT INTO refjaminankesehatan_detail (jamkesid, detailid, value, flag) VALUES ($1, $2, $3, $4)`,
				id, t.DetailID, t.Value, shownFlag(t.Shown)); err != nil {
				return err
			}
		}
		return nil
	})
	return translate(err)
}

func shownFlag(shown bool) int {
	if shown {
		return 1
	}
	return 0
}

func (p *PostgresRepository) Items(ctx context.Context, id int) ([]Item, error) {
	rows, err := p.pool.Query(ctx,
		`SELECT COALESCE(detailid, 0), COALESCE(value, ''), COALESCE(flag, 1) = 1
		 FROM refjaminankesehatan_detail_item WHERE jamkesid = $1 ORDER BY detailid, id`, id)
	if err != nil {
		return nil, err
	}
	list, err := pgx.CollectRows(rows, func(r pgx.CollectableRow) (Item, error) {
		var it Item
		err := r.Scan(&it.No, &it.Value, &it.Shown)
		return it, err
	})
	if list == nil {
		list = []Item{}
	}
	return list, err
}

// SaveItems replaces the insurer's list, numbering the items 1, 2, 3, ... in order.
func (p *PostgresRepository) SaveItems(ctx context.Context, id int, items []ItemInput) error {
	err := pgx.BeginFunc(ctx, p.pool, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `DELETE FROM refjaminankesehatan_detail_item WHERE jamkesid = $1`, id); err != nil {
			return err
		}
		for i, it := range items {
			if _, err := tx.Exec(ctx,
				`INSERT INTO refjaminankesehatan_detail_item (jamkesid, detailid, value, flag) VALUES ($1, $2, $3, $4)`,
				id, i+1, it.Value, shownFlag(it.Shown)); err != nil {
				return err
			}
		}
		return nil
	})
	return translate(err)
}

func translate(err error) error {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		return err
	}
	switch pgErr.ConstraintName {
	case "fk_refjaminankesehatan_type":
		return ErrUnknownType
	case "fk_refjaminankesehatan_detail_term":
		return ErrUnknownTerm
	case "refjaminankesehatan_detail_jamkes_term_key":
		return ErrDuplicateKey
	case "fk_refjaminankesehatan_detail_jamkes", "fk_refjaminankesehatan_detail_item_jamkes":
		return ErrNotFound
	}
	return err
}
