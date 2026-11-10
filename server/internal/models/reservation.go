package models

import "time"

// These records are private and must never be serialized into public responses.
type Reservation struct {
	WishID          string `gorm:"primaryKey"`
	ID              string
	ListID          *string
	CollectionOwner *string
	ContextKey      string
	Email           string
	Locale          string
	UserID          *string
	TokenHash       string
	Status          string
	ExpiresAt       *time.Time
	CreatedAt       time.Time
}

type ReservationChallenge struct {
	ID              string `gorm:"primaryKey"`
	WishID          string
	ListID          *string
	CollectionOwner *string
	AccessHash      string
	Email           string
	Locale          string
	UserID          *string
	CodeHash        string
	Attempts        int
	ExpiresAt       time.Time
	CreatedAt       time.Time
}

type ReservationMailOutbox struct {
	ID            string `gorm:"primaryKey"`
	Recipient     string
	Locale        string
	Kind          string
	WishTitle     string
	ListTitle     string
	EventDate     string
	Message       string
	ReservationID *string
	CreatedAt     time.Time
	AvailableAt   time.Time
	Attempts      int
	LeaseUntil    *time.Time
	SentAt        *time.Time
}

func (ReservationMailOutbox) TableName() string { return "reservation_mail_outbox" }
