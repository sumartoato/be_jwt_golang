// Package penerimaan serves Transaksi Klinik > Penerimaan Barang (legacy
// /GoodReceive): the goods receipts (trxfaktur) with their supplier and total
// (trxfakturtotal), and the Add form that records a receipt and adds its
// drugs to the branch stock. A user only sees, and receives for, the branches
// they are mapped to (refbranchauth).
package penerimaan

import (
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/example/be_jwt_golang/backend/internal/branchauth"
)

// Row is a goods receipt as listed.
type Row struct {
	ID       int     `json:"id"`       // No
	NoGR     string  `json:"noGr"`     // trxfaktur.no_fak
	TglTrans *string `json:"tglTrans"` // YYYY-MM-DD, entry date (tgl)
	TglGR    *string `json:"tglGr"`    // YYYY-MM-DD, receipt date (tgl_fak)
	Supplier string  `json:"supplier"` // "" when none (id_supp 0)
	Total    float64 `json:"total"`
	Cabang   string  `json:"cabang"`
}

// ListParams selects one page of receipts.
type ListParams struct {
	Branches   []string // always applied
	From, To   string   // YYYY-MM-DD, inclusive, clinic dates of tgl
	Search     string   // the table's search box
	NoGR       string   // Cari: No GR
	IDSupplier *int     // Cari: refsupplier id; 0 = none
	Sort       string
	Desc       bool
	Limit      int
	Offset     int
}

type Page struct {
	Rows            []Row `json:"rows"`
	Total           int   `json:"total"`
	UnfilteredTotal int   `json:"unfilteredTotal"`
}

type Option struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

type Repository interface {
	UserBranches(ctx context.Context, userID string) ([]branchauth.Branch, error)
	Suppliers(ctx context.Context) ([]Option, error)
	List(ctx context.Context, p ListParams) (Page, error)
	Distributors(ctx context.Context) ([]Distributor, error)
	Obat(ctx context.Context, branch string) ([]Obat, error)
	Save(ctx context.Context, in SaveInput, user string) (SaveResult, error)
	Detail(ctx context.Context, id int) (Detail, error)
}

type PostgresRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresRepository(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{pool: pool}
}

func (p *PostgresRepository) UserBranches(ctx context.Context, userID string) ([]branchauth.Branch, error) {
	return branchauth.ForUser(ctx, p.pool, userID)
}

func (p *PostgresRepository) Suppliers(ctx context.Context) ([]Option, error) {
	rows, err := p.pool.Query(ctx, `SELECT id, COALESCE(trim(nama), '') FROM refsupplier ORDER BY nama, id`)
	if err != nil {
		return nil, err
	}
	list, err := pgx.CollectRows(rows, func(r pgx.CollectableRow) (Option, error) {
		var o Option
		err := r.Scan(&o.ID, &o.Name)
		return o, err
	})
	if list == nil {
		list = []Option{}
	}
	return list, err
}

func escapeLike(s string) string {
	return strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(s)
}

// sortColumns are the sortable list columns (fixed SQL, never from the request).
var sortColumns = map[string]string{
	"id":       "f.id",
	"noGr":     "f.no_fak",
	"tglTrans": "f.tgl",
	"tglGr":    "f.tgl_fak",
	"supplier": "s.nama",
	"total":    "t.total",
	"cabang":   "b.nama",
}

// Sortable reports whether key is a list sort column.
func Sortable(key string) bool {
	_, ok := sortColumns[key]
	return ok
}

const fromJoins = ` FROM trxfaktur f
	LEFT JOIN trxfakturtotal t ON t.id_fak = f.id
	LEFT JOIN refsupplier s ON s.id = f.id_supp
	LEFT JOIN refbranch b ON b.kode = f.cabang`

func where(lp ListParams, withSearch bool) (string, []any) {
	args := []any{lp.Branches, lp.From, lp.To}
	conds := []string{`f.cabang = ANY($1)`, `f.tgl >= $2::date`, `f.tgl < $3::date + 1`}
	add := func(cond string, v any) {
		args = append(args, v)
		conds = append(conds, fmt.Sprintf(cond, len(args)))
	}
	if lp.NoGR != "" {
		add(`f.no_fak ILIKE $%d`, "%"+escapeLike(lp.NoGR)+"%")
	}
	if lp.IDSupplier != nil {
		add(`COALESCE(f.id_supp, 0) = $%d`, *lp.IDSupplier)
	}
	if withSearch && lp.Search != "" {
		args = append(args, "%"+escapeLike(lp.Search)+"%")
		n := len(args)
		cond := fmt.Sprintf(`f.no_fak ILIKE $%[1]d OR s.nama ILIKE $%[1]d OR b.nama ILIKE $%[1]d`, n)
		var id int
		if _, err := fmt.Sscanf(lp.Search, "%d", &id); err == nil && fmt.Sprint(id) == lp.Search {
			args = append(args, id)
			cond += fmt.Sprintf(` OR f.id = $%d`, len(args))
		}
		conds = append(conds, `(`+cond+`)`)
	}
	return strings.Join(conds, " AND "), args
}

func (p *PostgresRepository) List(ctx context.Context, lp ListParams) (Page, error) {
	page := Page{Rows: []Row{}}
	col, ok := sortColumns[lp.Sort]
	if !ok {
		col = "f.id"
	}
	dir := "ASC"
	if lp.Desc {
		dir = "DESC"
	}
	cond, args := where(lp, true)
	n := len(args)
	rows, err := p.pool.Query(ctx, fmt.Sprintf(`SELECT f.id, COALESCE(f.no_fak, ''), to_char(f.tgl, 'YYYY-MM-DD'), to_char(f.tgl_fak, 'YYYY-MM-DD'),
			COALESCE(trim(s.nama), ''), COALESCE(t.total, 0), COALESCE(f.cabang, ''), count(*) OVER ()
		%s WHERE %s ORDER BY %s %s NULLS LAST, f.id %s LIMIT $%d OFFSET $%d`, fromJoins, cond, col, dir, dir, n+1, n+2),
		append(args, lp.Limit, lp.Offset)...)
	if err != nil {
		return page, err
	}
	defer rows.Close()
	for rows.Next() {
		var x Row
		if err := rows.Scan(&x.ID, &x.NoGR, &x.TglTrans, &x.TglGR, &x.Supplier, &x.Total, &x.Cabang, &page.Total); err != nil {
			return page, err
		}
		page.Rows = append(page.Rows, x)
	}
	if err := rows.Err(); err != nil {
		return page, err
	}
	// An offset past the end returns no rows, and so no window count.
	if len(page.Rows) == 0 && lp.Offset > 0 {
		if err := p.pool.QueryRow(ctx, `SELECT count(*)`+fromJoins+` WHERE `+cond, args...).Scan(&page.Total); err != nil {
			return page, err
		}
	}
	if lp.Search == "" {
		page.UnfilteredTotal = page.Total
		return page, nil
	}
	cond, args = where(lp, false)
	err = p.pool.QueryRow(ctx, `SELECT count(*)`+fromJoins+` WHERE `+cond, args...).Scan(&page.UnfilteredTotal)
	return page, err
}
