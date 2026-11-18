// Package orglokasi serves Master Data Satu Sehat > Organisasi & Lokasi: per
// branch and poli, the SATUSEHAT Organization and Location the clinic uses
// (reforganizationlocation). A user only sees and edits the rows of the
// branches they are mapped to (refbranchauth). Rows are fixed per branch and
// poli, so there is no create or delete: only the names and SATUSEHAT IDs
// change.
package orglokasi

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/example/be_jwt_golang/backend/internal/branchauth"
	"github.com/example/be_jwt_golang/backend/internal/httpx"
)

var ErrNotFound = errors.New("organisasi & lokasi not found")

// Row is a reforganizationlocation row with its branch and poli names.
type Row struct {
	ID           int        `json:"id"`
	Cabang       string     `json:"cabang"`
	NamaCabang   string     `json:"namaCabang"`
	IDJenisRawat *int       `json:"idJenisRawat"`
	Poli         string     `json:"poli"`
	Organisasi   string     `json:"organisasi"`
	Lokasi       string     `json:"lokasi"`
	OrganisasiID string     `json:"organisasiId"` // "" until registered in SATUSEHAT
	LokasiID     string     `json:"lokasiId"`
	UpdatedBy    string     `json:"updatedBy"`
	UpdatedAt    *time.Time `json:"updatedAt"`
}

// Input is the editable part of a row.
type Input struct {
	Organisasi   string `json:"organisasi"`
	Lokasi       string `json:"lokasi"`
	OrganisasiID string `json:"organisasiId"`
	LokasiID     string `json:"lokasiId"`
}

// SATUSEHAT resource ids are UUIDs.
var uuidRe = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

// Normalize trims the fields, lower-cases the ids and validates them
// (names: required, varchar(50); ids: empty or a UUID).
func (in *Input) Normalize() httpx.FieldErrors {
	in.Organisasi = strings.TrimSpace(in.Organisasi)
	in.Lokasi = strings.TrimSpace(in.Lokasi)
	in.OrganisasiID = strings.ToLower(strings.TrimSpace(in.OrganisasiID))
	in.LokasiID = strings.ToLower(strings.TrimSpace(in.LokasiID))
	errs := httpx.FieldErrors{}
	name := func(field, label, v string) {
		switch {
		case v == "":
			errs[field] = label + " is required"
		case utf8.RuneCountInString(v) > 50:
			errs[field] = fmt.Sprintf("%s must be at most 50 characters", label)
		}
	}
	id := func(field, label, v string) {
		if v != "" && !uuidRe.MatchString(v) {
			errs[field] = label + " must be a SATUSEHAT id (UUID, e.g. b85ff1fd-84ce-46dc-a216-863df8f9993f)"
		}
	}
	name("organisasi", "Organisasi", in.Organisasi)
	name("lokasi", "Lokasi", in.Lokasi)
	id("organisasiId", "Organisasi ID", in.OrganisasiID)
	id("lokasiId", "Lokasi ID", in.LokasiID)
	if len(errs) == 0 {
		return nil
	}
	return errs
}

type Repository interface {
	UserBranches(ctx context.Context, userID string) ([]branchauth.Branch, error)
	List(ctx context.Context, branches []string) ([]Row, error)
	Get(ctx context.Context, id int) (Row, error)
	Update(ctx context.Context, id int, in Input, user string) (Row, error)
}

type PostgresRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresRepository(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{pool: pool}
}

func (p *PostgresRepository) UserBranches(ctx context.Context, userID string) ([]branchauth.Branch, error) {
	return branchauth.ForUser(ctx, p.pool, userID)
}

const selectRows = `SELECT o.id, COALESCE(o.cabang, ''), COALESCE(b.nama, ''), o.jenisrawatid, COALESCE(j.info, ''),
	COALESCE(o.organizationname, ''), COALESCE(o.locationname, ''), COALESCE(o.organizationkeyid, ''),
	COALESCE(o.locationkeyid, ''), COALESCE(o.updatedby, ''), o.updatedat
	FROM reforganizationlocation o
	LEFT JOIN refbranch b ON b.kode = o.cabang
	LEFT JOIN refjenisrawat j ON j.id = o.jenisrawatid`

func scan(r pgx.CollectableRow) (Row, error) {
	var x Row
	err := r.Scan(&x.ID, &x.Cabang, &x.NamaCabang, &x.IDJenisRawat, &x.Poli, &x.Organisasi, &x.Lokasi,
		&x.OrganisasiID, &x.LokasiID, &x.UpdatedBy, &x.UpdatedAt)
	return x, err
}

func (p *PostgresRepository) List(ctx context.Context, branches []string) ([]Row, error) {
	rows, err := p.pool.Query(ctx, selectRows+` WHERE o.cabang = ANY($1) ORDER BY b.nama, o.cabang, o.jenisrawatid`, branches)
	if err != nil {
		return nil, err
	}
	list, err := pgx.CollectRows(rows, scan)
	if list == nil {
		list = []Row{}
	}
	return list, err
}

func (p *PostgresRepository) Get(ctx context.Context, id int) (Row, error) {
	rows, err := p.pool.Query(ctx, selectRows+` WHERE o.id = $1`, id)
	if err != nil {
		return Row{}, err
	}
	x, err := pgx.CollectExactlyOneRow(rows, scan)
	if errors.Is(err, pgx.ErrNoRows) {
		return Row{}, ErrNotFound
	}
	return x, err
}

func nullIfEmpty(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func (p *PostgresRepository) Update(ctx context.Context, id int, in Input, user string) (Row, error) {
	// updatedat is the clinic's local time, like the legacy rows.
	tag, err := p.pool.Exec(ctx,
		`UPDATE reforganizationlocation
		 SET organizationname = $2, locationname = $3, organizationkeyid = $4, locationkeyid = $5,
		     updatedby = $6, updatedat = now() AT TIME ZONE 'Asia/Jakarta'
		 WHERE id = $1`,
		id, in.Organisasi, in.Lokasi, nullIfEmpty(in.OrganisasiID), nullIfEmpty(in.LokasiID), user)
	if err != nil {
		return Row{}, err
	}
	if tag.RowsAffected() == 0 {
		return Row{}, ErrNotFound
	}
	return p.Get(ctx, id)
}
