package reportklinik

import (
	"context"
	"fmt"
	"strings"
)

// DokterEksternal is refmdokter.id_internal of an external doctor
// (refdokterinternal: 1 -, 2 INTERNAL, 3 EKSTERNAL).
const DokterEksternal = 3

// RujukanRow is one row of Rujukan: a sale whose Dokter PJ is an external
// doctor, once per supporting examination it holds (Total is that
// examination's Harga × Banyak); a sale without examinations has one row with
// no examination and a zero total.
type RujukanRow struct {
	IDJual     int     `json:"idJual"`
	IDLine     *int    `json:"idLine"` // trxjualpnjng id; nil without an examination
	Tgl        string  `json:"tgl"`    // YYYY-MM-DD, clinic date
	NoRM       string  `json:"noRm"`
	Pasien     string  `json:"pasien"`
	Dokter     string  `json:"dokter"` // Dokter Eksternal
	JenisRawat string  `json:"jenisRawat"`
	Penunjang  string  `json:"penunjang"`
	Total      float64 `json:"total"`
	Cabang     string  `json:"cabang"`
	Status     string  `json:"status"` // open, close or cancel
}

// RujukanParams selects one page of Rujukan.
type RujukanParams struct {
	Branches []string // always applied
	From, To string   // YYYY-MM-DD, inclusive, clinic dates
	Search   string   // the table's search box
	Nama     string   // Cari: patient name
	Dokter   string   // Cari: doctor name
	Status   string   // Cari: open, close, cancel or ""
	Sort     string
	Desc     bool
	Limit    int
	Offset   int
}

type RujukanPage struct {
	Rows            []RujukanRow `json:"rows"`
	Total           int          `json:"total"`
	UnfilteredTotal int          `json:"unfilteredTotal"`
}

var rujukanSort = map[string]string{
	"tgl":        "j.tgl",
	"noRm":       "p.no_rm",
	"pasien":     "p.nama",
	"dokter":     "dk.nama",
	"jenisRawat": "r.info",
	"penunjang":  "rp.nama",
	"total":      "COALESCE(x.hjual, 0) * COALESCE(x.byk, 0)",
	"cabang":     "b.nama",
	"status":     statusSQL,
}

// RujukanSortable reports whether key is a Rujukan sort column.
func RujukanSortable(key string) bool {
	_, ok := rujukanSort[key]
	return ok
}

var rujukanFrom = fmt.Sprintf(` FROM trxjual j
	JOIN refmdokter dk ON dk.id = j.id_dok AND dk.id_internal = %d
	LEFT JOIN trxjualpnjng x ON x.id_jual = j.id
	LEFT JOIN refpenunjang rp ON rp.id = x.id_pnjng
	LEFT JOIN trxjualtotal t ON t.id_jual = j.id
	LEFT JOIN refmpasien p ON p.id = j.id_pas
	LEFT JOIN refjenisrawat r ON r.id = j.id_jenis_rwt
	LEFT JOIN refbranch b ON b.kode = j.cabang`, DokterEksternal)

func rujukanWhere(rp RujukanParams, withSearch bool) (string, []any) {
	args := []any{rp.Branches, rp.From, rp.To}
	conds := []string{`j.cabang = ANY($1)`, `j.tgl >= $2::date`, `j.tgl < $3::date + 1`}
	add := func(cond string, v any) {
		args = append(args, v)
		conds = append(conds, fmt.Sprintf(cond, len(args)))
	}
	if rp.Nama != "" {
		add(`p.nama ILIKE $%d`, "%"+escapeLike(rp.Nama)+"%")
	}
	if rp.Dokter != "" {
		add(`dk.nama ILIKE $%d`, "%"+escapeLike(rp.Dokter)+"%")
	}
	if rp.Status != "" {
		add(statusSQL+` = $%d`, rp.Status)
	}
	if withSearch && rp.Search != "" {
		args = append(args, "%"+escapeLike(rp.Search)+"%")
		conds = append(conds, fmt.Sprintf(`(p.nama ILIKE $%[1]d OR p.no_rm ILIKE $%[1]d OR dk.nama ILIKE $%[1]d OR r.info ILIKE $%[1]d
			OR rp.nama ILIKE $%[1]d OR b.nama ILIKE $%[1]d)`, len(args)))
	}
	return strings.Join(conds, " AND "), args
}

func (p *PostgresRepository) Rujukan(ctx context.Context, rp RujukanParams) (RujukanPage, error) {
	page := RujukanPage{Rows: []RujukanRow{}}
	col, ok := rujukanSort[rp.Sort]
	if !ok {
		col = "j.tgl"
	}
	dir := "ASC"
	if rp.Desc {
		dir = "DESC"
	}
	cond, args := rujukanWhere(rp, true)
	n := len(args)
	rows, err := p.pool.Query(ctx, fmt.Sprintf(`SELECT j.id, x.id, to_char(j.tgl, 'YYYY-MM-DD'), COALESCE(p.no_rm, ''), COALESCE(p.nama, ''),
			COALESCE(trim(dk.nama), ''), COALESCE(r.info, ''), COALESCE(rp.nama, CASE WHEN x.id IS NULL THEN '' ELSE '(penunjang ' || x.id_pnjng || ')' END),
			COALESCE(x.hjual, 0) * COALESCE(x.byk, 0), COALESCE(j.cabang, ''), `+statusSQL+`, count(*) OVER ()
		%s WHERE %s ORDER BY %s %s NULLS LAST, j.tgl %s, j.id %s, x.id LIMIT $%d OFFSET $%d`,
		rujukanFrom, cond, col, dir, dir, dir, n+1, n+2), append(args, rp.Limit, rp.Offset)...)
	if err != nil {
		return page, err
	}
	defer rows.Close()
	for rows.Next() {
		var x RujukanRow
		if err := rows.Scan(&x.IDJual, &x.IDLine, &x.Tgl, &x.NoRM, &x.Pasien, &x.Dokter, &x.JenisRawat, &x.Penunjang, &x.Total,
			&x.Cabang, &x.Status, &page.Total); err != nil {
			return page, err
		}
		page.Rows = append(page.Rows, x)
	}
	if err := rows.Err(); err != nil {
		return page, err
	}
	// An offset past the end returns no rows, and so no window count.
	if len(page.Rows) == 0 && rp.Offset > 0 {
		if err := p.pool.QueryRow(ctx, `SELECT count(*)`+rujukanFrom+` WHERE `+cond, args...).Scan(&page.Total); err != nil {
			return page, err
		}
	}
	if rp.Search == "" {
		page.UnfilteredTotal = page.Total
		return page, nil
	}
	cond, args = rujukanWhere(rp, false)
	err = p.pool.QueryRow(ctx, `SELECT count(*)`+rujukanFrom+` WHERE `+cond, args...).Scan(&page.UnfilteredTotal)
	return page, err
}
