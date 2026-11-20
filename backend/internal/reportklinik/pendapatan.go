package reportklinik

import (
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
)

// PendapatanRow is one sale of the Report Pendapatan PDF.
type PendapatanRow struct {
	IDJual    int     `json:"idJual"`
	Tgl       string  `json:"tgl"` // Tanggal / Waktu, "YYYY-MM-DD HH:MM:SS.mmm"
	NoKwi     string  `json:"noKwi"`
	Pasien    string  `json:"pasien"`
	Obat      float64 `json:"obat"`
	Tindakan  float64 `json:"tindakan"`
	Penunjang float64 `json:"penunjang"`
	Total     float64 `json:"total"`
	Potongan  float64 `json:"potongan"`
	Baru      bool    `json:"baru"` // the patient was registered in the period
}

// Pendapatan is the Report Pendapatan of one branch: its sales in the period
// (cancelled sales left out, they are no income) with the header the PDF prints.
type Pendapatan struct {
	Cabang     string          `json:"cabang"`
	NamaCabang string          `json:"namaCabang"`
	TelpCabang string          `json:"telpCabang"`
	Rows       []PendapatanRow `json:"rows"`
}

// PendapatanParams selects the sales of Report Pendapatan.
type PendapatanParams struct {
	Cabang       string // one branch, required
	From, To     string // YYYY-MM-DD, inclusive, clinic dates
	IDJenisRawat *int   // Layanan; nil = all
	IDDokter     *int   // Dokter PJ; nil = all
}

func (p *PostgresRepository) Pendapatan(ctx context.Context, pp PendapatanParams) (Pendapatan, error) {
	out := Pendapatan{Cabang: pp.Cabang, Rows: []PendapatanRow{}}
	if err := p.pool.QueryRow(ctx, `SELECT COALESCE(nama, kode), COALESCE(NULLIF(telp, '-'), '') FROM refbranch WHERE kode = $1`,
		pp.Cabang).Scan(&out.NamaCabang, &out.TelpCabang); err != nil && err != pgx.ErrNoRows {
		return out, err
	}
	args := []any{pp.Cabang, pp.From, pp.To}
	conds := []string{`j.cabang = $1`, `j.tgl >= $2::date`, `j.tgl < $3::date + 1`, `COALESCE(t.status, 0) <> 2`}
	if pp.IDJenisRawat != nil {
		args = append(args, *pp.IDJenisRawat)
		conds = append(conds, fmt.Sprintf(`j.id_jenis_rwt = $%d`, len(args)))
	}
	if pp.IDDokter != nil {
		args = append(args, *pp.IDDokter)
		conds = append(conds, fmt.Sprintf(`j.id_dok = $%d`, len(args)))
	}
	rows, err := p.pool.Query(ctx, `SELECT j.id, to_char(j.tgl, 'YYYY-MM-DD HH24:MI:SS.MS'), COALESCE(j.no_kwi, ''), COALESCE(p.nama, ''),
			COALESCE(t.tot_obat, 0), COALESCE(t.tot_tndk, 0), COALESCE(t.tot_pnjng, 0), COALESCE(t.total, 0), COALESCE(t.potongan, 0),
			COALESCE(p.create_date >= $2::date AND p.create_date < $3::date + 1, false)
		FROM trxjual j
		LEFT JOIN trxjualtotal t ON t.id_jual = j.id
		LEFT JOIN refmpasien p ON p.id = j.id_pas
		WHERE `+strings.Join(conds, " AND ")+`
		ORDER BY j.tgl, j.id`, args...)
	if err != nil {
		return out, err
	}
	list, err := pgx.CollectRows(rows, func(r pgx.CollectableRow) (PendapatanRow, error) {
		var x PendapatanRow
		err := r.Scan(&x.IDJual, &x.Tgl, &x.NoKwi, &x.Pasien, &x.Obat, &x.Tindakan, &x.Penunjang, &x.Total, &x.Potongan, &x.Baru)
		return x, err
	})
	if err != nil {
		return out, err
	}
	if list != nil {
		out.Rows = list
	}
	return out, nil
}

// DokterCabang lists the doctors practising at a branch (refmdokterauth), by
// id like the legacy Dokter dropdown.
func (p *PostgresRepository) DokterCabang(ctx context.Context, cabang string) ([]Option, error) {
	rows, err := p.pool.Query(ctx, `SELECT d.id, COALESCE(trim(d.nama), '') FROM refmdokter d
		JOIN refmdokterauth a ON a.dokterid = d.id AND a.cabang = $1 ORDER BY d.id`, cabang)
	if err != nil {
		return nil, err
	}
	list, err := pgx.CollectRows(rows, func(r pgx.CollectableRow) (Option, error) {
		var o Option
		err := r.Scan(&o.ID, &o.Name)
		return o, err
	})
	if list == nil {
		list = []Option{}
	}
	return list, err
}
