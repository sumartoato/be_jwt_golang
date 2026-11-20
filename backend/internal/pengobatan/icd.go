package pengobatan

import (
	"context"

	"github.com/example/be_jwt_golang/backend/internal/icd"
)

// ICD pages the ICD-X codes for the Diagnosa tab's "Data Diagnosa" lookup. It
// reuses the Master ICD query, so the form needs only the Pengobatan menu.
func (p *PostgresRepository) ICD(ctx context.Context, lp icd.ListParams) (icd.Page, error) {
	return icd.NewPostgresRepository(p.pool).List(ctx, lp)
}
