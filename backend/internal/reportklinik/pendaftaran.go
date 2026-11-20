package reportklinik

import (
	"context"
	"fmt"
	"strings"
)

// PendaftaranRow is one registration (trxpendaftaran) of the Pendaftaran report.
type PendaftaranRow struct {
	ID        int     `json:"id"`      // No Pendaftaran
	NoAntri   *int    `json:"noAntri"` // No Antrian
	Tgl       *string `json:"tgl"`     // Tanggal / Waktu, "YYYY-MM-DD HH:MM:SS"
	Pelayanan string  `json:"pelayanan"`
	StatusKun string  `json:"statusKun"` // refstatustransaksi, e.g. CLOSE, DAFTAR
	NoRM      string  `json:"noRm"`
	Pasien    string  `json:"pasien"`
	Umur      *int    `json:"umur"` // years, on the registration day
	Alamat    string  `json:"alamat"`
	Cabang    string  `json:"cabang"`
}

// PendaftaranParams selects one page of the Pendaftaran report.
type PendaftaranParams struct {
	Branches     []string // always applied
	From, To     string   // YYYY-MM-DD, inclusive, clinic dates
	Search       string   // the table's search box
	Nama         string   // Cari: patient name
	IDJenisRawat *int     // Cari: refjenisrawat id
	IDStatus     *int     // Cari: refstatustransaksi id
	Sort         string
	Desc         bool
	Limit        int
	Offset       int
}

type PendaftaranPage struct {
	Rows            []PendaftaranRow `json:"rows"`
	Total           int              `json:"total"`
	UnfilteredTotal int              `json:"unfilteredTotal"`
}

var pendaftaranSort = map[string]string{
	"id":        "d.id",
	"noAntri":   "d.no_antri",
	"tgl":       "d.tgl_masuk",
	"pelayanan": "r.info",
	"statusKun": "s.info",
	"noRm":      "p.no_rm",
	"pasien":    "p.nama",
	"umur":      "p.tgl_lahir", // older birth date = higher age; the handler flips the direction
	"alamat":    "p.alamat",
	"cabang":    "b.nama",
}

// PendaftaranSortable reports whether key is a Pendaftaran report sort column.
func PendaftaranSortable(key string) bool {
	_, ok := pendaftaranSort[key]
	return ok
}

const pendaftaranFrom = ` FROM trxpendaftaran d
	LEFT JOIN refmpasien p ON p.id = d.id_pas
	LEFT JOIN refjenisrawat r ON r.id = d.id_jenis_rwt
	LEFT JOIN refstatustransaksi s ON s.id = d.id_statustr
	LEFT JOIN refbranch b ON b.kode = d.cabang`

func pendaftaranWhere(pp PendaftaranParams, withSearch bool) (string, []any) {
	args := []any{pp.Branches, pp.From, pp.To}
	conds := []string{`d.cabang = ANY($1)`, `d.tgl_masuk >= $2::date`, `d.tgl_masuk < $3::date + 1`}
	add := func(cond string, v any) {
		args = append(args, v)
		conds = append(conds, fmt.Sprintf(cond, len(args)))
	}
	if pp.Nama != "" {
		add(`p.nama ILIKE $%d`, "%"+escapeLike(pp.Nama)+"%")
	}
	if pp.IDJenisRawat != nil {
		add(`d.id_jenis_rwt = $%d`, *pp.IDJenisRawat)
	}
	if pp.IDStatus != nil {
		add(`d.id_statustr = $%d`, *pp.IDStatus)
	}
	if withSearch && pp.Search != "" {
		args = append(args, "%"+escapeLike(pp.Search)+"%")
		n := len(args)
		cond := fmt.Sprintf(`p.nama ILIKE $%[1]d OR p.no_rm ILIKE $%[1]d OR p.alamat ILIKE $%[1]d OR r.info ILIKE $%[1]d
			OR s.info ILIKE $%[1]d OR b.nama ILIKE $%[1]d`, n)
		var id int
		if _, err := fmt.Sscanf(pp.Search, "%d", &id); err == nil && fmt.Sprint(id) == pp.Search {
			args = append(args, id)
			cond += fmt.Sprintf(` OR d.id = $%d`, len(args))
		}
		conds = append(conds, `(`+cond+`)`)
	}
	return strings.Join(conds, " AND "), args
}

func (p *PostgresRepository) Pendaftaran(ctx context.Context, pp PendaftaranParams) (PendaftaranPage, error) {
	page := PendaftaranPage{Rows: []PendaftaranRow{}}
	col, ok := pendaftaranSort[pp.Sort]
	if !ok {
		col = "d.id"
	}
	dir := "ASC"
	if pp.Desc {
		dir = "DESC"
	}
	cond, args := pendaftaranWhere(pp, true)
	n := len(args)
	rows, err := p.pool.Query(ctx, fmt.Sprintf(`SELECT d.id, d.no_antri, to_char(d.tgl_masuk, 'YYYY-MM-DD HH24:MI:SS'), COALESCE(r.info, ''),
			COALESCE(s.info, ''), COALESCE(p.no_rm, ''), COALESCE(p.nama, ''), date_part('year', age(d.tgl_masuk::date, p.tgl_lahir))::int,
			COALESCE(p.alamat, ''), COALESCE(d.cabang, ''), count(*) OVER ()
		%s WHERE %s ORDER BY %s %s NULLS LAST, d.id %s LIMIT $%d OFFSET $%d`,
		pendaftaranFrom, cond, col, dir, dir, n+1, n+2), append(args, pp.Limit, pp.Offset)...)
	if err != nil {
		return page, err
	}
	defer rows.Close()
	for rows.Next() {
		var x PendaftaranRow
		if err := rows.Scan(&x.ID, &x.NoAntri, &x.Tgl, &x.Pelayanan, &x.StatusKun, &x.NoRM, &x.Pasien, &x.Umur, &x.Alamat, &x.Cabang,
			&page.Total); err != nil {
			return page, err
		}
		page.Rows = append(page.Rows, x)
	}
	if err := rows.Err(); err != nil {
		return page, err
	}
	// An offset past the end returns no rows, and so no window count.
	if len(page.Rows) == 0 && pp.Offset > 0 {
		if err := p.pool.QueryRow(ctx, `SELECT count(*)`+pendaftaranFrom+` WHERE `+cond, args...).Scan(&page.Total); err != nil {
			return page, err
		}
	}
	if pp.Search == "" {
		page.UnfilteredTotal = page.Total
		return page, nil
	}
	cond, args = pendaftaranWhere(pp, false)
	err = p.pool.QueryRow(ctx, `SELECT count(*)`+pendaftaranFrom+` WHERE `+cond, args...).Scan(&page.UnfilteredTotal)
	return page, err
}

// StatusKunjungan lists the visit statuses (refstatustransaksi) for the Cari panel.
func (p *PostgresRepository) StatusKunjungan(ctx context.Context) ([]Option, error) {
	return p.optionList(ctx, `SELECT id, COALESCE(info, '') FROM refstatustransaksi ORDER BY id`)
}
