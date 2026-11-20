package reportklinik

import (
	"context"
	"fmt"
	"strings"
)

// HarianRow is one sale of Transaksi Harian with its cost per group
// (trxjualtotal), total and discount.
type HarianRow struct {
	IDJual     int     `json:"idJual"`
	Tgl        string  `json:"tgl"` // YYYY-MM-DD, clinic date
	NoRM       string  `json:"noRm"`
	Pasien     string  `json:"pasien"`
	Pelayanan  string  `json:"pelayanan"` // refjenisrawat
	Dokter     string  `json:"dokter"`
	Pembayaran string  `json:"pembayaran"` // refjenisbayar
	Obat       float64 `json:"obat"`
	Tindakan   float64 `json:"tindakan"`
	Penunjang  float64 `json:"penunjang"`
	Total      float64 `json:"total"`
	Cabang     string  `json:"cabang"`
	Status     string  `json:"status"` // open, close or cancel
	Diskon     float64 `json:"diskon"` // trxjualtotal.potongan
}

// HarianParams selects one page of Transaksi Harian.
type HarianParams struct {
	Branches     []string // always applied
	From, To     string   // YYYY-MM-DD, inclusive, clinic dates
	Search       string   // the table's search box
	IDJenisRawat *int     // Pelayanan (toolbar)
	Nama         string   // Cari: patient name
	IDJenisBayar *int     // Cari: refjenisbayar id
	Status       string   // Cari: open, close, cancel or ""
	Sort         string
	Desc         bool
	Limit        int
	Offset       int
}

type HarianPage struct {
	Rows            []HarianRow `json:"rows"`
	Total           int         `json:"total"`
	UnfilteredTotal int         `json:"unfilteredTotal"`
}

var harianSort = map[string]string{
	"tgl":        "j.tgl",
	"noRm":       "p.no_rm",
	"pasien":     "p.nama",
	"pelayanan":  "r.info",
	"dokter":     "dk.nama",
	"pembayaran": "jb.info",
	"obat":       "t.tot_obat",
	"tindakan":   "t.tot_tndk",
	"penunjang":  "t.tot_pnjng",
	"total":      "t.total",
	"cabang":     "b.nama",
	"status":     statusSQL,
	"diskon":     "t.potongan",
}

// HarianSortable reports whether key is a Transaksi Harian sort column.
func HarianSortable(key string) bool {
	_, ok := harianSort[key]
	return ok
}

const harianFrom = ` FROM trxjual j
	LEFT JOIN trxjualtotal t ON t.id_jual = j.id
	LEFT JOIN refmpasien p ON p.id = j.id_pas
	LEFT JOIN refjenisrawat r ON r.id = j.id_jenis_rwt
	LEFT JOIN refmdokter dk ON dk.id = j.id_dok
	LEFT JOIN refjenisbayar jb ON jb.id = j.id_jenis_byr
	LEFT JOIN refbranch b ON b.kode = j.cabang`

func harianWhere(hp HarianParams, withSearch bool) (string, []any) {
	args := []any{hp.Branches, hp.From, hp.To}
	conds := []string{`j.cabang = ANY($1)`, `j.tgl >= $2::date`, `j.tgl < $3::date + 1`}
	add := func(cond string, v any) {
		args = append(args, v)
		conds = append(conds, fmt.Sprintf(cond, len(args)))
	}
	if hp.IDJenisRawat != nil {
		add(`j.id_jenis_rwt = $%d`, *hp.IDJenisRawat)
	}
	if hp.Nama != "" {
		add(`p.nama ILIKE $%d`, "%"+escapeLike(hp.Nama)+"%")
	}
	if hp.IDJenisBayar != nil {
		add(`j.id_jenis_byr = $%d`, *hp.IDJenisBayar)
	}
	if hp.Status != "" {
		add(statusSQL+` = $%d`, hp.Status)
	}
	if withSearch && hp.Search != "" {
		args = append(args, "%"+escapeLike(hp.Search)+"%")
		conds = append(conds, fmt.Sprintf(`(p.nama ILIKE $%[1]d OR p.no_rm ILIKE $%[1]d OR r.info ILIKE $%[1]d OR dk.nama ILIKE $%[1]d
			OR jb.info ILIKE $%[1]d OR b.nama ILIKE $%[1]d)`, len(args)))
	}
	return strings.Join(conds, " AND "), args
}

func (p *PostgresRepository) TransaksiHarian(ctx context.Context, hp HarianParams) (HarianPage, error) {
	page := HarianPage{Rows: []HarianRow{}}
	col, ok := harianSort[hp.Sort]
	if !ok {
		col = "j.tgl"
	}
	dir := "ASC"
	if hp.Desc {
		dir = "DESC"
	}
	cond, args := harianWhere(hp, true)
	n := len(args)
	rows, err := p.pool.Query(ctx, fmt.Sprintf(`SELECT j.id, to_char(j.tgl, 'YYYY-MM-DD'), COALESCE(p.no_rm, ''), COALESCE(p.nama, ''), COALESCE(r.info, ''),
			COALESCE(trim(dk.nama), ''), COALESCE(jb.info, ''), COALESCE(t.tot_obat, 0), COALESCE(t.tot_tndk, 0), COALESCE(t.tot_pnjng, 0),
			COALESCE(t.total, 0), COALESCE(j.cabang, ''), `+statusSQL+`, COALESCE(t.potongan, 0), count(*) OVER ()
		%s WHERE %s ORDER BY %s %s NULLS LAST, j.tgl %s, j.id %s LIMIT $%d OFFSET $%d`,
		harianFrom, cond, col, dir, dir, dir, n+1, n+2), append(args, hp.Limit, hp.Offset)...)
	if err != nil {
		return page, err
	}
	defer rows.Close()
	for rows.Next() {
		var x HarianRow
		if err := rows.Scan(&x.IDJual, &x.Tgl, &x.NoRM, &x.Pasien, &x.Pelayanan, &x.Dokter, &x.Pembayaran, &x.Obat, &x.Tindakan,
			&x.Penunjang, &x.Total, &x.Cabang, &x.Status, &x.Diskon, &page.Total); err != nil {
			return page, err
		}
		page.Rows = append(page.Rows, x)
	}
	if err := rows.Err(); err != nil {
		return page, err
	}
	// An offset past the end returns no rows, and so no window count.
	if len(page.Rows) == 0 && hp.Offset > 0 {
		if err := p.pool.QueryRow(ctx, `SELECT count(*)`+harianFrom+` WHERE `+cond, args...).Scan(&page.Total); err != nil {
			return page, err
		}
	}
	if hp.Search == "" {
		page.UnfilteredTotal = page.Total
		return page, nil
	}
	cond, args = harianWhere(hp, false)
	err = p.pool.QueryRow(ctx, `SELECT count(*)`+harianFrom+` WHERE `+cond, args...).Scan(&page.UnfilteredTotal)
	return page, err
}

// JenisBayar lists the payment types in use (flag 1; flag 2 rows repeat TUNAI
// and HUTANG and are unused) for the Cari panel.
func (p *PostgresRepository) JenisBayar(ctx context.Context) ([]Option, error) {
	return p.optionList(ctx, `SELECT id, COALESCE(info, '') FROM refjenisbayar WHERE COALESCE(flag, 1) = 1 ORDER BY id`)
}
