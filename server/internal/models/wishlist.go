package models

import "time"

type Wishlist struct {
	ID          string
	OwnerID     string
	Title       string
	Description string
	EventDate   *time.Time
	Visibility  string
	ShareToken  string
	Revision    int64
	CreatedAt   time.Time
	UpdatedAt   time.Time
	WishCount   int64 `gorm:"->"`
}

type WishlistWish struct {
	ListID  string `gorm:"primaryKey"`
	WishID  string `gorm:"primaryKey"`
	OwnerID string
	AddedAt time.Time
}

func (WishlistWish) TableName() string { return "wishlist_wishes" }
