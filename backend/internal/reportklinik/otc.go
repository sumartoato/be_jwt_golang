package reportklinik

import (
	"context"
	"fmt"
	"strings"
)

// Drug types (refjenisobat.info) with a sales report of their own.
const (
	JenisOTC       = "OTC"        // Penjualan OTC
	JenisSoftDrink = "SOFT DRINK" // Penjualan Softdrink
)

// OTCRow is one drug line of Penjualan OTC or Penjualan Softdrink: a line
// whose drug is of the report's type (refjenisobat), with the sale's OTC
// referrer.
type OTCRow struct {
	ID      int     `json:"id"` // trxjualobat id
	IDJual  int     `json:"idJual"`
	Tgl     string  `json:"tgl"` // YYYY-MM-DD, clinic date
	NoKwi   string  `json:"noKwi"`
	Obat    string  `json:"obat"`
	Jenis   string  `json:"jenis"`
	Perujuk string  `json:"perujuk"` // refperujukotc via trxjual.id_perujuk_otc
	Byk     float64 `json:"byk"`
	Harga   float64 `json:"harga"` // trxjualobat.hjual
	PPN     float64 `json:"ppn"`   // trxjualobat.ppn
	Total   float64 `json:"total"` // Harga × Banyak + PPN
	Cabang  string  `json:"cabang"`
	Status  string  `json:"status"` // open, close or cancel
}

// OTCParams selects one page of Penjualan OTC.
type OTCParams struct {
	Branches  []string // always applied
	From, To  string   // YYYY-MM-DD, inclusive, clinic dates
	Jenis     string   // the drug type: JenisOTC or JenisSoftDrink
	Search    string   // the table's search box
	Nama      string   // Cari: drug name
	IDPerujuk *int     // Cari: refperujukotc id
	Status    string   // Cari: open, close, cancel or ""
	Sort      string
	Desc      bool
	Limit     int
	Offset    int
}

type OTCPage struct {
	Rows            []OTCRow `json:"rows"`
	Total           int      `json:"total"`
	UnfilteredTotal int      `json:"unfilteredTotal"`
}

var otcSort = map[string]string{
	"idJual":  "j.id",
	"tgl":     "j.tgl",
	"noKwi":   "j.no_kwi",
	"obat":    "o.nama",
	"jenis":   "jo.info",
	"perujuk": "pr.nama",
	"byk":     "x.byk",
	"harga":   "x.hjual",
	"ppn":     "x.ppn",
	"total":   "COALESCE(x.hjual, 0) * COALESCE(x.byk, 0) + COALESCE(x.ppn, 0)",
	"cabang":  "b.nama",
	"status":  statusSQL,
}

// OTCSortable reports whether key is a Penjualan OTC sort column.
func OTCSortable(key string) bool {
	_, ok := otcSort[key]
	return ok
}

const otcFrom = ` FROM trxjual j
	JOIN trxjualobat x ON x.id_jual = j.id
	JOIN refobat o ON o.id = x.id_obat
	JOIN refjenisobat jo ON jo.id = o.id_jenis
	LEFT JOIN trxjualtotal t ON t.id_jual = j.id
	LEFT JOIN refperujukotc pr ON pr.id = j.id_perujuk_otc
	LEFT JOIN refbranch b ON b.kode = j.cabang`

func otcWhere(op OTCParams, withSearch bool) (string, []any) {
	jenis := op.Jenis
	if jenis == "" {
		jenis = JenisOTC
	}
	args := []any{op.Branches, op.From, op.To, jenis}
	conds := []string{`j.cabang = ANY($1)`, `j.tgl >= $2::date`, `j.tgl < $3::date + 1`, `upper(trim(jo.info)) = $4`}
	add := func(cond string, v any) {
		args = append(args, v)
		conds = append(conds, fmt.Sprintf(cond, len(args)))
	}
	if op.Nama != "" {
		add(`o.nama ILIKE $%d`, "%"+escapeLike(op.Nama)+"%")
	}
	if op.IDPerujuk != nil {
		add(`j.id_perujuk_otc = $%d`, *op.IDPerujuk)
	}
	if op.Status != "" {
		add(statusSQL+` = $%d`, op.Status)
	}
	if withSearch && op.Search != "" {
		args = append(args, "%"+escapeLike(op.Search)+"%")
		n := len(args)
		cond := fmt.Sprintf(`o.nama ILIKE $%[1]d OR j.no_kwi ILIKE $%[1]d OR pr.nama ILIKE $%[1]d OR b.nama ILIKE $%[1]d`, n)
		var id int
		if _, err := fmt.Sscanf(op.Search, "%d", &id); err == nil && fmt.Sprint(id) == op.Search {
			args = append(args, id)
			cond += fmt.Sprintf(` OR j.id = $%d`, len(args))
		}
		conds = append(conds, `(`+cond+`)`)
	}
	return strings.Join(conds, " AND "), args
}

func (p *PostgresRepository) PenjualanOTC(ctx context.Context, op OTCParams) (OTCPage, error) {
	page := OTCPage{Rows: []OTCRow{}}
	col, ok := otcSort[op.Sort]
	if !ok {
		col = "j.id"
	}
	dir := "ASC"
	if op.Desc {
		dir = "DESC"
	}
	cond, args := otcWhere(op, true)
	n := len(args)
	rows, err := p.pool.Query(ctx, fmt.Sprintf(`SELECT x.id, j.id, to_char(j.tgl, 'YYYY-MM-DD'), COALESCE(j.no_kwi, ''), COALESCE(o.nama, ''),
			COALESCE(jo.info, ''), COALESCE(pr.nama, ''), COALESCE(x.byk, 0), COALESCE(x.hjual, 0), COALESCE(x.ppn, 0),
			COALESCE(x.hjual, 0) * COALESCE(x.byk, 0) + COALESCE(x.ppn, 0), COALESCE(j.cabang, ''), `+statusSQL+`, count(*) OVER ()
		%s WHERE %s ORDER BY %s %s NULLS LAST, j.id %s, x.id LIMIT $%d OFFSET $%d`,
		otcFrom, cond, col, dir, dir, n+1, n+2), append(args, op.Limit, op.Offset)...)
	if err != nil {
		return page, err
	}
	defer rows.Close()
	for rows.Next() {
		var x OTCRow
		if err := rows.Scan(&x.ID, &x.IDJual, &x.Tgl, &x.NoKwi, &x.Obat, &x.Jenis, &x.Perujuk, &x.Byk, &x.Harga, &x.PPN,
			&x.Total, &x.Cabang, &x.Status, &page.Total); err != nil {
			return page, err
		}
		page.Rows = append(page.Rows, x)
	}
	if err := rows.Err(); err != nil {
		return page, err
	}
	// An offset past the end returns no rows, and so no window count.
	if len(page.Rows) == 0 && op.Offset > 0 {
		if err := p.pool.QueryRow(ctx, `SELECT count(*)`+otcFrom+` WHERE `+cond, args...).Scan(&page.Total); err != nil {
			return page, err
		}
	}
	if op.Search == "" {
		page.UnfilteredTotal = page.Total
		return page, nil
	}
	cond, args = otcWhere(op, false)
	err = p.pool.QueryRow(ctx, `SELECT count(*)`+otcFrom+` WHERE `+cond, args...).Scan(&page.UnfilteredTotal)
	return page, err
}

// PerujukOTC lists the OTC referrers (refperujukotc) for the Cari panel.
func (p *PostgresRepository) PerujukOTC(ctx context.Context) ([]Option, error) {
	return p.optionList(ctx, `SELECT id, COALESCE(trim(nama), '') FROM refperujukotc ORDER BY nama, id`)
}
