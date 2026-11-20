package reportklinik

import (
	"context"
	"fmt"
	"strings"
)

// StokRow is one drug at one branch of All Stok, with its stock movement over
// the period.
//
// The app changes stock through sales (Save takes the drugs out, Cancel puts
// them back) and goods receipts (Penerimaan Barang adds them); stock
// adjustments are not recorded yet. Going back from today's stock:
//
//	Stok Akhir = current stock (trxstock.stok) + sold after the period − received after it
//	Jual Obat  = the drugs sold in the period (cancelled sales left out)
//	Stok Awal  = Stok Akhir + Jual Obat − the drugs received in the period
type StokRow struct {
	IDObat     int     `json:"idObat"` // Kode
	Obat       string  `json:"obat"`
	SatuanBeli string  `json:"satuanBeli"`
	IsiPerBox  string  `json:"isiPerBox"` // refobat.isi_per_box (text)
	SatuanJual string  `json:"satuanJual"`
	HargaBeli  float64 `json:"hargaBeli"` // trxstock.hbeli
	HargaJual  float64 `json:"hargaJual"` // trxstock.hjual
	Jenis      string  `json:"jenis"`
	Principal  string  `json:"principal"`
	Distrib    string  `json:"distributor"`
	MovingPO   string  `json:"movingPo"` // Kode Moving PO
	StokAwal   float64 `json:"stokAwal"`
	Jual       float64 `json:"jual"`
	StokAkhir  float64 `json:"stokAkhir"`
	Cabang     string  `json:"cabang"`
}

// StokParams selects one page of All Stok.
type StokParams struct {
	Branches   []string // always applied
	From, To   string   // YYYY-MM-DD, inclusive, clinic dates
	Search     string   // the table's search box
	Nama       string   // Cari: drug name
	IDJenis    *int     // Cari: refjenisobat id
	IDMovingPO *int     // Cari: refkodemovingpo id
	Sort       string
	Desc       bool
	Limit      int
	Offset     int
}

type StokPage struct {
	Rows            []StokRow `json:"rows"`
	Total           int       `json:"total"`
	UnfilteredTotal int       `json:"unfilteredTotal"`
}

const (
	stokAkhirSQL = `COALESCE(s.stok, 0) + COALESCE(sold.after, 0) - COALESCE(recv.after, 0)`
	stokAwalSQL  = `COALESCE(s.stok, 0) + COALESCE(sold.after, 0) - COALESCE(recv.after, 0) + COALESCE(sold.jual, 0) - COALESCE(recv.terima, 0)`
)

var stokSort = map[string]string{
	"idObat":      "s.id_obat",
	"obat":        "o.nama",
	"satuanBeli":  "sb.info",
	"isiPerBox":   "o.isi_per_box",
	"satuanJual":  "sj.info",
	"hargaBeli":   "s.hbeli",
	"hargaJual":   "s.hjual",
	"jenis":       "jo.info",
	"principal":   "pr.principal",
	"distributor": "di.distributor",
	"movingPo":    "mp.kodemovingpo",
	"stokAwal":    stokAwalSQL,
	"jual":        "COALESCE(sold.jual, 0)",
	"stokAkhir":   stokAkhirSQL,
	"cabang":      "b.nama",
}

// StokSortable reports whether key is an All Stok sort column.
func StokSortable(key string) bool {
	_, ok := stokSort[key]
	return ok
}

// stokFrom joins each stock row with its drug and with the drugs sold at its
// branch from the start of the period on: in the period (jual) and after it.
const stokFrom = ` FROM trxstock s
	JOIN refobat o ON o.id = s.id_obat
	LEFT JOIN refsatuan sb ON sb.id = o.sat_beli
	LEFT JOIN refsatuan sj ON sj.id = o.sat_jual
	LEFT JOIN refjenisobat jo ON jo.id = o.id_jenis
	LEFT JOIN refprincipalobat pr ON pr.id = o.id_principal
	LEFT JOIN refdistributorobat di ON di.id = o.id_distributor
	LEFT JOIN refkodemovingpo mp ON mp.id = o.id_moving_po
	LEFT JOIN refbranch b ON b.kode = s.cabang
	LEFT JOIN (
		SELECT j.cabang, x.id_obat,
			sum(COALESCE(x.byk, 0)) FILTER (WHERE j.tgl < $3::date + 1) AS jual,
			sum(COALESCE(x.byk, 0)) FILTER (WHERE j.tgl >= $3::date + 1) AS after
		FROM trxjual j
		JOIN trxjualobat x ON x.id_jual = j.id
		LEFT JOIN trxjualtotal t ON t.id_jual = j.id
		WHERE j.cabang = ANY($1) AND j.tgl >= $2::date AND COALESCE(t.status, 0) <> 2
		GROUP BY 1, 2
	) sold ON sold.cabang = s.cabang AND sold.id_obat = s.id_obat
	LEFT JOIN (
		SELECT f.cabang, fo.id_obat,
			sum(COALESCE(fo.tot_qjb, 0)) FILTER (WHERE f.tgl < $3::date + 1) AS terima,
			sum(COALESCE(fo.tot_qjb, 0)) FILTER (WHERE f.tgl >= $3::date + 1) AS after
		FROM trxfaktur f
		JOIN trxfakturobat fo ON fo.id_fak = f.id
		WHERE f.cabang = ANY($1) AND f.tgl >= $2::date
		GROUP BY 1, 2
	) recv ON recv.cabang = s.cabang AND recv.id_obat = s.id_obat`

func stokWhere(sp StokParams, withSearch bool) (string, []any) {
	args := []any{sp.Branches, sp.From, sp.To}
	conds := []string{`s.cabang = ANY($1)`}
	add := func(cond string, v any) {
		args = append(args, v)
		conds = append(conds, fmt.Sprintf(cond, len(args)))
	}
	if sp.Nama != "" {
		add(`o.nama ILIKE $%d`, "%"+escapeLike(sp.Nama)+"%")
	}
	if sp.IDJenis != nil {
		add(`o.id_jenis = $%d`, *sp.IDJenis)
	}
	if sp.IDMovingPO != nil {
		add(`o.id_moving_po = $%d`, *sp.IDMovingPO)
	}
	if withSearch && sp.Search != "" {
		args = append(args, "%"+escapeLike(sp.Search)+"%")
		n := len(args)
		cond := fmt.Sprintf(`o.nama ILIKE $%[1]d OR jo.info ILIKE $%[1]d OR pr.principal ILIKE $%[1]d OR di.distributor ILIKE $%[1]d
			OR b.nama ILIKE $%[1]d`, n)
		var id int
		if _, err := fmt.Sscanf(sp.Search, "%d", &id); err == nil && fmt.Sprint(id) == sp.Search {
			args = append(args, id)
			cond += fmt.Sprintf(` OR s.id_obat = $%d`, len(args))
		}
		conds = append(conds, `(`+cond+`)`)
	}
	return strings.Join(conds, " AND "), args
}

func (p *PostgresRepository) AllStok(ctx context.Context, sp StokParams) (StokPage, error) {
	page := StokPage{Rows: []StokRow{}}
	col, ok := stokSort[sp.Sort]
	if !ok {
		col = "s.hjual"
	}
	dir := "ASC"
	if sp.Desc {
		dir = "DESC"
	}
	cond, args := stokWhere(sp, true)
	n := len(args)
	rows, err := p.pool.Query(ctx, fmt.Sprintf(`SELECT s.id_obat, COALESCE(o.nama, ''), COALESCE(sb.info, ''), COALESCE(o.isi_per_box, ''),
			COALESCE(sj.info, ''), COALESCE(s.hbeli, 0), COALESCE(s.hjual, 0), COALESCE(jo.info, ''), COALESCE(pr.principal, ''),
			COALESCE(di.distributor, ''), COALESCE(mp.kodemovingpo, ''), `+stokAwalSQL+`, COALESCE(sold.jual, 0), `+stokAkhirSQL+`,
			COALESCE(s.cabang, ''), count(*) OVER ()
		%s WHERE %s ORDER BY %s %s NULLS LAST, o.nama, b.nama LIMIT $%d OFFSET $%d`,
		stokFrom, cond, col, dir, n+1, n+2), append(args, sp.Limit, sp.Offset)...)
	if err != nil {
		return page, err
	}
	defer rows.Close()
	for rows.Next() {
		var x StokRow
		if err := rows.Scan(&x.IDObat, &x.Obat, &x.SatuanBeli, &x.IsiPerBox, &x.SatuanJual, &x.HargaBeli, &x.HargaJual, &x.Jenis,
			&x.Principal, &x.Distrib, &x.MovingPO, &x.StokAwal, &x.Jual, &x.StokAkhir, &x.Cabang, &page.Total); err != nil {
			return page, err
		}
		page.Rows = append(page.Rows, x)
	}
	if err := rows.Err(); err != nil {
		return page, err
	}
	// An offset past the end returns no rows, and so no window count.
	if len(page.Rows) == 0 && sp.Offset > 0 {
		if err := p.pool.QueryRow(ctx, `SELECT count(*)`+stokFrom+` WHERE `+cond, args...).Scan(&page.Total); err != nil {
			return page, err
		}
	}
	if sp.Search == "" {
		page.UnfilteredTotal = page.Total
		return page, nil
	}
	cond, args = stokWhere(sp, false)
	err = p.pool.QueryRow(ctx, `SELECT count(*)`+stokFrom+` WHERE `+cond, args...).Scan(&page.UnfilteredTotal)
	return page, err
}

// MovingPO lists the PO moving codes (refkodemovingpo) for the Cari panel.
func (p *PostgresRepository) MovingPO(ctx context.Context) ([]Option, error) {
	return p.optionList(ctx, `SELECT id, COALESCE(kodemovingpo, '') FROM refkodemovingpo ORDER BY id`)
}
