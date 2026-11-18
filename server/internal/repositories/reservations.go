package repositories

import (
	"context"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	security "wishix/server/internal/auth"
	"wishix/server/internal/db"
	"wishix/server/internal/models"
	"wishix/server/internal/services"
)

type Reservations struct{ Base }

func NewReservations(executor *db.Executor) *Reservations {
	return &Reservations{Base: NewBase(executor)}
}

func (r *Reservations) Context(ctx context.Context, listID, token, username, wishID string) (*services.ReservationContext, error) {
	var wish models.Wish
	if listID != "" {
		var list models.Wishlist
		q := r.Ctx(ctx).Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ? AND (visibility = 'public' OR (visibility = 'link' AND share_token = ? AND ? <> ''))", listID, token, token).Limit(1).Find(&list)
		if q.Error != nil || q.RowsAffected == 0 {
			return nil, q.Error
		}
		if services.ListExpired(&list, time.Now()) {
			return nil, nil
		}
		q = r.Ctx(ctx).Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ? AND owner_id = ? AND id IN (SELECT wish_id FROM wishlist_wishes WHERE list_id = ? AND received = false)", wishID, list.OwnerID, list.ID).Limit(1).Find(&wish)
		if q.Error != nil || q.RowsAffected == 0 {
			return nil, q.Error
		}
		return &services.ReservationContext{List: &list, Wish: wish}, nil
	}
	var user models.User
	q := r.Ctx(ctx).Clauses(clause.Locking{Strength: "UPDATE"}).Where("username = ? AND show_all_wishes = true", username).Limit(1).Find(&user)
	if q.Error != nil || q.RowsAffected == 0 {
		return nil, q.Error
	}
	q = r.Ctx(ctx).Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ? AND owner_id = ?", wishID, user.ID).Limit(1).Find(&wish)
	if q.Error != nil || q.RowsAffected == 0 {
		return nil, q.Error
	}
	return &services.ReservationContext{CollectionOwner: user.ID, Wish: wish}, nil
}

func (r *Reservations) InsertChallenge(ctx context.Context, challenge *models.ReservationChallenge) error {
	return r.Ctx(ctx).Create(challenge).Error
}
func (r *Reservations) Challenge(ctx context.Context, id string) (*models.ReservationChallenge, error) {
	var rows []models.ReservationChallenge
	err := r.Ctx(ctx).Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", id).Limit(1).Find(&rows).Error
	if err != nil || len(rows) == 0 {
		return nil, err
	}
	return &rows[0], nil
}
func (r *Reservations) UpdateChallenge(ctx context.Context, challenge *models.ReservationChallenge) error {
	return r.Ctx(ctx).Model(challenge).Update("attempts", challenge.Attempts).Error
}
func (r *Reservations) DeleteChallenge(ctx context.Context, id string) error {
	return r.Ctx(ctx).Where("id = ?", id).Delete(&models.ReservationChallenge{}).Error
}
func (r *Reservations) LockLimit(ctx context.Context, key string) error {
	if err := r.Ctx(ctx).Exec("INSERT INTO reservation_limit_locks (key) VALUES (?) ON CONFLICT (key) DO NOTHING", key).Error; err != nil {
		return err
	}
	var value string
	return r.Ctx(ctx).Raw("SELECT key FROM reservation_limit_locks WHERE key = ? FOR UPDATE", key).Scan(&value).Error
}
func (r *Reservations) CountActive(ctx context.Context, key, email string, now time.Time) (int64, error) {
	var count int64
	err := r.Ctx(ctx).Model(&models.Reservation{}).Where("context_key = ? AND email = ? AND (status = 'purchased' OR expires_at > ?)", key, email, now).Count(&count).Error
	return count, err
}
func (r *Reservations) ByWish(ctx context.Context, wishID string) (*models.Reservation, error) {
	var rows []models.Reservation
	err := r.Ctx(ctx).Clauses(clause.Locking{Strength: "UPDATE"}).Where("wish_id = ?", wishID).Limit(1).Find(&rows).Error
	if err != nil || len(rows) == 0 {
		return nil, err
	}
	return &rows[0], nil
}
func (r *Reservations) ByToken(ctx context.Context, hash string) (*models.Reservation, error) {
	var rows []models.Reservation
	err := r.Ctx(ctx).Clauses(clause.Locking{Strength: "UPDATE"}).Where("token_hash = ?", hash).Limit(1).Find(&rows).Error
	if err != nil || len(rows) == 0 {
		return nil, err
	}
	return &rows[0], nil
}
func (r *Reservations) Insert(ctx context.Context, booking *models.Reservation) error {
	return r.Ctx(ctx).Create(booking).Error
}
func (r *Reservations) Delete(ctx context.Context, wishID string) error {
	return r.Ctx(ctx).Where("wish_id = ?", wishID).Delete(&models.Reservation{}).Error
}
func (r *Reservations) Update(ctx context.Context, booking *models.Reservation) error {
	return r.Ctx(ctx).Model(booking).Updates(map[string]any{"status": booking.Status, "expires_at": booking.ExpiresAt}).Error
}
func (r *Reservations) Queue(ctx context.Context, mail *models.ReservationMailOutbox) error {
	return r.Ctx(ctx).Create(mail).Error
}

func (r *Reservations) Details(ctx context.Context, booking *models.Reservation) (*models.Wish, *models.Wishlist, error) {
	var wish models.Wish
	q := r.Ctx(ctx).Where("id = ?", booking.WishID).Limit(1).Find(&wish)
	if q.Error != nil || q.RowsAffected == 0 {
		return nil, nil, q.Error
	}
	if booking.ListID == nil {
		return &wish, nil, nil
	}
	var list models.Wishlist
	q = r.Ctx(ctx).Where("id = ?", *booking.ListID).Limit(1).Find(&list)
	if q.Error != nil || q.RowsAffected == 0 {
		return nil, nil, q.Error
	}
	return &wish, &list, nil
}

// The predicates are fixed by the caller, never constructed from user input.
// Queue and delete participate in the surrounding domain transaction.
func cancelReservations(database *gorm.DB, predicate string, args []any, kind, message string) error {
	query := `INSERT INTO reservation_mail_outbox (id, recipient, locale, kind, wish_title, list_title, event_date, message, created_at, available_at)
		SELECT gen_random_uuid(), r.email, r.locale, ?, w.title, COALESCE(l.title, ''), '', ?, now(), now()
		FROM reservations r JOIN wishes w ON w.id = r.wish_id LEFT JOIN wishlists l ON l.id = r.list_id
		WHERE (r.status = 'purchased' OR r.expires_at > now()) AND ` + predicate
	parameters := append([]any{kind, message}, args...)
	if err := database.Exec(query, parameters...).Error; err != nil {
		return err
	}
	return database.Exec("DELETE FROM reservations AS r WHERE "+predicate, args...).Error
}

func (r *Lists) ReservationDateChanged(ctx context.Context, list *models.Wishlist, end *time.Time, now time.Time) error {
	date := ""
	if list.EventDate != nil {
		date = list.EventDate.Format("2006-01-02")
	}
	if err := r.Ctx(ctx).Exec(`INSERT INTO reservation_mail_outbox (id, recipient, locale, kind, wish_title, list_title, event_date, reservation_id, created_at, available_at)
		SELECT gen_random_uuid(), b.email, b.locale, 'date_changed', w.title, ?, ?, b.id, ?, ?
		FROM reservations b JOIN wishes w ON w.id = b.wish_id
		WHERE b.list_id = ? AND (b.status = 'purchased' OR b.expires_at > ?)`, list.Title, date, now, now, list.ID, now).Error; err != nil {
		return err
	}
	if end == nil {
		return nil
	}
	return r.Ctx(ctx).Exec(`UPDATE reservations SET expires_at = ? WHERE list_id = ? AND status = 'reserved' AND expires_at > ?`, *end, list.ID, *end).Error
}

// Claim one message with a lease. A crash may produce a duplicate email, but no
// committed notification is lost and no mail is sent from a retried transaction.
func (r *Reservations) ClaimMail(ctx context.Context, now time.Time) (*models.ReservationMailOutbox, error) {
	var rows []models.ReservationMailOutbox
	err := r.Ctx(ctx).Clauses(clause.Locking{Strength: "UPDATE"}).Where("sent_at IS NULL AND available_at <= ? AND (lease_until IS NULL OR lease_until <= ?)", now, now).Order("created_at, id").Limit(1).Find(&rows).Error
	if err != nil || len(rows) == 0 {
		return nil, err
	}
	mail := &rows[0]
	lease := now.Add(time.Minute)
	mail.Attempts++
	if err := r.Ctx(ctx).Model(mail).Updates(map[string]any{"attempts": mail.Attempts, "lease_until": lease}).Error; err != nil {
		return nil, err
	}
	return mail, nil
}
func (r *Reservations) FinishMail(ctx context.Context, id string, attempt int, now time.Time, sent bool) error {
	values := map[string]any{"lease_until": nil}
	if sent {
		values["sent_at"] = now
	} else {
		values["available_at"] = now.Add(5 * time.Minute)
	}
	return r.Ctx(ctx).Model(&models.ReservationMailOutbox{}).Where("id = ? AND attempts = ? AND sent_at IS NULL", id, attempt).Updates(values).Error
}
func (r *Reservations) ScheduleReminder(ctx context.Context, now time.Time) error {
	var bookings []models.Reservation
	if err := r.Ctx(ctx).Clauses(clause.Locking{Strength: "UPDATE", Options: "SKIP LOCKED"}).Where("status = 'reserved' AND expires_at > ? AND expires_at <= ? AND (reminder_for IS NULL OR reminder_for != expires_at)", now, now.Add(48*time.Hour)).Limit(50).Find(&bookings).Error; err != nil {
		return err
	}
	for _, booking := range bookings {
		wish, list, err := r.Details(ctx, &booking)
		if err != nil {
			return err
		}
		if wish == nil {
			continue
		}
		mail := &models.ReservationMailOutbox{ID: security.UUID(), Recipient: booking.Email, Locale: booking.Locale, Kind: "reminder", WishTitle: wish.Title, ReservationID: &booking.ID, CreatedAt: now, AvailableAt: now}
		if list != nil {
			mail.ListTitle = list.Title
		}
		if err := r.Queue(ctx, mail); err != nil {
			return err
		}
		if err := r.Ctx(ctx).Model(&models.Reservation{}).Where("wish_id = ? AND id = ?", booking.WishID, booking.ID).Update("reminder_for", booking.ExpiresAt).Error; err != nil {
			return err
		}
	}
	return nil
}
