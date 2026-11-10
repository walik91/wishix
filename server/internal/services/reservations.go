package services

import (
	"context"
	"crypto/subtle"
	"errors"
	"fmt"
	"strings"
	"time"

	security "wishix/server/internal/auth"
	"wishix/server/internal/models"
	"wishix/server/internal/reservation"
)

type ReservationError string

func (e ReservationError) Error() string { return string(e) }

const (
	ErrReservationUnavailable ReservationError = "reservation_unavailable"
	ErrReservationLimit       ReservationError = "reservation_limit"
	ErrReservationExpired     ReservationError = "reservation_expired"
	ErrReservationAction      ReservationError = "reservation_action_invalid"
)

type ReservationContext struct {
	List            *models.Wishlist
	CollectionOwner string
	Wish            models.Wish
}
type ReservationRepository interface {
	Context(context.Context, string, string, string, string) (*ReservationContext, error)
	InsertChallenge(context.Context, *models.ReservationChallenge) error
	Challenge(context.Context, string) (*models.ReservationChallenge, error)
	UpdateChallenge(context.Context, *models.ReservationChallenge) error
	DeleteChallenge(context.Context, string) error
	LockLimit(context.Context, string) error
	CountActive(context.Context, string, string, time.Time) (int64, error)
	ByWish(context.Context, string) (*models.Reservation, error)
	ByToken(context.Context, string) (*models.Reservation, error)
	Insert(context.Context, *models.Reservation) error
	Delete(context.Context, string) error
	Update(context.Context, *models.Reservation) error
	Queue(context.Context, *models.ReservationMailOutbox) error
	Details(context.Context, *models.Reservation) (*models.Wish, *models.Wishlist, error)
	ClaimMail(context.Context, time.Time) (*models.ReservationMailOutbox, error)
	FinishMail(context.Context, string, int, time.Time, bool) error
	ScheduleReminder(context.Context, time.Time) error
}
type ReservationMailer interface {
	SendMessage(context.Context, string, string, string) error
}
type Reservations struct {
	repo     ReservationRepository
	tx       Transactor
	auth     *Auth
	mail     CodeMailer
	messages ReservationMailer
	config   *security.Config
}

func NewReservations(repo ReservationRepository, tx Transactor, auth *Auth, mail CodeMailer, messages ReservationMailer, config *security.Config) *Reservations {
	return &Reservations{repo: repo, tx: tx, auth: auth, mail: mail, messages: messages, config: config}
}

type ReservationInput struct {
	WishID    string `json:"wish_id"`
	ListID    string `json:"list_id"`
	ListToken string `json:"list_token"`
	Username  string `json:"username"`
	Email     string `json:"email"`
	Locale    string `json:"locale"`
	Challenge string `json:"challenge"`
	Code      string `json:"code"`
}

func (s *Reservations) context(ctx context.Context, input ReservationInput) (*ReservationContext, error) {
	if !wishIDPattern.MatchString(input.WishID) {
		return nil, ErrReservationUnavailable
	}
	if input.ListID != "" {
		if !validGuestList(input.ListID, input.ListToken) || input.Username != "" {
			return nil, ErrReservationUnavailable
		}
	} else if !usernamePattern.MatchString(strings.ToLower(input.Username)) || input.ListToken != "" {
		return nil, ErrReservationUnavailable
	}
	c, err := s.repo.Context(ctx, input.ListID, input.ListToken, strings.ToLower(input.Username), input.WishID)
	if err == nil && c == nil {
		return nil, ErrReservationUnavailable
	}
	return c, err
}

func bookingActive(booking *models.Reservation, now time.Time) bool {
	return booking != nil && (booking.Status == "purchased" || (booking.ExpiresAt != nil && now.Before(*booking.ExpiresAt)))
}
func contextKey(c *ReservationContext) string {
	if c.List != nil {
		return "list:" + c.List.ID
	}
	return "collection:" + c.CollectionOwner
}
func accessKey(input ReservationInput) string {
	return input.ListID + "\x00" + input.ListToken + "\x00" + strings.ToLower(input.Username)
}
func (s *Reservations) Start(ctx context.Context, input ReservationInput, user *models.User) (string, error) {
	email, err := normalizeEmail(input.Email)
	if err != nil {
		return "", err
	}
	if user != nil && user.Email != email {
		return "", ErrInvalidInput
	}
	if err := s.auth.limit(ctx, "reservation-email", email, 20, time.Hour); err != nil {
		return "", err
	}
	token, code := security.Token(), security.Code()
	now := time.Now().UTC()
	id := security.Digest(token)
	challenge := &models.ReservationChallenge{ID: id, WishID: input.WishID, Email: email, Locale: normalizedLocale(input.Locale), AccessHash: security.MAC(s.config.Secret, "reservation-access:"+accessKey(input)), CodeHash: security.MAC(s.config.Secret, "reservation-code:"+id+":"+code), ExpiresAt: now.Add(10 * time.Minute), CreatedAt: now}
	if user != nil {
		challenge.UserID = &user.ID
	}
	err = s.tx.WithTransaction(ctx, func(tx context.Context) error {
		c, err := s.context(tx, input)
		if err != nil {
			return err
		}
		if c.List != nil {
			challenge.ListID = &c.List.ID
		} else {
			challenge.CollectionOwner = &c.CollectionOwner
		}
		booking, err := s.repo.ByWish(tx, input.WishID)
		if err != nil {
			return err
		}
		if bookingActive(booking, now) {
			return ErrReservationUnavailable
		}
		return s.repo.InsertChallenge(tx, challenge)
	})
	if err != nil {
		return "", err
	}
	if err := s.mail.SendCode(ctx, email, challenge.Locale, "reservation", code); err != nil {
		return "", err
	}
	return token, nil
}

func (s *Reservations) Confirm(ctx context.Context, input ReservationInput, user *models.User) error {
	if len(input.Challenge) != 43 || !codePattern.MatchString(input.Code) {
		return ErrCode
	}
	id := security.Digest(input.Challenge)
	var outcome error
	err := s.tx.WithTransaction(ctx, func(tx context.Context) error {
		outcome = nil
		challenge, err := s.repo.Challenge(tx, id)
		if err != nil {
			return err
		}
		now := time.Now().UTC()
		if challenge == nil || !now.Before(challenge.ExpiresAt) || challenge.Attempts >= 5 {
			outcome = ErrCode
			return nil
		}
		if (challenge.UserID == nil) != (user == nil) || (user != nil && *challenge.UserID != user.ID) {
			outcome = ErrCode
			return nil
		}
		expected := security.MAC(s.config.Secret, "reservation-code:"+id+":"+input.Code)
		access := security.MAC(s.config.Secret, "reservation-access:"+accessKey(input))
		if subtle.ConstantTimeCompare([]byte(challenge.CodeHash), []byte(expected)) != 1 || subtle.ConstantTimeCompare([]byte(challenge.AccessHash), []byte(access)) != 1 || challenge.WishID != input.WishID {
			challenge.Attempts++
			outcome = ErrCode
			return s.repo.UpdateChallenge(tx, challenge)
		}
		c, err := s.context(tx, input)
		if err != nil {
			return err
		}
		key := contextKey(c)
		if err := s.repo.LockLimit(tx, security.MAC(s.config.Secret, "reservation-limit:"+key+":"+challenge.Email)); err != nil {
			return err
		}
		count, err := s.repo.CountActive(tx, key, challenge.Email, now)
		if err != nil {
			return err
		}
		if count >= 4 {
			outcome = ErrReservationLimit
			return nil
		}
		previous, err := s.repo.ByWish(tx, input.WishID)
		if err != nil {
			return err
		}
		if bookingActive(previous, now) {
			outcome = ErrReservationUnavailable
			return nil
		}
		if previous != nil {
			if err := s.repo.Delete(tx, input.WishID); err != nil {
				return err
			}
		}
		var event *time.Time
		if c.List != nil && c.List.EventDate != nil {
			end, err := reservation.EventEnd(c.List.EventDate.Format("2006-01-02"), c.List.EventTimezone)
			if err != nil {
				return err
			}
			event = &end
		}
		expiry := reservation.InitialDeadline(now, event)
		bookingID := security.UUID()
		linkToken := reservation.ManagementToken(s.config.Secret, bookingID)
		booking := &models.Reservation{WishID: c.Wish.ID, ID: bookingID, ListID: challenge.ListID, CollectionOwner: challenge.CollectionOwner, ContextKey: key, Email: challenge.Email, Locale: challenge.Locale, UserID: challenge.UserID, TokenHash: security.Digest(linkToken), Status: "reserved", ExpiresAt: &expiry, CreatedAt: now}
		if err := s.repo.Insert(tx, booking); err != nil {
			return err
		}
		mail := &models.ReservationMailOutbox{ID: security.UUID(), Recipient: booking.Email, Locale: booking.Locale, Kind: "created", WishTitle: c.Wish.Title, ReservationID: &booking.ID, CreatedAt: now, AvailableAt: now}
		if c.List != nil {
			mail.ListTitle = c.List.Title
		}
		if err := s.repo.Queue(tx, mail); err != nil {
			return err
		}
		return s.repo.DeleteChallenge(tx, id)
	})
	if err != nil {
		return err
	}
	return outcome
}

// Recover does not disclose whether the address matches a guest reservation.
func (s *Reservations) Recover(ctx context.Context, input ReservationInput) error {
	email, err := normalizeEmail(input.Email)
	if err != nil {
		return err
	}
	if err := s.auth.limit(ctx, "reservation-recover-email", email, 5, time.Hour); err != nil {
		return err
	}
	return s.tx.WithTransaction(ctx, func(tx context.Context) error {
		c, err := s.context(tx, input)
		if err != nil {
			return err
		}
		booking, err := s.repo.ByWish(tx, input.WishID)
		if err != nil {
			return err
		}
		if !bookingActive(booking, time.Now()) || booking.UserID != nil || booking.Email != email {
			return nil
		}
		now := time.Now().UTC()
		mail := &models.ReservationMailOutbox{ID: security.UUID(), Recipient: booking.Email, Locale: booking.Locale, Kind: "recovery", WishTitle: c.Wish.Title, ReservationID: &booking.ID, CreatedAt: now, AvailableAt: now}
		if c.List != nil {
			mail.ListTitle = c.List.Title
		}
		return s.repo.Queue(tx, mail)
	})
}

type ReservationView struct {
	WishTitle string     `json:"wish_title"`
	ListTitle string     `json:"list_title"`
	Status    string     `json:"status"`
	ExpiresAt *time.Time `json:"expires_at"`
	CanExtend bool       `json:"can_extend"`
}

func (s *Reservations) Manage(ctx context.Context, token, action string) (*ReservationView, error) {
	if len(token) != 43 {
		return nil, ErrReservationUnavailable
	}
	var result *ReservationView
	err := s.tx.WithTransaction(ctx, func(tx context.Context) error {
		result = nil
		booking, err := s.repo.ByToken(tx, security.Digest(token))
		if err != nil {
			return err
		}
		now := time.Now().UTC()
		if !bookingActive(booking, now) {
			return ErrReservationExpired
		}
		wish, list, err := s.repo.Details(tx, booking)
		if err != nil {
			return err
		}
		if wish == nil {
			return ErrReservationUnavailable
		}
		var event *time.Time
		if list != nil && list.EventDate != nil {
			end, e := reservation.EventEnd(list.EventDate.Format("2006-01-02"), list.EventTimezone)
			if e != nil {
				return e
			}
			event = &end
		}
		switch action {
		case "":
		case "cancel":
			return s.repo.Delete(tx, booking.WishID)
		case "purchase":
			if booking.Status != "reserved" {
				return ErrReservationAction
			}
			booking.Status, booking.ExpiresAt = "purchased", nil
			if err := s.repo.Update(tx, booking); err != nil {
				return err
			}
		case "extend", "undo_purchase":
			if (action == "extend" && booking.Status != "reserved") || (action == "undo_purchase" && booking.Status != "purchased") {
				return ErrReservationAction
			}
			if event != nil && !now.Before(*event) {
				return ErrReservationExpired
			}
			deadline := reservation.ExtendedDeadline(now, event)
			if action == "extend" && !deadline.After(*booking.ExpiresAt) {
				return ErrReservationAction
			}
			booking.Status, booking.ExpiresAt = "reserved", &deadline
			if err := s.repo.Update(tx, booking); err != nil {
				return err
			}
		default:
			return ErrReservationAction
		}
		result = &ReservationView{WishTitle: wish.Title, Status: booking.Status, ExpiresAt: booking.ExpiresAt}
		if list != nil {
			result.ListTitle = list.Title
		}
		result.CanExtend = booking.Status == "reserved" && (event == nil || booking.ExpiresAt.Before(*event))
		return nil
	})
	return result, err
}

func ReservationErrorCode(err error) string {
	var e ReservationError
	if errors.As(err, &e) {
		return string(e)
	}
	return ErrorCode(err)
}

func (s *Reservations) ProcessNotifications(ctx context.Context) error {
	if err := s.tx.WithTransaction(ctx, func(tx context.Context) error { return s.repo.ScheduleReminder(tx, time.Now().UTC()) }); err != nil {
		return err
	}
	for i := 0; i < 20; i++ {
		var item *models.ReservationMailOutbox
		if err := s.tx.WithTransaction(ctx, func(tx context.Context) error {
			var err error
			item, err = s.repo.ClaimMail(tx, time.Now().UTC())
			return err
		}); err != nil {
			return err
		}
		if item == nil {
			return nil
		}
		subject, body := s.notification(item)
		deliveryErr := s.messages.SendMessage(ctx, item.Recipient, subject, body)
		if err := s.repo.FinishMail(ctx, item.ID, item.Attempts, time.Now().UTC(), deliveryErr == nil); err != nil {
			return err
		}
		if deliveryErr != nil {
			return deliveryErr
		}
	}
	return nil
}

func (s *Reservations) notification(item *models.ReservationMailOutbox) (string, string) {
	var subject, intro, dateLabel, linkLabel string
	switch item.Locale {
	case "ru":
		subject, dateLabel, linkLabel = "Wishix — информация о бронировании", "Новая дата", "Управление бронью"
		switch item.Kind {
		case "created":
			intro = "Вы забронировали подарок"
		case "recovery":
			intro = "Ссылка для управления бронью"
		case "reminder":
			intro = "Срок бронирования скоро истекает"
		case "date_changed":
			intro = "Автор изменил дату события"
		default:
			intro = "Желание или список удалены, бронирование отменено"
		}
		if item.Kind == "date_changed" && item.EventDate == "" {
			dateLabel = "Дата события удалена"
		}
	case "ro":
		subject, dateLabel, linkLabel = "Wishix — informații despre rezervare", "Data nouă", "Gestionează rezervarea"
		switch item.Kind {
		case "created":
			intro = "Ai rezervat un cadou"
		case "recovery":
			intro = "Linkul pentru gestionarea rezervării"
		case "reminder":
			intro = "Rezervarea expiră în curând"
		case "date_changed":
			intro = "Autorul a schimbat data evenimentului"
		default:
			intro = "Dorința sau lista a fost ștearsă, iar rezervarea a fost anulată"
		}
		if item.Kind == "date_changed" && item.EventDate == "" {
			dateLabel = "Data evenimentului a fost eliminată"
		}
	default:
		subject, dateLabel, linkLabel = "Wishix — reservation update", "New date", "Manage your reservation"
		switch item.Kind {
		case "created":
			intro = "You reserved a gift"
		case "recovery":
			intro = "Your reservation management link"
		case "reminder":
			intro = "Your reservation expires soon"
		case "date_changed":
			intro = "The list owner changed the event date"
		default:
			intro = "The wish or list was deleted, so your reservation was cancelled"
		}
		if item.Kind == "date_changed" && item.EventDate == "" {
			dateLabel = "The event date was removed"
		}
	}
	body := fmt.Sprintf("%s: %s\r\n", intro, item.WishTitle)
	if item.ListTitle != "" {
		body += item.ListTitle + "\r\n"
	}
	if item.Message != "" {
		body += "\r\n" + item.Message + "\r\n"
	}
	if item.Kind == "date_changed" {
		body += dateLabel
		if item.EventDate != "" {
			body += ": " + item.EventDate
		}
		body += "\r\n"
	}
	if item.ReservationID != nil {
		body += "\r\n" + linkLabel + ": " + s.config.Origin + "/reservation#" + reservation.ManagementToken(s.config.Secret, *item.ReservationID) + "\r\n"
	}
	return subject, body
}
