// Package tindakan serves Master Data Klinik > Tindakan: medical procedures
// (reftindakan) and their prices per branch (reftindakandetail). A user only
// sees and edits the prices of the branches they are mapped to (refbranchauth).
package tindakan

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

var ErrNotFound = errors.New("tindakan not found")

// Price is one reftindakandetail row: a procedure's prices at one branch.
type Price struct {
	Cabang      string   `json:"cabang"`
	HBeli       *float64 `json:"hbeli"`
	HJual       *float64 `json:"hjual"`
	HJualJamkes *float64 `json:"hjualJamkes"`
	IsActive    bool     `json:"isActive"`
}

// Tindakan is a reftindakan row plus its prices at the caller's branches.
type Tindakan struct {
	ID          int        `json:"id"`
	Nama        string     `json:"nama"`
	IDJenis     *int       `json:"idJenis"`
	HBeli       *float64   `json:"hbeli"`
	HJual       *float64   `json:"hjual"`
	HJualJamkes *float64   `json:"hjualJamkes"`
	IsActive    bool       `json:"isActive"`
	IsEdit      bool       `json:"isEdit"`
	CetakKartu  bool       `json:"cetakKartu"`
	UpdatedBy   string     `json:"updatedBy"`
	UpdatedAt   *time.Time `json:"updatedAt"`
	Prices      []Price    `json:"prices"`
}

// Input is the editable part of reftindakan.
type Input struct {
	Nama        string   `json:"nama"`
	IDJenis     *int     `json:"idJenis"`
	HBeli       *float64 `json:"hbeli"`
	HJual       *float64 `json:"hjual"`
	HJualJamkes *float64 `json:"hjualJamkes"`
	IsActive    bool     `json:"isActive"`
	IsEdit      bool     `json:"isEdit"`
	CetakKartu  bool     `json:"cetakKartu"`
}

// PriceInput is the editable part of a reftindakandetail row.
type PriceInput struct {
	HBeli       *float64 `json:"hbeli"`
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

// Normalize trims the name and validates against the reftindakan column sizes.
func (in *Input) Normalize() httpx.FieldErrors {
	in.Nama = strings.TrimSpace(in.Nama)
	errs := httpx.FieldErrors{}
	if in.Nama == "" {
		errs["nama"] = "Nama Tindakan is required"
	} else if utf8.RuneCountInString(in.Nama) > 100 {
		errs["nama"] = "Nama Tindakan must be at most 100 characters"
	}
	checkAmounts(errs, map[string]*float64{"hbeli": in.HBeli, "hjual": in.HJual, "hjualJamkes": in.HJualJamkes})
	return errs
}

func (in *PriceInput) Validate() httpx.FieldErrors {
	errs := httpx.FieldErrors{}
	checkAmounts(errs, map[string]*float64{"hbeli": in.HBeli, "hjual": in.HJual, "hjualJamkes": in.HJualJamkes})
	return errs
}

// Option is an entry of refjenistindakan.
type Option struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

type Repository interface {
	Jenis(ctx context.Context) ([]Option, error)
	UserBranches(ctx context.Context, userID string) ([]branchauth.Branch, error)
	// List returns every procedure with prices at the given branches only.
	List(ctx context.Context, branches []string) ([]Tindakan, error)
	Get(ctx context.Context, id int, branches []string) (Tindakan, error)
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
	rows, err := p.pool.Query(ctx, `SELECT id, COALESCE(info, '') FROM refjenistindakan ORDER BY info, id`)
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

const columns = `id, COALESCE(nama, ''), id_jenis, hbeli, hjual, hjual_jamkes,
	COALESCE(isactive, 0) = 1, COALESCE(isedit, 0) = 1, COALESCE(sts_ctk_krtu, 0) = 1,
	COALESCE(update_user, create_user, ''), COALESCE(update_date, create_date)`

func scan(r pgx.CollectableRow) (Tindakan, error) {
	var t Tindakan
	err := r.Scan(&t.ID, &t.Nama, &t.IDJenis, &t.HBeli, &t.HJual, &t.HJualJamkes,
		&t.IsActive, &t.IsEdit, &t.CetakKartu, &t.UpdatedBy, &t.UpdatedAt)
	t.Prices = []Price{}
	return t, err
}

// attachPrices fills Prices for the procedures in list from reftindakandetail at branches.
func (p *PostgresRepository) attachPrices(ctx context.Context, list []Tindakan, branches []string, only *int) error {
	if len(list) == 0 || len(branches) == 0 {
		return nil
	}
	index := make(map[int]int, len(list))
	for i, t := range list {
		index[t.ID] = i
	}
	rows, err := p.pool.Query(ctx,
		`SELECT id_tindakan, cabang, hbeli, hjual, hjual_jamkes, isactive = 1
		 FROM reftindakandetail
		 WHERE cabang = ANY($1) AND ($2::int IS NULL OR id_tindakan = $2)
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
		if err := rows.Scan(&id, &pr.Cabang, &pr.HBeli, &pr.HJual, &pr.HJualJamkes, &pr.IsActive); err != nil {
			return err
		}
		if i, ok := index[id]; ok {
			list[i].Prices = append(list[i].Prices, pr)
		}
	}
	return rows.Err()
}

func (p *PostgresRepository) List(ctx context.Context, branches []string) ([]Tindakan, error) {
	rows, err := p.pool.Query(ctx, `SELECT `+columns+` FROM reftindakan ORDER BY id`)
	if err != nil {
		return nil, err
	}
	list, err := pgx.CollectRows(rows, scan)
	if err != nil {
		return nil, err
	}
	if list == nil {
		list = []Tindakan{}
	}
	return list, p.attachPrices(ctx, list, branches, nil)
}

func (p *PostgresRepository) Get(ctx context.Context, id int, branches []string) (Tindakan, error) {
	rows, err := p.pool.Query(ctx, `SELECT `+columns+` FROM reftindakan WHERE id = $1`, id)
	if err != nil {
		return Tindakan{}, err
	}
	list, err := pgx.CollectRows(rows, scan)
	if err != nil {
		return Tindakan{}, err
	}
	if len(list) == 0 {
		return Tindakan{}, ErrNotFound
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
		`INSERT INTO reftindakan (nama, id_jenis, hbeli, hjual, hjual_jamkes, isactive, isedit, sts_ctk_krtu,
		                          create_user, create_date)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, now())
		 RETURNING id`,
		in.Nama, in.IDJenis, in.HBeli, in.HJual, in.HJualJamkes, flag(in.IsActive), flag(in.IsEdit), flag(in.CetakKartu), user,
	).Scan(&id)
	return id, err
}

func (p *PostgresRepository) Update(ctx context.Context, id int, in Input, user string) error {
	tag, err := p.pool.Exec(ctx,
		`UPDATE reftindakan SET nama = $2, id_jenis = $3, hbeli = $4, hjual = $5, hjual_jamkes = $6,
		        isactive = $7, isedit = $8, sts_ctk_krtu = $9, update_user = $10, update_date = now()
		 WHERE id = $1`,
		id, in.Nama, in.IDJenis, in.HBeli, in.HJual, in.HJualJamkes, flag(in.IsActive), flag(in.IsEdit), flag(in.CetakKartu), user)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (p *PostgresRepository) UpsertPrice(ctx context.Context, id int, cabang string, in PriceInput) error {
	_, err := p.pool.Exec(ctx,
		`INSERT INTO reftindakandetail (id_tindakan, cabang, hbeli, hjual, hjual_jamkes, isactive)
		 VALUES ($1, $2, $3, $4, $5, $6)
		 ON CONFLICT (id_tindakan, cabang) DO UPDATE SET
		     hbeli = EXCLUDED.hbeli, hjual = EXCLUDED.hjual,
		     hjual_jamkes = EXCLUDED.hjual_jamkes, isactive = EXCLUDED.isactive`,
		id, cabang, in.HBeli, in.HJual, in.HJualJamkes, flag(in.IsActive))
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.ConstraintName == "fk_reftindakandetail_tindakan" {
		return ErrNotFound
	}
	return err
}
