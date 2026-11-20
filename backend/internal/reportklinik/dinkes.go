package reportklinik

import (
	"context"
	"fmt"
	"strings"
)

// DinkesRow is one line of Report ICD Dinkes: the cases (diagnoses of
// non-cancelled sales) of one ICD code and service in the period, by the
// patient's age on the visit day and by sex.
type DinkesRow struct {
	Kode       string `json:"kode"` // ICD.X code
	ICD        string `json:"icd"`  // ICD name
	JenisRawat string `json:"jenisRawat"`
	Hari0_7    int    `json:"hari0_7"`
	Hari8_28   int    `json:"hari8_28"`
	Bulan1_11  int    `json:"bulan1_11"`
	Tahun1_4   int    `json:"tahun1_4"`
	Tahun5_9   int    `json:"tahun5_9"`
	Tahun10_14 int    `json:"tahun10_14"`
	Tahun15_19 int    `json:"tahun15_19"`
	Tahun20_44 int    `json:"tahun20_44"`
	Tahun45_59 int    `json:"tahun45_59"`
	Tahun60    int    `json:"tahun60"` // > 59 years
	Laki       int    `json:"laki"`
	Perempuan  int    `json:"perempuan"`
	Total      int    `json:"total"`
}

// DinkesParams selects one page of Report ICD Dinkes.
type DinkesParams struct {
	Branches []string // always applied (the report is usually asked for one branch)
	From, To string   // YYYY-MM-DD, inclusive, clinic dates
	Search   string   // the table's search box: ICD code or name, service
	Sort     string
	Desc     bool
	Limit    int
	Offset   int
}

type DinkesPage struct {
	Rows            []DinkesRow `json:"rows"`
	Total           int         `json:"total"`
	UnfilteredTotal int         `json:"unfilteredTotal"`
}

var dinkesSort = map[string]string{
	"kode": "g.kode", "icd": "g.icd", "jenisRawat": "g.jenis_rawat",
	"hari0_7": "g.h0", "hari8_28": "g.h8", "bulan1_11": "g.b1", "tahun1_4": "g.t1", "tahun5_9": "g.t5", "tahun10_14": "g.t10",
	"tahun15_19": "g.t15", "tahun20_44": "g.t20", "tahun45_59": "g.t45", "tahun60": "g.t60",
	"laki": "g.laki", "perempuan": "g.perempuan", "total": "g.total",
}

// DinkesSortable reports whether key is a Report ICD Dinkes sort column.
func DinkesSortable(key string) bool {
	_, ok := dinkesSort[key]
	return ok
}

// dinkesGrouped counts the cases per ICD code and service. hari is the
// patient's age in days and tahun in whole years, both on the visit day; a
// patient without a birth date is counted in no age group (but in the total).
func dinkesGrouped(dp DinkesParams, withSearch bool) (string, []any) {
	args := []any{dp.Branches, dp.From, dp.To}
	conds := []string{`j.cabang = ANY($1)`, `j.tgl >= $2::date`, `j.tgl < $3::date + 1`, `COALESCE(t.status, 0) <> 2`}
	if withSearch && dp.Search != "" {
		args = append(args, "%"+escapeLike(dp.Search)+"%")
		conds = append(conds, fmt.Sprintf(`(i.kode_icd ILIKE $%[1]d OR i.nama ILIKE $%[1]d OR r.info ILIKE $%[1]d)`, len(args)))
	}
	return `FROM (
		SELECT COALESCE(c.kode_icd, '') AS kode, c.icd, c.jenis_rawat,
			count(*) FILTER (WHERE c.hari BETWEEN 0 AND 7) AS h0,
			count(*) FILTER (WHERE c.hari BETWEEN 8 AND 28) AS h8,
			count(*) FILTER (WHERE c.hari > 28 AND c.tahun < 1) AS b1,
			count(*) FILTER (WHERE c.tahun BETWEEN 1 AND 4) AS t1,
			count(*) FILTER (WHERE c.tahun BETWEEN 5 AND 9) AS t5,
			count(*) FILTER (WHERE c.tahun BETWEEN 10 AND 14) AS t10,
			count(*) FILTER (WHERE c.tahun BETWEEN 15 AND 19) AS t15,
			count(*) FILTER (WHERE c.tahun BETWEEN 20 AND 44) AS t20,
			count(*) FILTER (WHERE c.tahun BETWEEN 45 AND 59) AS t45,
			count(*) FILTER (WHERE c.tahun >= 60) AS t60,
			count(*) FILTER (WHERE c.gender = 1) AS laki,
			count(*) FILTER (WHERE c.gender = 2) AS perempuan,
			count(*) AS total
		FROM (
			SELECT i.kode_icd, COALESCE(i.nama, '(ICD ' || x.icd_id || ')') AS icd, COALESCE(r.info, '') AS jenis_rawat,
				j.tgl::date - p.tgl_lahir AS hari, date_part('year', age(j.tgl::date, p.tgl_lahir)) AS tahun, p.id_gender AS gender
			FROM trxjualdiagnosa x
			JOIN trxjual j ON j.id = x.id_jual
			LEFT JOIN trxjualtotal t ON t.id_jual = j.id
			LEFT JOIN reficd i ON i.id = x.icd_id
			LEFT JOIN refjenisrawat r ON r.id = j.id_jenis_rwt
			LEFT JOIN refmpasien p ON p.id = j.id_pas
			WHERE ` + strings.Join(conds, " AND ") + `
		) c
		GROUP BY 1, 2, 3
	) g`, args
}

func (p *PostgresRepository) ReportDinkes(ctx context.Context, dp DinkesParams) (DinkesPage, error) {
	page := DinkesPage{Rows: []DinkesRow{}}
	col, ok := dinkesSort[dp.Sort]
	if !ok {
		col = "g.kode"
	}
	dir := "ASC"
	if dp.Desc {
		dir = "DESC"
	}
	from, args := dinkesGrouped(dp, true)
	n := len(args)
	rows, err := p.pool.Query(ctx, fmt.Sprintf(`SELECT g.kode, g.icd, g.jenis_rawat, g.h0, g.h8, g.b1, g.t1, g.t5, g.t10, g.t15, g.t20, g.t45, g.t60,
			g.laki, g.perempuan, g.total, count(*) OVER ()
		%s ORDER BY %s %s NULLS LAST, g.kode %s, g.jenis_rawat LIMIT $%d OFFSET $%d`, from, col, dir, dir, n+1, n+2),
		append(args, dp.Limit, dp.Offset)...)
	if err != nil {
		return page, err
	}
	defer rows.Close()
	for rows.Next() {
		var x DinkesRow
		if err := rows.Scan(&x.Kode, &x.ICD, &x.JenisRawat, &x.Hari0_7, &x.Hari8_28, &x.Bulan1_11, &x.Tahun1_4, &x.Tahun5_9, &x.Tahun10_14,
			&x.Tahun15_19, &x.Tahun20_44, &x.Tahun45_59, &x.Tahun60, &x.Laki, &x.Perempuan, &x.Total, &page.Total); err != nil {
			return page, err
		}
		page.Rows = append(page.Rows, x)
	}
	if err := rows.Err(); err != nil {
		return page, err
	}
	// An offset past the end returns no rows, and so no window count.
	if len(page.Rows) == 0 && dp.Offset > 0 {
		if err := p.pool.QueryRow(ctx, `SELECT count(*) `+from, args...).Scan(&page.Total); err != nil {
			return page, err
		}
	}
	if dp.Search == "" {
		page.UnfilteredTotal = page.Total
		return page, nil
	}
	from, args = dinkesGrouped(dp, false)
	err = p.pool.QueryRow(ctx, `SELECT count(*) `+from, args...).Scan(&page.UnfilteredTotal)
	return page, err
}
