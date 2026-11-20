package pengobatan

import (
	"context"
	"errors"
	"math"

	"github.com/jackc/pgx/v5"
)

// NotaLine is one priced line of a sale on the printed bill.
type NotaLine struct {
	Nama   string  `json:"nama"`
	Satuan string  `json:"satuan,omitempty"`
	Byk    float64 `json:"byk"`
	Harga  float64 `json:"harga"`
	Jumlah float64 `json:"jumlah"`
	PPN    float64 `json:"ppn,omitempty"`
	Dosis  string  `json:"dosis,omitempty"`
}

// Nota is a saved sale as printed from the Print menu (A5, A5 bold, thermal, summary).
type Nota struct {
	ID           int        `json:"id"`
	NoKwi        string     `json:"noKwi"`
	Tgl          *string    `json:"tgl"` // "YYYY-MM-DD HH:MM:SS", clinic time
	Cabang       string     `json:"cabang"`
	NamaCabang   string     `json:"namaCabang"`
	AlamatCabang string     `json:"alamatCabang"`
	TelpCabang   string     `json:"telpCabang"`
	NamaPT       string     `json:"namaPt"`  // the company the branch is registered under
	NoCetak      string     `json:"noCetak"` // print number, set at the first print
	IDKun        *int       `json:"idKun"`
	NoRM         string     `json:"noRm"`
	Pasien       string     `json:"pasien"`
	AlamatPasien string     `json:"alamatPasien"`
	Penanggung   string     `json:"penanggung"`
	Point        float64    `json:"point"` // the patient's points balance
	Dokter       string     `json:"dokter"`
	Pelayanan    string     `json:"pelayanan"`
	JenisBayar   string     `json:"jenisBayar"`
	Jamkes       string     `json:"jamkes"`
	Kasir        string     `json:"kasir"`
	Obat         []NotaLine `json:"obat"`
	Tindakan     []NotaLine `json:"tindakan"`
	Penunjang    []NotaLine `json:"penunjang"`
	Diagnosa     []string   `json:"diagnosa"` // "K05.0 ACUTE GINGIVITIS"
	TotObat      float64    `json:"totObat"`
	TotTindakan  float64    `json:"totTindakan"`
	TotPenunjang float64    `json:"totPenunjang"`
	Total        float64    `json:"total"`
	Potongan     float64    `json:"potongan"`
	GrandTotal   float64    `json:"grandTotal"`
	Pembayaran   float64    `json:"pembayaran"`
	Piutang      float64    `json:"piutang"`
	Kembalian    float64    `json:"kembalian"`
	Status       string     `json:"status"` // open, close, cancel
}

// Nota loads a sale with its lines for printing.
func (p *PostgresRepository) Nota(ctx context.Context, id int) (Nota, error) {
	n := Nota{Obat: []NotaLine{}, Tindakan: []NotaLine{}, Penunjang: []NotaLine{}, Diagnosa: []string{}}
	err := p.pool.QueryRow(ctx, `SELECT j.id, COALESCE(j.no_kwi, ''), to_char(j.tgl, 'YYYY-MM-DD HH24:MI:SS'), COALESCE(j.cabang, ''),
			COALESCE(b.nama, ''), COALESCE(b.alamat, ''), COALESCE(b.telp, ''), COALESCE(NULLIF(b.nama_npwp, '-'), ''),
			COALESCE((SELECT c.no_cetak FROM trxjualcetak c WHERE c.id_jual = j.id ORDER BY c.createdate LIMIT 1), ''), j.id_kun, COALESCE(pa.no_rm, ''), COALESCE(pa.nama, ''),
			COALESCE(pa.alamat, ''), COALESCE(pa.penanggung, ''), COALESCE(pp.point, 0), COALESCE(trim(dk.nama), ''), COALESCE(r.info, ''), COALESCE(jb.info, ''), COALESCE(jk.nama, ''),
			COALESCE(j.create_user, ''), COALESCE(t.tot_obat, 0), COALESCE(t.tot_tndk, 0), COALESCE(t.tot_pnjng, 0), COALESCE(t.total, 0),
			COALESCE(t.potongan, 0), COALESCE(t.pembayaran, 0), COALESCE(t.piutang, 0), `+statusSQL+`
		FROM trxjual j
		LEFT JOIN trxjualtotal t ON t.id_jual = j.id
		LEFT JOIN refbranch b ON b.kode = j.cabang
		LEFT JOIN refmpasien pa ON pa.id = j.id_pas
		LEFT JOIN refpasienpoint pp ON pp.id_pas = j.id_pas
		LEFT JOIN refmdokter dk ON dk.id = j.id_dok
		LEFT JOIN refjenisrawat r ON r.id = j.id_jenis_rwt
		LEFT JOIN refjenisbayar jb ON jb.id = j.id_jenis_byr
		LEFT JOIN refjaminankesehatan jk ON jk.id = j.id_jamkes
		WHERE j.id = $1`, id).Scan(&n.ID, &n.NoKwi, &n.Tgl, &n.Cabang, &n.NamaCabang, &n.AlamatCabang, &n.TelpCabang, &n.NamaPT, &n.NoCetak, &n.IDKun, &n.NoRM,
		&n.Pasien, &n.AlamatPasien, &n.Penanggung, &n.Point, &n.Dokter, &n.Pelayanan, &n.JenisBayar, &n.Jamkes, &n.Kasir, &n.TotObat, &n.TotTindakan,
		&n.TotPenunjang, &n.Total, &n.Potongan, &n.Pembayaran, &n.Piutang, &n.Status)
	if errors.Is(err, pgx.ErrNoRows) {
		return n, ErrSaleNotFound
	}
	if err != nil {
		return n, err
	}
	n.GrandTotal = math.Max(n.Total-n.Potongan, 0)
	n.Kembalian = math.Max(n.Pembayaran-n.GrandTotal, 0)

	lines := func(dst *[]NotaLine, sql string) error {
		rows, err := p.pool.Query(ctx, sql, id)
		if err != nil {
			return err
		}
		list, err := pgx.CollectRows(rows, func(r pgx.CollectableRow) (NotaLine, error) {
			var l NotaLine
			err := r.Scan(&l.Nama, &l.Satuan, &l.Byk, &l.Harga, &l.PPN, &l.Dosis)
			l.Jumlah = l.Harga * l.Byk
			return l, err
		})
		if list != nil {
			*dst = list
		}
		return err
	}
	if err := lines(&n.Obat, `SELECT COALESCE(o.nama, '(obat '||x.id_obat||')'), COALESCE(s.info, ''), COALESCE(x.byk, 0), COALESCE(x.hjual, 0),
			COALESCE(x.ppn, 0), COALESCE(x.dosis, '')
		FROM trxjualobat x LEFT JOIN refobat o ON o.id = x.id_obat LEFT JOIN refsatuan s ON s.id = o.sat_jual
		WHERE x.id_jual = $1 ORDER BY x.id`); err != nil {
		return n, err
	}
	if err := lines(&n.Tindakan, `SELECT COALESCE(r.nama, '(tindakan '||x.id_tndk||')'), '', COALESCE(x.byk, 0), COALESCE(x.hjual, 0), 0::float8, ''
		FROM trxjualtindakan x LEFT JOIN reftindakan r ON r.id = x.id_tndk WHERE x.id_jual = $1 ORDER BY x.id`); err != nil {
		return n, err
	}
	if err := lines(&n.Penunjang, `SELECT COALESCE(r.nama, '(penunjang '||x.id_pnjng||')'), '', COALESCE(x.byk, 0), COALESCE(x.hjual, 0), 0::float8, ''
		FROM trxjualpnjng x LEFT JOIN refpenunjang r ON r.id = x.id_pnjng WHERE x.id_jual = $1 ORDER BY x.id`); err != nil {
		return n, err
	}
	rows, err := p.pool.Query(ctx, `SELECT COALESCE(i.kode_icd || ' ' || i.nama, '(ICD '||x.icd_id||')')
		FROM trxjualdiagnosa x LEFT JOIN reficd i ON i.id = x.icd_id WHERE x.id_jual = $1 ORDER BY x.id`, id)
	if err != nil {
		return n, err
	}
	dx, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if dx != nil {
		n.Diagnosa = dx
	}
	return n, err
}
