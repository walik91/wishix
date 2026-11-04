package repositories

import (
	"context"
	"time"

	"gorm.io/gorm"

	"wishix/server/internal/db"
	"wishix/server/internal/models"
)

type Wishes struct{ Base }

func NewWishes(executor *db.Executor) *Wishes { return &Wishes{Base: NewBase(executor)} }

func (r *Wishes) List(ctx context.Context, ownerID string) ([]models.Wish, error) {
	wishes := make([]models.Wish, 0)
	err := r.Ctx(ctx).Select("wishes.*, COALESCE((SELECT version::STRING FROM wish_images WHERE wish_id = wishes.id), '') AS image_version").Where("owner_id = ?", ownerID).
		Order("created_at DESC, id DESC").
		Preload("ShopLinks", func(query *gorm.DB) *gorm.DB { return query.Order("position ASC") }).Find(&wishes).Error
	return wishes, err
}

func (r *Wishes) Create(ctx context.Context, wish *models.Wish) error {
	if err := r.Ctx(ctx).Omit("ShopLinks", "Image").Create(wish).Error; err != nil {
		return err
	}
	if len(wish.ShopLinks) != 0 {
		if err := r.Ctx(ctx).Create(&wish.ShopLinks).Error; err != nil {
			return err
		}
	}
	if wish.Image != nil {
		if err := r.Ctx(ctx).Create(wish.Image).Error; err != nil {
			return err
		}
	}
	if len(wish.Members) > 0 {
		if err := r.Ctx(ctx).Create(&wish.Members).Error; err != nil {
			return err
		}
		ids := make([]string, 0, len(wish.Members))
		for _, member := range wish.Members {
			ids = append(ids, member.ListID)
		}
		return r.Ctx(ctx).Model(&models.Wishlist{}).Where("owner_id = ? AND id IN ?", wish.OwnerID, ids).Updates(map[string]any{"revision": gorm.Expr("revision + 1"), "updated_at": wish.CreatedAt}).Error
	}
	return nil
}

func (r *Wishes) ValidLists(ctx context.Context, owner string, ids []string) (bool, error) {
	var count int64
	err := r.Ctx(ctx).Model(&models.Wishlist{}).Where("owner_id = ? AND id IN ?", owner, ids).Count(&count).Error
	return count == int64(len(ids)), err
}

func (r *Wishes) Image(ctx context.Context, ownerID, id string) (*models.WishImage, error) {
	var images []models.WishImage
	err := r.Ctx(ctx).Model(&models.WishImage{}).Select("wish_images.*").Joins("JOIN wishes ON wishes.id = wish_images.wish_id").Where("wishes.owner_id = ? AND wishes.id = ?", ownerID, id).Limit(1).Find(&images).Error
	if err != nil || len(images) == 0 {
		return nil, err
	}
	return &images[0], nil
}

func (r *Wishes) SaveImage(ctx context.Context, ownerID string, image *models.WishImage) (bool, error) {
	result := r.Ctx(ctx).Exec(`INSERT INTO wish_images (wish_id, version, data)
		SELECT id, ?::UUID, ?::BYTES FROM wishes WHERE id = ? AND owner_id = ?
		ON CONFLICT (wish_id) DO UPDATE SET version = excluded.version, data = excluded.data`, image.Version, image.Data, image.WishID, ownerID)
	return result.RowsAffected == 1, result.Error
}

func (r *Wishes) Owns(ctx context.Context, ownerID, id string) (bool, error) {
	var count int64
	err := r.Ctx(ctx).Model(&models.Wish{}).Where("id = ? AND owner_id = ?", id, ownerID).Count(&count).Error
	return count == 1, err
}

func (r *Wishes) DeleteImage(ctx context.Context, ownerID, id string) (bool, error) {
	result := r.Ctx(ctx).Where("wish_id = ? AND wish_id IN (SELECT id FROM wishes WHERE owner_id = ?)", id, ownerID).Delete(&models.WishImage{})
	return result.RowsAffected == 1, result.Error
}

func (r *Wishes) Delete(ctx context.Context, ownerID, id string) (bool, error) {
	if err := r.Ctx(ctx).Model(&models.Wishlist{}).Where("owner_id = ? AND id IN (SELECT list_id FROM wishlist_wishes WHERE wish_id = ? AND owner_id = ?)", ownerID, id, ownerID).Updates(map[string]any{"revision": gorm.Expr("revision + 1"), "updated_at": time.Now().UTC()}).Error; err != nil {
		return false, err
	}
	result := r.Ctx(ctx).Where("id = ? AND owner_id = ?", id, ownerID).Delete(&models.Wish{})
	return result.RowsAffected == 1, result.Error
}
