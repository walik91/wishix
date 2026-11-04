package models

import "time"

// Table models are private persistence data, never HTTP response DTOs.
type User struct {
	ID              string
	Username        string
	Email           string
	PasswordHash    string
	EmailVerifiedAt time.Time
	CreatedAt       time.Time
	ShowAllWishes   bool
}

type AuthChallenge struct {
	ID           string
	Purpose      string
	Email        string
	Username     string
	PasswordHash string
	UserID       *string
	Locale       string
	CodeHash     string
	Attempts     int
	ExpiresAt    time.Time
	SentAt       time.Time
}

type AuthSession struct {
	TokenHash string
	UserID    string
	ExpiresAt time.Time
	CreatedAt time.Time
}
