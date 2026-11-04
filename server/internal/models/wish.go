package models

import "time"

// A wish belongs to the account's collection; list memberships share its identity.
type Wish struct {
	ID           string
	OwnerID      string
	Title        string
	Description  string
	Price        *string
	Currency     string
	Priority     string
	CreatedAt    time.Time
	ListsVersion int64          `gorm:"default:1"`
	Members      []WishlistWish `gorm:"-"`
	ShopLinks    []WishShopLink `gorm:"foreignKey:WishID"`
	ImageVersion string         `gorm:"->"`
	Image        *WishImage     `gorm:"foreignKey:WishID"`
}

type WishImage struct {
	WishID  string `gorm:"primaryKey"`
	Version string
	Data    []byte
}

type WishShopLink struct {
	WishID   string `gorm:"primaryKey"`
	Position int    `gorm:"primaryKey;autoIncrement:false"`
	URL      string
}
