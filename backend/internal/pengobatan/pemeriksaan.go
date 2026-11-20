package pengobatan

import (
	"context"
	"errors"
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"

	"github.com/example/be_jwt_golang/backend/internal/httpx"
	"github.com/example/be_jwt_golang/backend/internal/pendaftaran"
)

var (
	ErrRegNotFound    = errors.New("pendaftaran not found")
	ErrRegNotEditable = errors.New("the examination can only change while the visit is DAFTAR, PROSES or PAYMENT")
	ErrUnknownDokter  = errors.New("unknown dokter")
)

// Pemeriksaan is the examination of one registration (trxpemeriksaan, one row
// per id_kun): complaints, vital signs, findings and the responsible doctor.
type Pemeriksaan struct {
	KeluhanUtama           string   `json:"keluhanUtama"`
	Anamnesis              string   `json:"anamnesis"`
	Sistole                *int     `json:"sistole"`  // mmHg
	Diastole               *int     `json:"diastole"` // mmHg
	BeratBadan             *int     `json:"beratBadan"`
	TinggiBadan            *int     `json:"tinggiBadan"`
	Suhu                   *float64 `json:"suhu"` // °C
	PemeriksaanFisik       string   `json:"pemeriksaanFisik"`
	RekapHasilPenunjang    string   `json:"rekapHasilPenunjang"`
	HasilPenunjang         string   `json:"hasilPenunjang"`
	RencanaPenatalaksanaan string   `json:"rencanaPenatalaksanaan"`
	HasilPemeriksaanDokter string   `json:"hasilPemeriksaanDokter"`
	TindakanDokter         string   `json:"tindakanDokter"`
	IDDokter               *int     `json:"idDokter"` // Dokter PJ
}

// PemeriksaanRecord is a saved examination with who last changed it.
type PemeriksaanRecord struct {
	Pemeriksaan
	Dokter    string  `json:"dokter"`
	UpdatedBy string  `json:"updatedBy"`
	UpdatedAt *string `json:"updatedAt"` // "YYYY-MM-DD HH:MM:SS", clinic time
}

// Column sizes of trxpemeriksaan.
var pemeriksaanText = []struct {
	field string
	max   int
	get   func(*Pemeriksaan) *string
}{
	{"keluhanUtama", 300, func(p *Pemeriksaan) *string { return &p.KeluhanUtama }},
	{"anamnesis", 300, func(p *Pemeriksaan) *string { return &p.Anamnesis }},
	{"pemeriksaanFisik", 300, func(p *Pemeriksaan) *string { return &p.PemeriksaanFisik }},
	{"rekapHasilPenunjang", 300, func(p *Pemeriksaan) *string { return &p.RekapHasilPenunjang }},
	{"hasilPenunjang", 300, func(p *Pemeriksaan) *string { return &p.HasilPenunjang }},
	{"rencanaPenatalaksanaan", 300, func(p *Pemeriksaan) *string { return &p.RencanaPenatalaksanaan }},
	{"hasilPemeriksaanDokter", 400, func(p *Pemeriksaan) *string { return &p.HasilPemeriksaanDokter }},
	{"tindakanDokter", 400, func(p *Pemeriksaan) *string { return &p.TindakanDokter }},
}

// Validate trims the text fields and checks lengths and plausible vital signs.
func (p *Pemeriksaan) Validate() httpx.FieldErrors {
	errs := httpx.FieldErrors{}
	for _, t := range pemeriksaanText {
		v := t.get(p)
		*v = strings.TrimSpace(*v)
		if utf8.RuneCountInString(*v) > t.max {
			errs[t.field] = "At most " + strconv.Itoa(t.max) + " characters"
		}
	}
	intRange := func(field string, v *int, lo, hi int) {
		if v != nil && (*v < lo || *v > hi) {
			errs[field] = "Between " + strconv.Itoa(lo) + " and " + strconv.Itoa(hi)
		}
	}
	intRange("sistole", p.Sistole, 0, 300)
	intRange("diastole", p.Diastole, 0, 250)
	intRange("beratBadan", p.BeratBadan, 0, 500)
	intRange("tinggiBadan", p.TinggiBadan, 0, 300)
	if p.Suhu != nil && (*p.Suhu < 25 || *p.Suhu > 45) {
		errs["suhu"] = "Between 25 and 45 °C"
	}
	if p.IDDokter != nil && *p.IDDokter <= 0 {
		p.IDDokter = nil
	}
	return errs
}

// RegBranch returns a registration's branch and visit status.
func (p *PostgresRepository) RegBranch(ctx context.Context, idKun int) (string, int, error) {
	var cabang string
	var status int
	err := p.pool.QueryRow(ctx, `SELECT COALESCE(cabang, ''), COALESCE(id_statustr, 0) FROM trxpendaftaran WHERE id = $1`, idKun).Scan(&cabang, &status)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", 0, ErrRegNotFound
	}
	return cabang, status, err
}

// GetPemeriksaan returns the registration's examination, or nil when there is none yet.
func (p *PostgresRepository) GetPemeriksaan(ctx context.Context, idKun int) (*PemeriksaanRecord, error) {
	var r PemeriksaanRecord
	err := p.pool.QueryRow(ctx,
		`SELECT COALESCE(pm.keluhan_utama, ''), COALESCE(pm.anamnesis, ''), pm.tekanan_darah_sistole, pm.tekanan_darah_diastole,
		        pm.berat_badan, pm.tinggi_badan, pm.suhu, COALESCE(pm.pemeriksaanfisik, ''),
		        COALESCE(pm.rekap_hasil_pemeriksaan_penunjang, ''), COALESCE(pm.hasil_pemeriksaan_penunjang, ''),
		        COALESCE(pm.rencana_penatalaksanaan, ''), COALESCE(pm.hasil_pemeriksaan_dokter, ''), COALESCE(pm.tindakan_dokter, ''),
		        dk.id, COALESCE(trim(dk.nama), ''), COALESCE(pm.update_user, pm.create_user, ''),
		        to_char(COALESCE(pm.update_date, pm.create_date), 'YYYY-MM-DD HH24:MI:SS')
		 FROM trxpemeriksaan pm LEFT JOIN refmdokter dk ON dk.id = pm.id_dok
		 WHERE pm.id_kun = $1`, idKun).Scan(
		&r.KeluhanUtama, &r.Anamnesis, &r.Sistole, &r.Diastole, &r.BeratBadan, &r.TinggiBadan, &r.Suhu, &r.PemeriksaanFisik,
		&r.RekapHasilPenunjang, &r.HasilPenunjang, &r.RencanaPenatalaksanaan, &r.HasilPemeriksaanDokter, &r.TindakanDokter,
		&r.IDDokter, &r.Dokter, &r.UpdatedBy, &r.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &r, nil
}

// SavePemeriksaan inserts or updates the registration's examination (one row
// per id_kun) in clinic time.
func (p *PostgresRepository) SavePemeriksaan(ctx context.Context, idKun int, in Pemeriksaan, user string) error {
	if in.IDDokter != nil {
		var ok bool
		if err := p.pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM refmdokter WHERE id = $1)`, *in.IDDokter).Scan(&ok); err != nil {
			return err
		}
		if !ok {
			return ErrUnknownDokter
		}
	}
	now := `now() AT TIME ZONE '` + pendaftaran.ClinicTimeZone + `'`
	_, err := p.pool.Exec(ctx,
		`INSERT INTO trxpemeriksaan (id_kun, keluhan_utama, anamnesis, tekanan_darah_sistole, tekanan_darah_diastole, berat_badan,
		        tinggi_badan, suhu, pemeriksaanfisik, rekap_hasil_pemeriksaan_penunjang, hasil_pemeriksaan_penunjang,
		        rencana_penatalaksanaan, hasil_pemeriksaan_dokter, tindakan_dokter, id_dok, create_user, create_date)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, `+now+`)
		 ON CONFLICT (id_kun) DO UPDATE SET
		        keluhan_utama = EXCLUDED.keluhan_utama, anamnesis = EXCLUDED.anamnesis,
		        tekanan_darah_sistole = EXCLUDED.tekanan_darah_sistole, tekanan_darah_diastole = EXCLUDED.tekanan_darah_diastole,
		        berat_badan = EXCLUDED.berat_badan, tinggi_badan = EXCLUDED.tinggi_badan, suhu = EXCLUDED.suhu,
		        pemeriksaanfisik = EXCLUDED.pemeriksaanfisik,
		        rekap_hasil_pemeriksaan_penunjang = EXCLUDED.rekap_hasil_pemeriksaan_penunjang,
		        hasil_pemeriksaan_penunjang = EXCLUDED.hasil_pemeriksaan_penunjang,
		        rencana_penatalaksanaan = EXCLUDED.rencana_penatalaksanaan,
		        hasil_pemeriksaan_dokter = EXCLUDED.hasil_pemeriksaan_dokter, tindakan_dokter = EXCLUDED.tindakan_dokter,
		        id_dok = EXCLUDED.id_dok, update_user = EXCLUDED.create_user, update_date = `+now,
		idKun, in.KeluhanUtama, in.Anamnesis, in.Sistole, in.Diastole, in.BeratBadan, in.TinggiBadan, in.Suhu, in.PemeriksaanFisik,
		in.RekapHasilPenunjang, in.HasilPenunjang, in.RencanaPenatalaksanaan, in.HasilPemeriksaanDokter, in.TindakanDokter,
		in.IDDokter, user)
	return err
}

// examEditable reports whether a visit status still allows changing the examination.
func examEditable(status int) bool { return slices.Contains(BillableStatuses, status) }
