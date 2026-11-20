package reportklinik

import (
	"context"
	"fmt"
	"strings"
)

// DiagnosaRow is one diagnosis of a sale (trxjualdiagnosa) in Report Diagnosa,
// with the patient, the sale's prescription and the visit's main complaint
// (trxpemeriksaan). A sale with two diagnoses has two rows.
type DiagnosaRow struct {
	ID           int     `json:"id"` // trxjualdiagnosa id
	IDJual       int     `json:"idJual"`
	Tgl          string  `json:"tgl"` // "YYYY-MM-DD HH:MM:SS"
	NoRM         string  `json:"noRm"`
	Pasien       string  `json:"pasien"`
	TglLahir     *string `json:"tglLahir"` // YYYY-MM-DD
	Usia         *int    `json:"usia"`     // years, on the visit day
	JenisKelamin string  `json:"jenisKelamin"`
	Dokter       string  `json:"dokter"`
	JenisRawat   string  `json:"jenisRawat"`
	ICD          string  `json:"icd"`      // ICD.X code
	Diagnosa     string  `json:"diagnosa"` // ICD name
	Resep        string  `json:"resep"`    // "DRUG (qty) , DRUG (qty)"
	Keluhan      string  `json:"keluhan"`  // trxpemeriksaan.keluhan_utama
	Cabang       string  `json:"cabang"`
}

// DiagnosaParams selects one page of Report Diagnosa.
type DiagnosaParams struct {
	Branches     []string // always applied
	From, To     string   // YYYY-MM-DD, inclusive, clinic dates
	Search       string   // the table's search box
	Nama         string   // Cari: patient name
	Diagnosa     string   // Cari: ICD code or name
	IDJenisRawat *int     // Cari: refjenisrawat id
	Sort         string
	Desc         bool
	Limit        int
	Offset       int
}

type DiagnosaPage struct {
	Rows            []DiagnosaRow `json:"rows"`
	Total           int           `json:"total"`
	UnfilteredTotal int           `json:"unfilteredTotal"`
}

var diagnosaSort = map[string]string{
	"idJual":       "j.id",
	"tgl":          "j.tgl",
	"noRm":         "p.no_rm",
	"pasien":       "p.nama",
	"tglLahir":     "p.tgl_lahir",
	"usia":         "p.tgl_lahir", // older birth date = higher age; the handler flips the direction
	"jenisKelamin": "g.info",
	"dokter":       "dk.nama",
	"jenisRawat":   "r.info",
	"icd":          "i.kode_icd",
	"diagnosa":     "i.nama",
	"cabang":       "b.nama",
}

// DiagnosaSortable reports whether key is a Report Diagnosa sort column.
func DiagnosaSortable(key string) bool {
	_, ok := diagnosaSort[key]
	return ok
}

const diagnosaFrom = ` FROM trxjualdiagnosa x
	JOIN trxjual j ON j.id = x.id_jual
	LEFT JOIN reficd i ON i.id = x.icd_id
	LEFT JOIN refmpasien p ON p.id = j.id_pas
	LEFT JOIN refgender g ON g.id = p.id_gender
	LEFT JOIN refmdokter dk ON dk.id = j.id_dok
	LEFT JOIN refjenisrawat r ON r.id = j.id_jenis_rwt
	LEFT JOIN trxpemeriksaan pm ON pm.id_kun = j.id_kun
	LEFT JOIN refbranch b ON b.kode = j.cabang`

func diagnosaWhere(dp DiagnosaParams, withSearch bool) (string, []any) {
	args := []any{dp.Branches, dp.From, dp.To}
	conds := []string{`j.cabang = ANY($1)`, `j.tgl >= $2::date`, `j.tgl < $3::date + 1`}
	add := func(cond string, v any) {
		args = append(args, v)
		conds = append(conds, fmt.Sprintf(cond, len(args)))
	}
	if dp.Nama != "" {
		add(`p.nama ILIKE $%d`, "%"+escapeLike(dp.Nama)+"%")
	}
	if dp.Diagnosa != "" {
		args = append(args, "%"+escapeLike(dp.Diagnosa)+"%")
		conds = append(conds, fmt.Sprintf(`(i.kode_icd ILIKE $%[1]d OR i.nama ILIKE $%[1]d)`, len(args)))
	}
	if dp.IDJenisRawat != nil {
		add(`j.id_jenis_rwt = $%d`, *dp.IDJenisRawat)
	}
	if withSearch && dp.Search != "" {
		args = append(args, "%"+escapeLike(dp.Search)+"%")
		n := len(args)
		cond := fmt.Sprintf(`p.nama ILIKE $%[1]d OR p.no_rm ILIKE $%[1]d OR i.kode_icd ILIKE $%[1]d OR i.nama ILIKE $%[1]d
			OR dk.nama ILIKE $%[1]d OR pm.keluhan_utama ILIKE $%[1]d OR b.nama ILIKE $%[1]d`, n)
		var id int
		if _, err := fmt.Sscanf(dp.Search, "%d", &id); err == nil && fmt.Sprint(id) == dp.Search {
			args = append(args, id)
			cond += fmt.Sprintf(` OR j.id = $%d`, len(args))
		}
		conds = append(conds, `(`+cond+`)`)
	}
	return strings.Join(conds, " AND "), args
}

func (p *PostgresRepository) ReportDiagnosa(ctx context.Context, dp DiagnosaParams) (DiagnosaPage, error) {
	page := DiagnosaPage{Rows: []DiagnosaRow{}}
	col, ok := diagnosaSort[dp.Sort]
	if !ok {
		col = "j.id"
	}
	dir := "ASC"
	if dp.Desc {
		dir = "DESC"
	}
	cond, args := diagnosaWhere(dp, true)
	n := len(args)
	// The prescription is built for the page's rows only (LATERAL after LIMIT).
	rows, err := p.pool.Query(ctx, fmt.Sprintf(`SELECT d.*, COALESCE(rx.resep, '') FROM (
			SELECT x.id, j.id AS id_jual, to_char(j.tgl, 'YYYY-MM-DD HH24:MI:SS'), COALESCE(p.no_rm, ''), COALESCE(p.nama, ''),
				to_char(p.tgl_lahir, 'YYYY-MM-DD'), date_part('year', age(j.tgl::date, p.tgl_lahir))::int, COALESCE(g.info, ''),
				COALESCE(trim(dk.nama), ''), COALESCE(r.info, ''), COALESCE(i.kode_icd, ''), COALESCE(i.nama, '(ICD ' || x.icd_id || ')'),
				COALESCE(pm.keluhan_utama, ''), COALESCE(j.cabang, ''), count(*) OVER ()
			%s WHERE %s ORDER BY %s %s NULLS LAST, j.id %s, x.id LIMIT $%d OFFSET $%d) d
		LEFT JOIN LATERAL (
			SELECT string_agg(COALESCE(o.nama, '(obat ' || ob.id_obat || ')') || ' (' || COALESCE(ob.byk, 0)::text || ')', ' , ' ORDER BY ob.id) AS resep
			FROM trxjualobat ob LEFT JOIN refobat o ON o.id = ob.id_obat WHERE ob.id_jual = d.id_jual
		) rx ON true`,
		diagnosaFrom, cond, col, dir, dir, n+1, n+2), append(args, dp.Limit, dp.Offset)...)
	if err != nil {
		return page, err
	}
	defer rows.Close()
	for rows.Next() {
		var x DiagnosaRow
		if err := rows.Scan(&x.ID, &x.IDJual, &x.Tgl, &x.NoRM, &x.Pasien, &x.TglLahir, &x.Usia, &x.JenisKelamin, &x.Dokter, &x.JenisRawat,
			&x.ICD, &x.Diagnosa, &x.Keluhan, &x.Cabang, &page.Total, &x.Resep); err != nil {
			return page, err
		}
		page.Rows = append(page.Rows, x)
	}
	if err := rows.Err(); err != nil {
		return page, err
	}
	// An offset past the end returns no rows, and so no window count.
	if len(page.Rows) == 0 && dp.Offset > 0 {
		if err := p.pool.QueryRow(ctx, `SELECT count(*)`+diagnosaFrom+` WHERE `+cond, args...).Scan(&page.Total); err != nil {
			return page, err
		}
	}
	if dp.Search == "" {
		page.UnfilteredTotal = page.Total
		return page, nil
	}
	cond, args = diagnosaWhere(dp, false)
	err = p.pool.QueryRow(ctx, `SELECT count(*)`+diagnosaFrom+` WHERE `+cond, args...).Scan(&page.UnfilteredTotal)
	return page, err
}
