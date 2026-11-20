package reportklinik

import (
	"context"
	"fmt"
	"strings"
)

// TindakanRow is one procedure line of Penjualan Tindakan. Fee Dokter is the
// branch's doctor fee for the procedure (reftindakandetail.hbeli, the master
// value: sales do not store it), and Total Fee Dokter is Fee Dokter × Banyak.
type TindakanRow struct {
	ID       int     `json:"id"` // trxjualtindakan id
	IDJual   int     `json:"idJual"`
	Tgl      string  `json:"tgl"` // YYYY-MM-DD, clinic date
	Tindakan string  `json:"tindakan"`
	Jenis    string  `json:"jenis"` // refjenistindakan
	Dokter   string  `json:"dokter"`
	Pasien   string  `json:"pasien"`
	Fee      float64 `json:"fee"`
	Harga    float64 `json:"harga"` // the price sold at (trxjualtindakan.hjual)
	Byk      float64 `json:"byk"`
	TotalFee float64 `json:"totalFee"`
	Cabang   string  `json:"cabang"`
	Status   string  `json:"status"` // open, close or cancel
}

// TindakanParams selects one page of Penjualan Tindakan.
type TindakanParams struct {
	Branches []string // always applied
	From, To string   // YYYY-MM-DD, inclusive, clinic dates
	Search   string   // the table's search box
	Nama     string   // Cari: procedure name
	Dokter   string   // Cari: doctor name
	Status   string   // Cari: open, close, cancel or ""
	Sort     string
	Desc     bool
	Limit    int
	Offset   int
}

type TindakanPage struct {
	Rows            []TindakanRow `json:"rows"`
	Total           int           `json:"total"`
	UnfilteredTotal int           `json:"unfilteredTotal"`
}

var tindakanSort = map[string]string{
	"tgl":      "j.tgl",
	"tindakan": "r.nama",
	"jenis":    "jt.info",
	"dokter":   "dk.nama",
	"pasien":   "p.nama",
	"fee":      "d.hbeli",
	"harga":    "x.hjual",
	"byk":      "x.byk",
	"totalFee": "COALESCE(d.hbeli, 0) * COALESCE(x.byk, 0)",
	"cabang":   "b.nama",
	"status":   statusSQL,
}

// TindakanSortable reports whether key is a Penjualan Tindakan sort column.
func TindakanSortable(key string) bool {
	_, ok := tindakanSort[key]
	return ok
}

// The line's doctor (trxjualtindakan.id_dok), else the sale's Dokter PJ.
const tindakanFrom = ` FROM trxjual j
	JOIN trxjualtindakan x ON x.id_jual = j.id
	LEFT JOIN trxjualtotal t ON t.id_jual = j.id
	LEFT JOIN reftindakan r ON r.id = x.id_tndk
	LEFT JOIN refjenistindakan jt ON jt.id = r.id_jenis
	LEFT JOIN reftindakandetail d ON d.id_tindakan = x.id_tndk AND d.cabang = j.cabang
	LEFT JOIN refmdokter dk ON dk.id = COALESCE(x.id_dok, j.id_dok)
	LEFT JOIN refmpasien p ON p.id = j.id_pas
	LEFT JOIN refbranch b ON b.kode = j.cabang`

func tindakanWhere(tp TindakanParams, withSearch bool) (string, []any) {
	args := []any{tp.Branches, tp.From, tp.To}
	conds := []string{`j.cabang = ANY($1)`, `j.tgl >= $2::date`, `j.tgl < $3::date + 1`}
	add := func(cond string, v any) {
		args = append(args, v)
		conds = append(conds, fmt.Sprintf(cond, len(args)))
	}
	if tp.Nama != "" {
		add(`r.nama ILIKE $%d`, "%"+escapeLike(tp.Nama)+"%")
	}
	if tp.Dokter != "" {
		add(`dk.nama ILIKE $%d`, "%"+escapeLike(tp.Dokter)+"%")
	}
	if tp.Status != "" {
		add(statusSQL+` = $%d`, tp.Status)
	}
	if withSearch && tp.Search != "" {
		args = append(args, "%"+escapeLike(tp.Search)+"%")
		conds = append(conds, fmt.Sprintf(`(r.nama ILIKE $%[1]d OR jt.info ILIKE $%[1]d OR dk.nama ILIKE $%[1]d OR p.nama ILIKE $%[1]d OR b.nama ILIKE $%[1]d)`, len(args)))
	}
	return strings.Join(conds, " AND "), args
}

func (p *PostgresRepository) PenjualanTindakan(ctx context.Context, tp TindakanParams) (TindakanPage, error) {
	page := TindakanPage{Rows: []TindakanRow{}}
	col, ok := tindakanSort[tp.Sort]
	if !ok {
		col = "j.tgl"
	}
	dir := "ASC"
	if tp.Desc {
		dir = "DESC"
	}
	cond, args := tindakanWhere(tp, true)
	n := len(args)
	rows, err := p.pool.Query(ctx, fmt.Sprintf(`SELECT x.id, j.id, to_char(j.tgl, 'YYYY-MM-DD'), COALESCE(r.nama, '(tindakan ' || x.id_tndk || ')'),
			COALESCE(jt.info, ''), COALESCE(trim(dk.nama), ''), COALESCE(p.nama, ''), COALESCE(d.hbeli, 0), COALESCE(x.hjual, 0),
			COALESCE(x.byk, 0), COALESCE(d.hbeli, 0) * COALESCE(x.byk, 0), COALESCE(j.cabang, ''), `+statusSQL+`, count(*) OVER ()
		%s WHERE %s ORDER BY %s %s NULLS LAST, j.tgl %s, x.id %s LIMIT $%d OFFSET $%d`,
		tindakanFrom, cond, col, dir, dir, dir, n+1, n+2), append(args, tp.Limit, tp.Offset)...)
	if err != nil {
		return page, err
	}
	defer rows.Close()
	for rows.Next() {
		var x TindakanRow
		if err := rows.Scan(&x.ID, &x.IDJual, &x.Tgl, &x.Tindakan, &x.Jenis, &x.Dokter, &x.Pasien, &x.Fee, &x.Harga, &x.Byk,
			&x.TotalFee, &x.Cabang, &x.Status, &page.Total); err != nil {
			return page, err
		}
		page.Rows = append(page.Rows, x)
	}
	if err := rows.Err(); err != nil {
		return page, err
	}
	// An offset past the end returns no rows, and so no window count.
	if len(page.Rows) == 0 && tp.Offset > 0 {
		if err := p.pool.QueryRow(ctx, `SELECT count(*)`+tindakanFrom+` WHERE `+cond, args...).Scan(&page.Total); err != nil {
			return page, err
		}
	}
	if tp.Search == "" {
		page.UnfilteredTotal = page.Total
		return page, nil
	}
	cond, args = tindakanWhere(tp, false)
	err = p.pool.QueryRow(ctx, `SELECT count(*)`+tindakanFrom+` WHERE `+cond, args...).Scan(&page.UnfilteredTotal)
	return page, err
}
