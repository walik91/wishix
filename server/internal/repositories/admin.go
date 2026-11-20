package repositories

import (
	"context"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"wishix/server/internal/db"
	"wishix/server/internal/models"
)

type Admin struct{ executor *db.Executor }

func NewAdmin(executor *db.Executor) *Admin { return &Admin{executor: executor} }

type AdminUser struct {
	ID          string     `json:"id"`
	Username    string     `json:"username"`
	DisplayName string     `json:"display_name"`
	Email       *string    `json:"email"`
	CreatedAt   time.Time  `json:"created_at"`
	DeletedAt   *time.Time `json:"deleted_at"`
}

func (a *Admin) Users(ctx context.Context, offset, limit int) ([]AdminUser, error) {
	users := make([]AdminUser, 0)
	err := a.executor.DB(ctx).Table("users").Select("id, username, display_name, email, created_at, deleted_at").Order("created_at DESC, id DESC").Offset(offset).Limit(limit).Scan(&users).Error
	return users, err
}

// SoftDeleteUser is called within the retrying serializable transaction. Keep
// profiles, content, identities, sessions and relations, but hide them through
// the deleted user marker. Reservations move to an archive so their unique
// wish slots are released without destroying their history.
func (a *Admin) SoftDeleteUser(ctx context.Context, id string) (bool, error) {
	tx := a.executor.DB(ctx)
	var user models.User
	q := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", id).Take(&user)
	if q.Error == gorm.ErrRecordNotFound {
		return false, nil
	}
	if q.Error != nil {
		return false, q.Error
	}
	predicate := `(r.user_id = ? OR r.wish_id IN (SELECT id FROM wishes WHERE owner_id = ?))`
	// Suppress pending mail tied to these bookings and messages to the account.
	if err := tx.Exec(`UPDATE reservation_mail_outbox SET suppressed_at = now() WHERE sent_at IS NULL AND (recipient = ? OR reservation_id IN (SELECT r.id FROM reservations r WHERE `+predicate+`))`, user.Email, id, id).Error; err != nil {
		return false, err
	}
	if err := tx.Exec(`INSERT INTO admin_deleted_reservations (reservation_id, deleted_for_user_id, archived_at, reservation)
 SELECT r.id, ?::UUID, now(), to_jsonb(r) FROM reservations r WHERE `+predicate, id, id, id).Error; err != nil {
		return false, err
	}
	if err := tx.Exec(`DELETE FROM reservations AS r WHERE `+predicate, id, id).Error; err != nil {
		return false, err
	}
	result := tx.Model(&models.User{}).Where("id = ?", id).Update("deleted_at", time.Now().UTC())
	return result.RowsAffected == 1, result.Error
}
