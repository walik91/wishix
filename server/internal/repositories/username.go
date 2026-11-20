package repositories

import (
	"errors"
	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/gorm"
)

// claimUsername runs inside the caller's serializable transaction. Expired
// claims are soft deleted; the partial unique index arbitrates concurrent claims.
func claimUsername(tx *gorm.DB, username, userID string, allowOwned bool) (bool, error) {
	if err := tx.Exec(`UPDATE username_registry SET deleted_at = now()
  WHERE username = ? AND deleted_at IS NULL AND expires_at IS NOT NULL
  AND (expires_at <= now() OR (? AND user_id = ?))`, username, allowOwned, userID).Error; err != nil {
		return false, err
	}
	result := tx.Exec(`INSERT INTO username_registry (username, user_id, expires_at, deleted_at) VALUES (?, ?, NULL, NULL)
  ON CONFLICT (username, user_id) DO UPDATE SET expires_at = NULL, deleted_at = NULL
  WHERE username_registry.deleted_at IS NOT NULL`, username, userID)
	var pg *pgconn.PgError
	if errors.As(result.Error, &pg) && pg.Code == "23505" {
		return false, nil
	}
	return result.RowsAffected == 1, result.Error
}
