package repositories

import (
	"context"

	"gorm.io/gorm"

	"wishix/server/internal/db"
	"wishix/server/internal/models"
)

type Wishes struct{ Base }

func NewWishes(executor *db.Executor) *Wishes { return &Wishes{Base: NewBase(executor)} }

func (r *Wishes) List(ctx context.Context, ownerID string) ([]models.Wish, error) {
	wishes := make([]models.Wish, 0)
	err := r.Ctx(ctx).Where("owner_id = ?", ownerID).
		Order("created_at DESC, id DESC").
		Preload("ShopLinks", func(query *gorm.DB) *gorm.DB { return query.Order("position ASC") }).Find(&wishes).Error
	return wishes, err
}

func (r *Wishes) Create(ctx context.Context, wish *models.Wish) error {
	if err := r.Ctx(ctx).Omit("ShopLinks").Create(wish).Error; err != nil {
		return err
	}
	if len(wish.ShopLinks) != 0 {
		return r.Ctx(ctx).Create(&wish.ShopLinks).Error
	}
	return nil
}

func (r *Wishes) Delete(ctx context.Context, ownerID, id string) (bool, error) {
	result := r.Ctx(ctx).Where("id = ? AND owner_id = ?", id, ownerID).Delete(&models.Wish{})
	return result.RowsAffected == 1, result.Error
}
