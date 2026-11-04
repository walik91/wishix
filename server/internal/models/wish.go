package models

import "time"

// A wish belongs to the account's collection, independently of future lists.
type Wish struct {
	ID          string
	OwnerID     string
	Title       string
	Description string
	Price       *string
	Currency    string
	Priority    string
	CreatedAt   time.Time
	ShopLinks   []WishShopLink `gorm:"foreignKey:WishID"`
}

type WishShopLink struct {
	WishID   string `gorm:"primaryKey"`
	Position int    `gorm:"primaryKey;autoIncrement:false"`
	URL      string
}
