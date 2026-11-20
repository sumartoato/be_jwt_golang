package reportklinik

import (
	"context"
	"fmt"
	"strings"
)

// PenunjangRow is one supporting-examination line of Penjualan Penunjang.
type PenunjangRow struct {
	ID         int     `json:"id"` // trxjualpnjng id
	IDJual     int     `json:"idJual"`
	Tgl        string  `json:"tgl"` // YYYY-MM-DD, clinic date
	NoRM       string  `json:"noRm"`
	Pasien     string  `json:"pasien"`
	Penunjang  string  `json:"penunjang"`
	Jenis      string  `json:"jenis"`  // refjenispanjang, e.g. LABORATORIUM CV
	Dokter     string  `json:"dokter"` // the sale's Dokter PJ
	Harga      float64 `json:"harga"`  // the price sold at (trxjualpnjng.hjual)
	Byk        float64 `json:"byk"`
	Cabang     string  `json:"cabang"`
	Status     string  `json:"status"`     // open, close or cancel
	CreateUser string  `json:"createUser"` // Dibuat Oleh
}

// PenunjangParams selects one page of Penjualan Penunjang.
type PenunjangParams struct {
	Branches []string // always applied
	From, To string   // YYYY-MM-DD, inclusive, clinic dates
	Search   string   // the table's search box
	Nama     string   // Cari: examination name
	IDJenis  *int     // Cari: refjenispanjang id
	Status   string   // Cari: open, close, cancel or ""
	Sort     string
	Desc     bool
	Limit    int
	Offset   int
}

type PenunjangPage struct {
	Rows            []PenunjangRow `json:"rows"`
	Total           int            `json:"total"`
	UnfilteredTotal int            `json:"unfilteredTotal"`
}

var penunjangSort = map[string]string{
	"tgl":        "j.tgl",
	"noRm":       "p.no_rm",
	"pasien":     "p.nama",
	"penunjang":  "r.nama",
	"jenis":      "jp.info",
	"dokter":     "dk.nama",
	"harga":      "x.hjual",
	"byk":        "x.byk",
	"cabang":     "b.nama",
	"status":     statusSQL,
	"createUser": "j.create_user",
}

// PenunjangSortable reports whether key is a Penjualan Penunjang sort column.
func PenunjangSortable(key string) bool {
	_, ok := penunjangSort[key]
	return ok
}

const penunjangFrom = ` FROM trxjual j
	JOIN trxjualpnjng x ON x.id_jual = j.id
	LEFT JOIN trxjualtotal t ON t.id_jual = j.id
	LEFT JOIN refpenunjang r ON r.id = x.id_pnjng
	LEFT JOIN refjenispanjang jp ON jp.id = r.id_jenis
	LEFT JOIN refmdokter dk ON dk.id = j.id_dok
	LEFT JOIN refmpasien p ON p.id = j.id_pas
	LEFT JOIN refbranch b ON b.kode = j.cabang`

func penunjangWhere(pp PenunjangParams, withSearch bool) (string, []any) {
	args := []any{pp.Branches, pp.From, pp.To}
	conds := []string{`j.cabang = ANY($1)`, `j.tgl >= $2::date`, `j.tgl < $3::date + 1`}
	add := func(cond string, v any) {
		args = append(args, v)
		conds = append(conds, fmt.Sprintf(cond, len(args)))
	}
	if pp.Nama != "" {
		add(`r.nama ILIKE $%d`, "%"+escapeLike(pp.Nama)+"%")
	}
	if pp.IDJenis != nil {
		add(`r.id_jenis = $%d`, *pp.IDJenis)
	}
	if pp.Status != "" {
		add(statusSQL+` = $%d`, pp.Status)
	}
	if withSearch && pp.Search != "" {
		args = append(args, "%"+escapeLike(pp.Search)+"%")
		conds = append(conds, fmt.Sprintf(`(r.nama ILIKE $%[1]d OR jp.info ILIKE $%[1]d OR dk.nama ILIKE $%[1]d OR p.nama ILIKE $%[1]d
			OR p.no_rm ILIKE $%[1]d OR b.nama ILIKE $%[1]d OR j.create_user ILIKE $%[1]d)`, len(args)))
	}
	return strings.Join(conds, " AND "), args
}

func (p *PostgresRepository) PenjualanPenunjang(ctx context.Context, pp PenunjangParams) (PenunjangPage, error) {
	page := PenunjangPage{Rows: []PenunjangRow{}}
	col, ok := penunjangSort[pp.Sort]
	if !ok {
		col = "j.tgl"
	}
	dir := "ASC"
	if pp.Desc {
		dir = "DESC"
	}
	cond, args := penunjangWhere(pp, true)
	n := len(args)
	rows, err := p.pool.Query(ctx, fmt.Sprintf(`SELECT x.id, j.id, to_char(j.tgl, 'YYYY-MM-DD'), COALESCE(p.no_rm, ''), COALESCE(p.nama, ''),
			COALESCE(r.nama, '(penunjang ' || x.id_pnjng || ')'), COALESCE(jp.info, ''), COALESCE(trim(dk.nama), ''),
			COALESCE(x.hjual, 0), COALESCE(x.byk, 0), COALESCE(j.cabang, ''), `+statusSQL+`, COALESCE(j.create_user, ''), count(*) OVER ()
		%s WHERE %s ORDER BY %s %s NULLS LAST, j.tgl %s, x.id %s LIMIT $%d OFFSET $%d`,
		penunjangFrom, cond, col, dir, dir, dir, n+1, n+2), append(args, pp.Limit, pp.Offset)...)
	if err != nil {
		return page, err
	}
	defer rows.Close()
	for rows.Next() {
		var x PenunjangRow
		if err := rows.Scan(&x.ID, &x.IDJual, &x.Tgl, &x.NoRM, &x.Pasien, &x.Penunjang, &x.Jenis, &x.Dokter, &x.Harga, &x.Byk,
			&x.Cabang, &x.Status, &x.CreateUser, &page.Total); err != nil {
			return page, err
		}
		page.Rows = append(page.Rows, x)
	}
	if err := rows.Err(); err != nil {
		return page, err
	}
	// An offset past the end returns no rows, and so no window count.
	if len(page.Rows) == 0 && pp.Offset > 0 {
		if err := p.pool.QueryRow(ctx, `SELECT count(*)`+penunjangFrom+` WHERE `+cond, args...).Scan(&page.Total); err != nil {
			return page, err
		}
	}
	if pp.Search == "" {
		page.UnfilteredTotal = page.Total
		return page, nil
	}
	cond, args = penunjangWhere(pp, false)
	err = p.pool.QueryRow(ctx, `SELECT count(*)`+penunjangFrom+` WHERE `+cond, args...).Scan(&page.UnfilteredTotal)
	return page, err
}

// JenisPenunjang lists the examination types (refjenispanjang) for the Cari panel.
func (p *PostgresRepository) JenisPenunjang(ctx context.Context) ([]Option, error) {
	return p.optionList(ctx, `SELECT id, COALESCE(trim(info), '') FROM refjenispanjang ORDER BY info, id`)
}
