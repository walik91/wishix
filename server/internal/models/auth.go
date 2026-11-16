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

type AuthGoogleIdentity struct {
	Subject   string `gorm:"primaryKey"`
	UserID    string
	CreatedAt time.Time
}

type AuthGoogleFlow struct {
	StateHash   string `gorm:"primaryKey"`
	Verifier    string
	Mode        string
	SessionHash string
	ExpiresAt   time.Time
}

type AuthGooglePending struct {
	TokenHash string `gorm:"primaryKey"`
	Subject   string
	Email     string
	ExpiresAt time.Time
}

func (AuthGooglePending) TableName() string { return "auth_google_pending" }

type AuthFacebookIdentity struct {
	Subject   string `gorm:"primaryKey"`
	UserID    string
	CreatedAt time.Time
}

type AuthFacebookFlow struct {
	StateHash   string `gorm:"primaryKey"`
	Verifier    string
	Mode        string
	SessionHash string
	ExpiresAt   time.Time
}

type AuthFacebookPending struct {
	TokenHash string `gorm:"primaryKey"`
	Subject   string
	Email     string
	ExpiresAt time.Time
}

func (AuthFacebookPending) TableName() string { return "auth_facebook_pending" }
