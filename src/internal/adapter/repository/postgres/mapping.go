package postgres

import (
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/specvital/collector/internal/adapter/mapping"
	"github.com/specvital/collector/internal/domain/analysis"
	"github.com/specvital/core/pkg/domain"
)

// toPgUUID converts domain UUID (google/uuid) to pgtype.UUID for database operations.
func toPgUUID(id analysis.UUID) pgtype.UUID {
	return pgtype.UUID{
		Bytes: id,
		Valid: id != analysis.NilUUID,
	}
}

// fromPgUUID converts pgtype.UUID to domain UUID (google/uuid).
func fromPgUUID(id pgtype.UUID) analysis.UUID {
	if !id.Valid {
		return analysis.NilUUID
	}
	return analysis.UUID(id.Bytes)
}

// convertCoreToDomainInventory delegates to shared mapping package.
func convertCoreToDomainInventory(coreInv *domain.Inventory) *analysis.Inventory {
	return mapping.ConvertCoreToDomainInventory(coreInv)
}
