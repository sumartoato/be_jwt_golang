package pengobatan

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/example/be_jwt_golang/backend/internal/pendaftaran"
)

// ErrSaleNotFound is returned for an unknown sale id.
var ErrSaleNotFound = errors.New("pengobatan not found")

// Kuitansi is what the receipt dialog and the printed "KWITANSI PENGOBATAN" show.
type Kuitansi struct {
	ID          int     `json:"id"`
	NoKwi       string  `json:"noKwi"`
	TglKwi      *string `json:"tglKwi"` // YYYY-MM-DD
	Cabang      string  `json:"cabang"`
	NamaCabang  string  `json:"namaCabang"`
	NoRM        string  `json:"noRm"`
	Pasien      string  `json:"pasien"`
	Obat        float64 `json:"biayaObat"`
	Tindakan    float64 `json:"biayaTindakan"`
	Penunjang   float64 `json:"biayaPenunjang"`
	Potongan    float64 `json:"potongan"`
	Jumlah      float64 `json:"jumlah"` // the amount on the receipt: total minus discount
	NoCetak     string  `json:"noCetak"`
	PrintedBy   string  `json:"printedBy"`
	PrintedAt   *string `json:"printedAt"` // first print, "YYYY-MM-DD HH:MM:SS.mmm" clinic time
	PrintedNow  string  `json:"printedNow,omitempty"`
	PrintedUser string  `json:"printedUser,omitempty"`
}

const kuitansiSQL = `SELECT j.id, COALESCE(j.no_kwi, ''), to_char(j.tgl_kwi, 'YYYY-MM-DD'), COALESCE(j.cabang, ''), COALESCE(b.nama, ''),
	COALESCE(p.no_rm, ''), COALESCE(p.nama, ''), COALESCE(t.tot_obat, 0), COALESCE(t.tot_tndk, 0), COALESCE(t.tot_pnjng, 0),
	COALESCE(t.potongan, 0), GREATEST(COALESCE(t.total, 0) - COALESCE(t.potongan, 0), 0),
	COALESCE(c.no_cetak, ''), COALESCE(c.userid, ''), to_char(c.createdate, 'YYYY-MM-DD HH24:MI:SS.MS')
	FROM trxjual j
	LEFT JOIN trxjualtotal t ON t.id_jual = j.id
	LEFT JOIN refmpasien p ON p.id = j.id_pas
	LEFT JOIN refbranch b ON b.kode = j.cabang
	LEFT JOIN trxjualcetak c ON c.id_jual = j.id
	WHERE j.id = $1`

func scanKuitansi(row pgx.Row) (Kuitansi, error) {
	var k Kuitansi
	err := row.Scan(&k.ID, &k.NoKwi, &k.TglKwi, &k.Cabang, &k.NamaCabang, &k.NoRM, &k.Pasien, &k.Obat, &k.Tindakan, &k.Penunjang,
		&k.Potongan, &k.Jumlah, &k.NoCetak, &k.PrintedBy, &k.PrintedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return k, ErrSaleNotFound
	}
	return k, err
}

// Kuitansi returns the receipt data of a sale.
func (p *PostgresRepository) Kuitansi(ctx context.Context, id int) (Kuitansi, error) {
	return scanKuitansi(p.pool.QueryRow(ctx, kuitansiSQL, id))
}

// Cetak records the first print of a sale's receipt (trxjualcetak) and returns
// the receipt with its print number and this print's time. Reprints keep the
// first number, as in lab (one row per sale).
func (p *PostgresRepository) Cetak(ctx context.Context, id int, user string) (Kuitansi, error) {
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return Kuitansi{}, err
	}
	defer tx.Rollback(ctx) //nolint:errcheck // no-op after Commit

	var cabang, noKwi, kodeKwt string
	err = tx.QueryRow(ctx, `SELECT COALESCE(j.cabang, ''), COALESCE(j.no_kwi, ''), COALESCE(NULLIF(b.kode_kwt, ''), j.cabang, '')
		FROM trxjual j LEFT JOIN refbranch b ON b.kode = j.cabang WHERE j.id = $1 FOR UPDATE OF j`, id).Scan(&cabang, &noKwi, &kodeKwt)
	if errors.Is(err, pgx.ErrNoRows) {
		return Kuitansi{}, ErrSaleNotFound
	}
	if err != nil {
		return Kuitansi{}, err
	}
	now := `(now() AT TIME ZONE '` + pendaftaran.ClinicTimeZone + `')`
	var printed bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM trxjualcetak WHERE id_jual = $1)`, id).Scan(&printed); err != nil {
		return Kuitansi{}, err
	}
	if !printed {
		// <code>-YY-MM-<5 digits>, numbered per code and month.
		var prefix string
		if err := tx.QueryRow(ctx, `SELECT $1 || '-' || to_char(`+now+`, 'YY-MM') || '-'`, kodeKwt).Scan(&prefix); err != nil {
			return Kuitansi{}, err
		}
		if len(prefix)+5 > 15 {
			return Kuitansi{}, fmt.Errorf("receipt code %q is too long for a print number", kodeKwt)
		}
		if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext('trxjualcetak.no_cetak:' || $1))`, prefix); err != nil {
			return Kuitansi{}, err
		}
		var last int
		if err := tx.QueryRow(ctx, `SELECT COALESCE(max(substr(no_cetak, $2)::int), 0) FROM trxjualcetak
			WHERE no_cetak LIKE $1 || '%' AND substr(no_cetak, $2) ~ '^[0-9]+$'`, prefix, len(prefix)+1).Scan(&last); err != nil {
			return Kuitansi{}, err
		}
		if _, err := tx.Exec(ctx, `INSERT INTO trxjualcetak (id_jual, no_kwi, no_cetak, cabang, userid, createdate)
			VALUES ($1, $2, $3, $4, $5, `+now+`)`, id, noKwi, fmt.Sprintf("%s%05d", prefix, last+1), cabang, user); err != nil {
			return Kuitansi{}, err
		}
	}
	k, err := scanKuitansi(tx.QueryRow(ctx, kuitansiSQL, id))
	if err != nil {
		return k, err
	}
	if err := tx.QueryRow(ctx, `SELECT to_char(`+now+`, 'YYYY-MM-DD HH24:MI:SS.MS')`).Scan(&k.PrintedNow); err != nil {
		return k, err
	}
	k.PrintedUser = user
	return k, tx.Commit(ctx)
}
