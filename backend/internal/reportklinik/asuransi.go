package reportklinik

import (
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
)

// AsuransiLine is one item of an insured sale: a drug, procedure or
// examination with its quantity, price and Harga × Banyak.
type AsuransiLine struct {
	Nama   string  `json:"nama"`
	Byk    float64 `json:"byk"`
	Harga  float64 `json:"harga"`
	Jumlah float64 `json:"jumlah"`
}

// AsuransiSale is one sale of Rekap Asuransi: a sale with an insurer other
// than NON ASURANSI (trxjual.id_jamkes), with its items per group and diagnoses.
type AsuransiSale struct {
	IDJual     int            `json:"idJual"`
	Tgl        string         `json:"tgl"` // YYYY-MM-DD, clinic date
	NoRM       string         `json:"noRm"`
	NoKartu    string         `json:"noKartu"`    // Card Number (trxjual.no_jamkes)
	Perusahaan string         `json:"perusahaan"` // the insurer (refjaminankesehatan)
	Pasien     string         `json:"pasien"`
	Dokter     string         `json:"dokter"`
	Diagnosa   []string       `json:"diagnosa"` // ICD names
	Obat       []AsuransiLine `json:"obat"`
	Tindakan   []AsuransiLine `json:"tindakan"`
	Penunjang  []AsuransiLine `json:"penunjang"`
	Total      float64        `json:"total"` // trxjualtotal.total, the sum of the items
	Cabang     string         `json:"cabang"`
	Status     string         `json:"status"` // open, close or cancel
}

// AsuransiParams selects one page of Rekap Asuransi, counted in sales.
type AsuransiParams struct {
	Branches []string // always applied
	From, To string   // YYYY-MM-DD, inclusive, clinic dates
	Search   string   // the table's search box
	Nama     string   // Cari: patient name
	IDJamkes *int     // Cari: the insurer (refjaminankesehatan id)
	Status   string   // Cari: open, close, cancel or ""
	Sort     string
	Desc     bool
	Limit    int
	Offset   int
}

type AsuransiPage struct {
	Rows            []AsuransiSale `json:"rows"`
	Total           int            `json:"total"`
	UnfilteredTotal int            `json:"unfilteredTotal"`
}

var asuransiSort = map[string]string{
	"tgl":        "j.tgl",
	"noRm":       "p.no_rm",
	"noKartu":    "j.no_jamkes",
	"perusahaan": "jk.nama",
	"pasien":     "p.nama",
	"dokter":     "dk.nama",
	"total":      "t.total",
	"cabang":     "b.nama",
	"status":     statusSQL,
}

// AsuransiSortable reports whether key is a Rekap Asuransi sort column.
func AsuransiSortable(key string) bool {
	_, ok := asuransiSort[key]
	return ok
}

// NON ASURANSI (and no insurer at all) is not an insured sale.
const asuransiFrom = ` FROM trxjual j
	JOIN refjaminankesehatan jk ON jk.id = j.id_jamkes AND upper(trim(jk.nama)) <> 'NON ASURANSI'
	LEFT JOIN trxjualtotal t ON t.id_jual = j.id
	LEFT JOIN refmpasien p ON p.id = j.id_pas
	LEFT JOIN refmdokter dk ON dk.id = j.id_dok
	LEFT JOIN refbranch b ON b.kode = j.cabang`

func asuransiWhere(ap AsuransiParams, withSearch bool) (string, []any) {
	args := []any{ap.Branches, ap.From, ap.To}
	conds := []string{`j.cabang = ANY($1)`, `j.tgl >= $2::date`, `j.tgl < $3::date + 1`}
	add := func(cond string, v any) {
		args = append(args, v)
		conds = append(conds, fmt.Sprintf(cond, len(args)))
	}
	if ap.Nama != "" {
		add(`p.nama ILIKE $%d`, "%"+escapeLike(ap.Nama)+"%")
	}
	if ap.IDJamkes != nil {
		add(`j.id_jamkes = $%d`, *ap.IDJamkes)
	}
	if ap.Status != "" {
		add(statusSQL+` = $%d`, ap.Status)
	}
	if withSearch && ap.Search != "" {
		args = append(args, "%"+escapeLike(ap.Search)+"%")
		conds = append(conds, fmt.Sprintf(`(p.nama ILIKE $%[1]d OR p.no_rm ILIKE $%[1]d OR j.no_jamkes ILIKE $%[1]d OR jk.nama ILIKE $%[1]d
			OR dk.nama ILIKE $%[1]d OR b.nama ILIKE $%[1]d)`, len(args)))
	}
	return strings.Join(conds, " AND "), args
}

func (p *PostgresRepository) RekapAsuransi(ctx context.Context, ap AsuransiParams) (AsuransiPage, error) {
	page := AsuransiPage{Rows: []AsuransiSale{}}
	col, ok := asuransiSort[ap.Sort]
	if !ok {
		col = "j.tgl"
	}
	dir := "ASC"
	if ap.Desc {
		dir = "DESC"
	}
	cond, args := asuransiWhere(ap, true)
	n := len(args)
	rows, err := p.pool.Query(ctx, fmt.Sprintf(`SELECT j.id, to_char(j.tgl, 'YYYY-MM-DD'), COALESCE(p.no_rm, ''), COALESCE(j.no_jamkes, ''),
			COALESCE(jk.nama, ''), COALESCE(p.nama, ''), COALESCE(trim(dk.nama), ''), COALESCE(t.total, 0), COALESCE(j.cabang, ''),
			`+statusSQL+`, count(*) OVER ()
		%s WHERE %s ORDER BY %s %s NULLS LAST, j.tgl %s, j.id %s LIMIT $%d OFFSET $%d`,
		asuransiFrom, cond, col, dir, dir, dir, n+1, n+2), append(args, ap.Limit, ap.Offset)...)
	if err != nil {
		return page, err
	}
	ids := []int{}
	index := map[int]int{}
	for rows.Next() {
		x := AsuransiSale{Diagnosa: []string{}, Obat: []AsuransiLine{}, Tindakan: []AsuransiLine{}, Penunjang: []AsuransiLine{}}
		if err := rows.Scan(&x.IDJual, &x.Tgl, &x.NoRM, &x.NoKartu, &x.Perusahaan, &x.Pasien, &x.Dokter, &x.Total, &x.Cabang,
			&x.Status, &page.Total); err != nil {
			rows.Close()
			return page, err
		}
		index[x.IDJual] = len(page.Rows)
		ids = append(ids, x.IDJual)
		page.Rows = append(page.Rows, x)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return page, err
	}

	if len(ids) > 0 {
		// The page's items, per group, in the order they were entered.
		lines, err := p.pool.Query(ctx, `
			SELECT 'obat', x.id_jual, COALESCE(o.nama, '(obat ' || x.id_obat || ')'), COALESCE(x.byk, 0), COALESCE(x.hjual, 0), x.id
			  FROM trxjualobat x LEFT JOIN refobat o ON o.id = x.id_obat WHERE x.id_jual = ANY($1)
			UNION ALL
			SELECT 'tindakan', x.id_jual, COALESCE(r.nama, '(tindakan ' || x.id_tndk || ')'), COALESCE(x.byk, 0), COALESCE(x.hjual, 0), x.id
			  FROM trxjualtindakan x LEFT JOIN reftindakan r ON r.id = x.id_tndk WHERE x.id_jual = ANY($1)
			UNION ALL
			SELECT 'penunjang', x.id_jual, COALESCE(r.nama, '(penunjang ' || x.id_pnjng || ')'), COALESCE(x.byk, 0), COALESCE(x.hjual, 0), x.id
			  FROM trxjualpnjng x LEFT JOIN refpenunjang r ON r.id = x.id_pnjng WHERE x.id_jual = ANY($1)
			ORDER BY 6`, ids)
		if err != nil {
			return page, err
		}
		for lines.Next() {
			var group string
			var id int
			var l AsuransiLine
			if err := lines.Scan(&group, &id, &l.Nama, &l.Byk, &l.Harga, new(int)); err != nil {
				lines.Close()
				return page, err
			}
			l.Jumlah = l.Harga * l.Byk
			s := &page.Rows[index[id]]
			switch group {
			case "obat":
				s.Obat = append(s.Obat, l)
			case "tindakan":
				s.Tindakan = append(s.Tindakan, l)
			default:
				s.Penunjang = append(s.Penunjang, l)
			}
		}
		lines.Close()
		if err := lines.Err(); err != nil {
			return page, err
		}

		dx, err := p.pool.Query(ctx, `SELECT x.id_jual, COALESCE(i.nama, '(ICD ' || x.icd_id || ')')
			FROM trxjualdiagnosa x LEFT JOIN reficd i ON i.id = x.icd_id WHERE x.id_jual = ANY($1) ORDER BY x.id`, ids)
		if err != nil {
			return page, err
		}
		list, err := pgx.CollectRows(dx, func(r pgx.CollectableRow) ([2]any, error) {
			var id int
			var nama string
			err := r.Scan(&id, &nama)
			return [2]any{id, nama}, err
		})
		if err != nil {
			return page, err
		}
		for _, d := range list {
			s := &page.Rows[index[d[0].(int)]]
			s.Diagnosa = append(s.Diagnosa, d[1].(string))
		}
	}

	// An offset past the end returns no rows, and so no window count.
	if len(page.Rows) == 0 && ap.Offset > 0 {
		if err := p.pool.QueryRow(ctx, `SELECT count(*)`+asuransiFrom+` WHERE `+cond, args...).Scan(&page.Total); err != nil {
			return page, err
		}
	}
	if ap.Search == "" {
		page.UnfilteredTotal = page.Total
		return page, nil
	}
	cond, args = asuransiWhere(ap, false)
	err = p.pool.QueryRow(ctx, `SELECT count(*)`+asuransiFrom+` WHERE `+cond, args...).Scan(&page.UnfilteredTotal)
	return page, err
}

// Asuransi lists the insurers other than NON ASURANSI for the Cari panel.
func (p *PostgresRepository) Asuransi(ctx context.Context) ([]Option, error) {
	return p.optionList(ctx, `SELECT id, COALESCE(trim(nama), '') FROM refjaminankesehatan
		WHERE upper(trim(nama)) <> 'NON ASURANSI' ORDER BY nama, id`)
}
