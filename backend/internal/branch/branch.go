// Package branch manages clinic branches (Management > Cabang), stored in the
// refbranch table whose layout matches refbranch in the legacy "lab" database.
package branch

import (
	"context"
	"errors"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/example/be_jwt_golang/backend/internal/httpx"
)

var (
	ErrNotFound  = errors.New("branch not found")
	ErrCodeTaken = errors.New("branch code already exists")
)

// Branch is what the API returns. The secret key itself is never sent back.
type Branch struct {
	ID             int    `json:"id"`
	Kode           string `json:"kode"`
	Nama           string `json:"nama"`
	Alamat         string `json:"alamat"`
	Telp           string `json:"telp"`
	KodeKwt        string `json:"kodeKwt"`
	NPWP           string `json:"npwp"`
	NamaNPWP       string `json:"namaNpwp"`
	DBHbee         string `json:"dbHbee"`
	KodeFasyankes  string `json:"kodeFasyankes"`
	NamaFasyankes  string `json:"namaFasyankes"`
	ClientKey      string `json:"clientKey"`
	SecretKeySet   bool   `json:"secretKeySet"`
	SecretKeyHint  string `json:"secretKeyHint"` // last 4 characters of a long enough key, else ""
	OrganizationID string `json:"organizationId"`
	IsActive       bool   `json:"isActive"`
	IsInternal     bool   `json:"isInternal"`
}

// Input is the create/update body. Kode is only used on create (it is the
// primary key); on update an empty secretKey keeps the stored one.
type Input struct {
	Kode           string `json:"kode"`
	Nama           string `json:"nama"`
	Alamat         string `json:"alamat"`
	Telp           string `json:"telp"`
	KodeKwt        string `json:"kodeKwt"`
	NPWP           string `json:"npwp"`
	NamaNPWP       string `json:"namaNpwp"`
	DBHbee         string `json:"dbHbee"`
	KodeFasyankes  string `json:"kodeFasyankes"`
	NamaFasyankes  string `json:"namaFasyankes"`
	ClientKey      string `json:"clientKey"`
	SecretKey      string `json:"secretKey"`
	OrganizationID string `json:"organizationId"`
	IsActive       bool   `json:"isActive"`
	IsInternal     bool   `json:"isInternal"`
}

var kodePattern = regexp.MustCompile(`^[A-Z0-9_-]+$`)

// Normalize trims every field, uppercases kode and returns per-field errors.
// Limits match the varchar sizes of refbranch. checkKode is false on update.
func (in *Input) Normalize(checkKode bool) httpx.FieldErrors {
	for _, f := range []*string{&in.Kode, &in.Nama, &in.Alamat, &in.Telp, &in.KodeKwt, &in.NPWP, &in.NamaNPWP,
		&in.DBHbee, &in.KodeFasyankes, &in.NamaFasyankes, &in.ClientKey, &in.SecretKey, &in.OrganizationID} {
		*f = strings.TrimSpace(*f)
	}
	in.Kode = strings.ToUpper(in.Kode)
	in.KodeKwt = strings.ToUpper(in.KodeKwt)

	errs := httpx.FieldErrors{}
	maxLen := func(field, label, v string, n int) {
		if _, done := errs[field]; !done && utf8.RuneCountInString(v) > n {
			errs[field] = label + " must be at most " + strconv.Itoa(n) + " characters"
		}
	}
	if checkKode {
		switch {
		case in.Kode == "":
			errs["kode"] = "Kode Cabang is required"
		case !kodePattern.MatchString(in.Kode):
			errs["kode"] = "Use only letters, numbers, - and _"
		}
		maxLen("kode", "Kode Cabang", in.Kode, 20)
	}
	if in.Nama == "" {
		errs["nama"] = "Nama Cabang is required"
	}
	maxLen("nama", "Nama Cabang", in.Nama, 50)
	maxLen("alamat", "Alamat", in.Alamat, 200)
	maxLen("telp", "Telp", in.Telp, 200)
	maxLen("kodeKwt", "Kode KWT", in.KodeKwt, 10)
	maxLen("npwp", "NPWP", in.NPWP, 50)
	maxLen("namaNpwp", "Nama NPWP", in.NamaNPWP, 100)
	maxLen("dbHbee", "DB Hbee", in.DBHbee, 50)
	maxLen("kodeFasyankes", "Kode Fasyankes", in.KodeFasyankes, 50)
	maxLen("namaFasyankes", "Nama Fasyankes", in.NamaFasyankes, 100)
	maxLen("clientKey", "Client Key", in.ClientKey, 100)
	maxLen("secretKey", "Secret Key", in.SecretKey, 100)
	maxLen("organizationId", "Organization ID", in.OrganizationID, 100)
	return errs
}

type Repository interface {
	List(ctx context.Context) ([]Branch, error)
	Create(ctx context.Context, in Input) (Branch, error)
	Update(ctx context.Context, kode string, in Input) (Branch, error)
}

type PostgresRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresRepository(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{pool: pool}
}

// columns reads NULLs as "" and computes the secret hint in SQL, so the secret
// never leaves the database.
const columns = `id, kode, COALESCE(nama, ''), COALESCE(alamat, ''), COALESCE(telp, ''), COALESCE(kode_kwt, ''),
	COALESCE(npwp, ''), COALESCE(nama_npwp, ''), COALESCE(db_hbee, ''),
	COALESCE(kode_fasyankes, ''), COALESCE(nama_fasyankes, ''), COALESCE(clientkey, ''),
	COALESCE(secretkey, '') <> '',
	CASE WHEN length(secretkey) >= 12 THEN right(secretkey, 4) ELSE '' END,
	COALESCE(organizationid, ''), COALESCE(isactive, 0) = 1, isinternal = 1`

func scan(row pgx.Row) (Branch, error) {
	var b Branch
	err := row.Scan(&b.ID, &b.Kode, &b.Nama, &b.Alamat, &b.Telp, &b.KodeKwt, &b.NPWP, &b.NamaNPWP, &b.DBHbee,
		&b.KodeFasyankes, &b.NamaFasyankes, &b.ClientKey, &b.SecretKeySet, &b.SecretKeyHint,
		&b.OrganizationID, &b.IsActive, &b.IsInternal)
	return b, err
}

func flag(v bool) int {
	if v {
		return 1
	}
	return 0
}

func (p *PostgresRepository) List(ctx context.Context) ([]Branch, error) {
	rows, err := p.pool.Query(ctx, `SELECT `+columns+` FROM refbranch ORDER BY nama, kode`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Branch{}
	for rows.Next() {
		b, err := scan(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, b)
	}
	return out, rows.Err()
}

// Empty optional values are stored as NULL, like the lab data.
func (p *PostgresRepository) Create(ctx context.Context, in Input) (Branch, error) {
	b, err := scan(p.pool.QueryRow(ctx,
		`INSERT INTO refbranch (kode, nama, alamat, telp, kode_kwt, npwp, nama_npwp, db_hbee,
		                        kode_fasyankes, nama_fasyankes, clientkey, secretkey, organizationid, isactive, isinternal)
		 VALUES ($1, NULLIF($2, ''), NULLIF($3, ''), NULLIF($4, ''), NULLIF($5, ''), NULLIF($6, ''), NULLIF($7, ''), NULLIF($8, ''),
		         NULLIF($9, ''), NULLIF($10, ''), NULLIF($11, ''), NULLIF($12, ''), NULLIF($13, ''), $14, $15)
		 RETURNING `+columns,
		in.Kode, in.Nama, in.Alamat, in.Telp, in.KodeKwt, in.NPWP, in.NamaNPWP, in.DBHbee,
		in.KodeFasyankes, in.NamaFasyankes, in.ClientKey, in.SecretKey, in.OrganizationID, flag(in.IsActive), flag(in.IsInternal)))
	return b, translate(err)
}

func (p *PostgresRepository) Update(ctx context.Context, kode string, in Input) (Branch, error) {
	b, err := scan(p.pool.QueryRow(ctx,
		`UPDATE refbranch SET nama = NULLIF($2, ''), alamat = NULLIF($3, ''), telp = NULLIF($4, ''),
		        kode_kwt = NULLIF($5, ''), npwp = NULLIF($6, ''), nama_npwp = NULLIF($7, ''), db_hbee = NULLIF($8, ''),
		        kode_fasyankes = NULLIF($9, ''), nama_fasyankes = NULLIF($10, ''), clientkey = NULLIF($11, ''),
		        secretkey = COALESCE(NULLIF($12, ''), secretkey),
		        organizationid = NULLIF($13, ''), isactive = $14, isinternal = $15
		 WHERE kode = $1
		 RETURNING `+columns,
		kode, in.Nama, in.Alamat, in.Telp, in.KodeKwt, in.NPWP, in.NamaNPWP, in.DBHbee,
		in.KodeFasyankes, in.NamaFasyankes, in.ClientKey, in.SecretKey, in.OrganizationID, flag(in.IsActive), flag(in.IsInternal)))
	return b, translate(err)
}

func translate(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return ErrCodeTaken
	}
	return err
}
