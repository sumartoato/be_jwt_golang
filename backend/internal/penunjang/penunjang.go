// Package penunjang serves Master Data Klinik > Penunjang: supporting
// examinations (refpenunjang) and their prices per branch (refpenunjangdetail).
// A user only sees and edits the prices of the branches they are mapped to.
package penunjang

import (
	"context"
	"errors"
	"math"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/example/be_jwt_golang/backend/internal/branchauth"
	"github.com/example/be_jwt_golang/backend/internal/httpx"
)

var (
	ErrNotFound     = errors.New("penunjang not found")
	ErrUnknownJenis = errors.New("unknown jenis penunjang")
)

// Price is one refpenunjangdetail row: an examination's prices at one branch.
// The lab tables have no purchase price for examinations.
type Price struct {
	Cabang      string   `json:"cabang"`
	HJual       *float64 `json:"hjual"`
	HJualJamkes *float64 `json:"hjualJamkes"`
	IsActive    bool     `json:"isActive"`
}

// Penunjang is a refpenunjang row plus its prices at the caller's branches.
type Penunjang struct {
	ID          int        `json:"id"`
	Nama        string     `json:"nama"`
	IDJenis     *int       `json:"idJenis"`
	HJual       *float64   `json:"hjual"`
	HJualJamkes *float64   `json:"hjualJamkes"`
	IsActive    bool       `json:"isActive"`
	UpdatedBy   string     `json:"updatedBy"`
	UpdatedAt   *time.Time `json:"updatedAt"`
	Prices      []Price    `json:"prices"`
}

// Input is the editable part of refpenunjang.
type Input struct {
	Nama        string   `json:"nama"`
	IDJenis     *int     `json:"idJenis"`
	HJual       *float64 `json:"hjual"`
	HJualJamkes *float64 `json:"hjualJamkes"`
	IsActive    bool     `json:"isActive"`
}

// PriceInput is the editable part of a refpenunjangdetail row.
type PriceInput struct {
	HJual       *float64 `json:"hjual"`
	HJualJamkes *float64 `json:"hjualJamkes"`
	IsActive    bool     `json:"isActive"`
}

func checkAmounts(errs httpx.FieldErrors, fields map[string]*float64) {
	for name, v := range fields {
		if v != nil && (math.IsNaN(*v) || math.IsInf(*v, 0) || *v < 0 || *v > 1e12) {
			errs[name] = "Must be a number from 0"
		}
	}
}

// Normalize trims the name and validates against the refpenunjang column sizes.
func (in *Input) Normalize() httpx.FieldErrors {
	in.Nama = strings.TrimSpace(in.Nama)
	errs := httpx.FieldErrors{}
	if in.Nama == "" {
		errs["nama"] = "Nama Penunjang is required"
	} else if utf8.RuneCountInString(in.Nama) > 100 {
		errs["nama"] = "Nama Penunjang must be at most 100 characters"
	}
	checkAmounts(errs, map[string]*float64{"hjual": in.HJual, "hjualJamkes": in.HJualJamkes})
	return errs
}

func (in *PriceInput) Validate() httpx.FieldErrors {
	errs := httpx.FieldErrors{}
	checkAmounts(errs, map[string]*float64{"hjual": in.HJual, "hjualJamkes": in.HJualJamkes})
	return errs
}

// Option is an entry of refjenispanjang (the examination types).
type Option struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

type Repository interface {
	Jenis(ctx context.Context) ([]Option, error)
	UserBranches(ctx context.Context, userID string) ([]branchauth.Branch, error)
	// List returns every examination with prices at the given branches only.
	List(ctx context.Context, branches []string) ([]Penunjang, error)
	Get(ctx context.Context, id int, branches []string) (Penunjang, error)
	Create(ctx context.Context, in Input, user string) (int, error)
	Update(ctx context.Context, id int, in Input, user string) error
	UpsertPrice(ctx context.Context, id int, cabang string, p PriceInput) error
}

type PostgresRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresRepository(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{pool: pool}
}

func (p *PostgresRepository) Jenis(ctx context.Context) ([]Option, error) {
	rows, err := p.pool.Query(ctx, `SELECT id, COALESCE(trim(info), '') FROM refjenispanjang ORDER BY info, id`)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(r pgx.CollectableRow) (Option, error) {
		var o Option
		err := r.Scan(&o.ID, &o.Name)
		return o, err
	})
}

func (p *PostgresRepository) UserBranches(ctx context.Context, userID string) ([]branchauth.Branch, error) {
	return branchauth.ForUser(ctx, p.pool, userID)
}

const columns = `id, nama, id_jenis, hjual, hjual_jamkes, COALESCE(isactive, 0) = 1,
	COALESCE(update_user, create_user, ''), COALESCE(update_date, create_date)`

func scan(r pgx.CollectableRow) (Penunjang, error) {
	var x Penunjang
	err := r.Scan(&x.ID, &x.Nama, &x.IDJenis, &x.HJual, &x.HJualJamkes, &x.IsActive, &x.UpdatedBy, &x.UpdatedAt)
	x.Prices = []Price{}
	return x, err
}

// attachPrices fills Prices for the examinations in list at branches.
func (p *PostgresRepository) attachPrices(ctx context.Context, list []Penunjang, branches []string, only *int) error {
	if len(list) == 0 || len(branches) == 0 {
		return nil
	}
	index := make(map[int]int, len(list))
	for i, x := range list {
		index[x.ID] = i
	}
	rows, err := p.pool.Query(ctx,
		`SELECT id_penunjang, cabang, hjual, hjual_jamkes, isactive = 1
		 FROM refpenunjangdetail
		 WHERE cabang = ANY($1) AND ($2::int IS NULL OR id_penunjang = $2)
		 ORDER BY cabang`, branches, only)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var (
			id int
			pr Price
		)
		if err := rows.Scan(&id, &pr.Cabang, &pr.HJual, &pr.HJualJamkes, &pr.IsActive); err != nil {
			return err
		}
		if i, ok := index[id]; ok {
			list[i].Prices = append(list[i].Prices, pr)
		}
	}
	return rows.Err()
}

func (p *PostgresRepository) List(ctx context.Context, branches []string) ([]Penunjang, error) {
	rows, err := p.pool.Query(ctx, `SELECT `+columns+` FROM refpenunjang ORDER BY id`)
	if err != nil {
		return nil, err
	}
	list, err := pgx.CollectRows(rows, scan)
	if err != nil {
		return nil, err
	}
	if list == nil {
		list = []Penunjang{}
	}
	return list, p.attachPrices(ctx, list, branches, nil)
}

func (p *PostgresRepository) Get(ctx context.Context, id int, branches []string) (Penunjang, error) {
	rows, err := p.pool.Query(ctx, `SELECT `+columns+` FROM refpenunjang WHERE id = $1`, id)
	if err != nil {
		return Penunjang{}, err
	}
	list, err := pgx.CollectRows(rows, scan)
	if err != nil {
		return Penunjang{}, err
	}
	if len(list) == 0 {
		return Penunjang{}, ErrNotFound
	}
	return list[0], p.attachPrices(ctx, list, branches, &id)
}

func flag(v bool) int {
	if v {
		return 1
	}
	return 0
}

func (p *PostgresRepository) Create(ctx context.Context, in Input, user string) (int, error) {
	var id int
	err := p.pool.QueryRow(ctx,
		`INSERT INTO refpenunjang (nama, id_jenis, hjual, hjual_jamkes, isactive, create_user, create_date)
		 VALUES ($1, $2, $3, $4, $5, $6, now())
		 RETURNING id`,
		in.Nama, in.IDJenis, in.HJual, in.HJualJamkes, flag(in.IsActive), user,
	).Scan(&id)
	return id, translate(err)
}

func (p *PostgresRepository) Update(ctx context.Context, id int, in Input, user string) error {
	tag, err := p.pool.Exec(ctx,
		`UPDATE refpenunjang SET nama = $2, id_jenis = $3, hjual = $4, hjual_jamkes = $5, isactive = $6,
		        update_user = $7, update_date = now()
		 WHERE id = $1`,
		id, in.Nama, in.IDJenis, in.HJual, in.HJualJamkes, flag(in.IsActive), user)
	if err != nil {
		return translate(err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (p *PostgresRepository) UpsertPrice(ctx context.Context, id int, cabang string, in PriceInput) error {
	_, err := p.pool.Exec(ctx,
		`INSERT INTO refpenunjangdetail (id_penunjang, cabang, hjual, hjual_jamkes, isactive)
		 VALUES ($1, $2, $3, $4, $5)
		 ON CONFLICT (id_penunjang, cabang) DO UPDATE SET
		     hjual = EXCLUDED.hjual, hjual_jamkes = EXCLUDED.hjual_jamkes, isactive = EXCLUDED.isactive`,
		id, cabang, in.HJual, in.HJualJamkes, flag(in.IsActive))
	return translate(err)
}

func translate(err error) error {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != "23503" {
		return err
	}
	switch pgErr.ConstraintName {
	case "fk_refpenunjangdetail_penunjang":
		return ErrNotFound
	case "fk_refpenunjang_jenis":
		return ErrUnknownJenis
	}
	return err
}
