package reportklinik

import (
	"context"
	"fmt"
	"strings"
)

// PasienBaruRow is one visit of Pasien Baru: a sale in the period whose
// patient was registered (refmpasien.create_date) in the same period. A new
// patient with two sales has two rows.
type PasienBaruRow struct {
	IDJual      int     `json:"idJual"`
	Tgl         string  `json:"tgl"` // Tanggal Kunjungan, "YYYY-MM-DD HH:MM:SS"
	NoRM        string  `json:"noRm"`
	Pasien      string  `json:"pasien"`
	JenisRawat  string  `json:"jenisRawat"`
	TempatLahir string  `json:"tempatLahir"`
	TglLahir    *string `json:"tglLahir"` // YYYY-MM-DD
	Umur        *int    `json:"umur"`     // years, on the visit day
	Alamat      string  `json:"alamat"`
	TglDaftar   string  `json:"tglDaftar"` // the day the patient was registered
	Dokter      string  `json:"dokter"`
	Status      string  `json:"status"` // the sale's: open, close or cancel
	Cabang      string  `json:"cabang"`
}

// PasienBaruParams selects one page of Pasien Baru.
type PasienBaruParams struct {
	Branches     []string // always applied
	From, To     string   // YYYY-MM-DD, inclusive, clinic dates
	Search       string   // the table's search box
	Nama         string   // Cari: patient name
	IDJenisRawat *int     // Cari: refjenisrawat id
	Status       string   // Cari: open, close, cancel or ""
	Sort         string
	Desc         bool
	Limit        int
	Offset       int
}

type PasienBaruPage struct {
	Rows            []PasienBaruRow `json:"rows"`
	Total           int             `json:"total"`
	UnfilteredTotal int             `json:"unfilteredTotal"`
}

var pasienBaruSort = map[string]string{
	"tgl":         "j.tgl",
	"noRm":        "p.no_rm",
	"pasien":      "p.nama",
	"jenisRawat":  "r.info",
	"tempatLahir": "p.tmpt_lahir",
	"tglLahir":    "p.tgl_lahir",
	"umur":        "p.tgl_lahir", // older birth date = higher age; the handler flips the direction
	"alamat":      "p.alamat",
	"tglDaftar":   "p.create_date",
	"dokter":      "dk.nama",
	"status":      statusSQL,
	"cabang":      "b.nama",
}

// PasienBaruSortable reports whether key is a Pasien Baru sort column.
func PasienBaruSortable(key string) bool {
	_, ok := pasienBaruSort[key]
	return ok
}

const pasienBaruFrom = ` FROM trxjual j
	JOIN refmpasien p ON p.id = j.id_pas
	LEFT JOIN trxjualtotal t ON t.id_jual = j.id
	LEFT JOIN refjenisrawat r ON r.id = j.id_jenis_rwt
	LEFT JOIN refmdokter dk ON dk.id = j.id_dok
	LEFT JOIN refbranch b ON b.kode = j.cabang`

func pasienBaruWhere(pp PasienBaruParams, withSearch bool) (string, []any) {
	args := []any{pp.Branches, pp.From, pp.To}
	conds := []string{
		`j.cabang = ANY($1)`, `j.tgl >= $2::date`, `j.tgl < $3::date + 1`,
		// Registered in the same period: a new patient.
		`p.create_date >= $2::date`, `p.create_date < $3::date + 1`,
	}
	add := func(cond string, v any) {
		args = append(args, v)
		conds = append(conds, fmt.Sprintf(cond, len(args)))
	}
	if pp.Nama != "" {
		add(`p.nama ILIKE $%d`, "%"+escapeLike(pp.Nama)+"%")
	}
	if pp.IDJenisRawat != nil {
		add(`j.id_jenis_rwt = $%d`, *pp.IDJenisRawat)
	}
	if pp.Status != "" {
		add(statusSQL+` = $%d`, pp.Status)
	}
	if withSearch && pp.Search != "" {
		args = append(args, "%"+escapeLike(pp.Search)+"%")
		conds = append(conds, fmt.Sprintf(`(p.nama ILIKE $%[1]d OR p.no_rm ILIKE $%[1]d OR p.alamat ILIKE $%[1]d OR p.tmpt_lahir ILIKE $%[1]d
			OR r.info ILIKE $%[1]d OR dk.nama ILIKE $%[1]d OR b.nama ILIKE $%[1]d)`, len(args)))
	}
	return strings.Join(conds, " AND "), args
}

func (p *PostgresRepository) PasienBaru(ctx context.Context, pp PasienBaruParams) (PasienBaruPage, error) {
	page := PasienBaruPage{Rows: []PasienBaruRow{}}
	col, ok := pasienBaruSort[pp.Sort]
	if !ok {
		col = "j.tgl"
	}
	dir := "ASC"
	if pp.Desc {
		dir = "DESC"
	}
	cond, args := pasienBaruWhere(pp, true)
	n := len(args)
	rows, err := p.pool.Query(ctx, fmt.Sprintf(`SELECT j.id, to_char(j.tgl, 'YYYY-MM-DD HH24:MI:SS'), COALESCE(p.no_rm, ''), COALESCE(p.nama, ''),
			COALESCE(r.info, ''), COALESCE(p.tmpt_lahir, ''), to_char(p.tgl_lahir, 'YYYY-MM-DD'),
			date_part('year', age(j.tgl::date, p.tgl_lahir))::int, COALESCE(p.alamat, ''), to_char(p.create_date, 'YYYY-MM-DD'),
			COALESCE(trim(dk.nama), ''), `+statusSQL+`, COALESCE(j.cabang, ''), count(*) OVER ()
		%s WHERE %s ORDER BY %s %s NULLS LAST, j.tgl %s, j.id %s LIMIT $%d OFFSET $%d`,
		pasienBaruFrom, cond, col, dir, dir, dir, n+1, n+2), append(args, pp.Limit, pp.Offset)...)
	if err != nil {
		return page, err
	}
	defer rows.Close()
	for rows.Next() {
		var x PasienBaruRow
		if err := rows.Scan(&x.IDJual, &x.Tgl, &x.NoRM, &x.Pasien, &x.JenisRawat, &x.TempatLahir, &x.TglLahir, &x.Umur, &x.Alamat,
			&x.TglDaftar, &x.Dokter, &x.Status, &x.Cabang, &page.Total); err != nil {
			return page, err
		}
		page.Rows = append(page.Rows, x)
	}
	if err := rows.Err(); err != nil {
		return page, err
	}
	// An offset past the end returns no rows, and so no window count.
	if len(page.Rows) == 0 && pp.Offset > 0 {
		if err := p.pool.QueryRow(ctx, `SELECT count(*)`+pasienBaruFrom+` WHERE `+cond, args...).Scan(&page.Total); err != nil {
			return page, err
		}
	}
	if pp.Search == "" {
		page.UnfilteredTotal = page.Total
		return page, nil
	}
	cond, args = pasienBaruWhere(pp, false)
	err = p.pool.QueryRow(ctx, `SELECT count(*)`+pasienBaruFrom+` WHERE `+cond, args...).Scan(&page.UnfilteredTotal)
	return page, err
}

// JenisRawat lists the services (refjenisrawat) for the Cari panel.
func (p *PostgresRepository) JenisRawat(ctx context.Context) ([]Option, error) {
	return p.optionList(ctx, `SELECT id, COALESCE(info, '') FROM refjenisrawat ORDER BY info, id`)
}
