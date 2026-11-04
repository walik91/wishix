package repositories

import (
	"context"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"wishix/server/internal/db"
	"wishix/server/internal/models"
	"wishix/server/internal/services"
)

type Lists struct{ Base }

func NewLists(executor *db.Executor) *Lists { return &Lists{Base: NewBase(executor)} }

const listColumns = "wishlists.*, (SELECT count(*) FROM wishlist_wishes WHERE list_id = wishlists.id) AS wish_count"
const wishColumns = "wishes.*, COALESCE((SELECT version::STRING FROM wish_images WHERE wish_id = wishes.id), '') AS image_version"

func (r *Lists) List(ctx context.Context, owner string) ([]models.Wishlist, error) {
	lists := make([]models.Wishlist, 0)
	err := r.Ctx(ctx).Select(listColumns).Where("owner_id = ?", owner).Order("created_at DESC, id DESC").Find(&lists).Error
	return lists, err
}
func (r *Lists) Get(ctx context.Context, owner, id string) (*models.Wishlist, error) {
	return r.get(ctx, owner, id, false)
}
func (r *Lists) Lock(ctx context.Context, owner, id string) (*models.Wishlist, error) {
	return r.get(ctx, owner, id, true)
}
func (r *Lists) get(ctx context.Context, owner, id string, lock bool) (*models.Wishlist, error) {
	var lists []models.Wishlist
	query := r.Ctx(ctx).Select(listColumns).Where("owner_id = ? AND id = ?", owner, id)
	if lock {
		query = query.Clauses(clause.Locking{Strength: "UPDATE"})
	}
	err := query.Limit(1).Find(&lists).Error
	if err != nil || len(lists) == 0 {
		return nil, err
	}
	return &lists[0], nil
}
func (r *Lists) Create(ctx context.Context, list *models.Wishlist) error {
	return r.Ctx(ctx).Create(list).Error
}
func (r *Lists) Update(ctx context.Context, list *models.Wishlist, revision int64) (bool, error) {
	result := r.Ctx(ctx).Model(&models.Wishlist{}).Where("id = ? AND owner_id = ? AND revision = ?", list.ID, list.OwnerID, revision).Updates(map[string]any{"title": list.Title, "description": list.Description, "event_date": list.EventDate, "visibility": list.Visibility, "share_token": list.ShareToken, "updated_at": list.UpdatedAt, "revision": gorm.Expr("revision + 1")})
	return result.RowsAffected == 1, result.Error
}
func (r *Lists) Delete(ctx context.Context, owner, id string) error {
	if err := r.Clear(ctx, owner, id); err != nil {
		return err
	}
	return r.Ctx(ctx).Where("owner_id = ? AND id = ?", owner, id).Delete(&models.Wishlist{}).Error
}
func (r *Lists) Wishes(ctx context.Context, owner, id string) ([]models.Wish, error) {
	wishes := make([]models.Wish, 0)
	err := r.Ctx(ctx).Select(wishColumns).Joins("JOIN wishlist_wishes m ON m.wish_id = wishes.id AND m.owner_id = wishes.owner_id").Where("m.list_id = ? AND m.owner_id = ?", id, owner).Order("wishes.created_at DESC, wishes.id DESC").Preload("ShopLinks", func(q *gorm.DB) *gorm.DB { return q.Order("position ASC") }).Find(&wishes).Error
	return wishes, err
}
func (r *Lists) OwnedWishes(ctx context.Context, owner string, ids []string) (bool, error) {
	var count int64
	err := r.Ctx(ctx).Model(&models.Wish{}).Where("owner_id = ? AND id IN ?", owner, ids).Count(&count).Error
	return count == int64(len(ids)), err
}
func (r *Lists) Add(ctx context.Context, owner, id string, ids []string, now time.Time) error {
	members := make([]models.WishlistWish, 0, len(ids))
	for _, wishID := range ids {
		members = append(members, models.WishlistWish{ListID: id, WishID: wishID, OwnerID: owner, AddedAt: now})
	}
	if len(members) == 0 {
		return nil
	}
	if err := r.Ctx(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(&members).Error; err != nil {
		return err
	}
	return r.Ctx(ctx).Model(&models.Wish{}).Where("owner_id = ? AND id IN ?", owner, ids).UpdateColumn("lists_version", gorm.Expr("lists_version + 1")).Error
}
func (r *Lists) Remove(ctx context.Context, owner, id string, ids []string) error {
	if err := r.Ctx(ctx).Model(&models.Wish{}).Where("owner_id = ? AND id IN ? AND id IN (SELECT wish_id FROM wishlist_wishes WHERE list_id = ? AND owner_id = ?)", owner, ids, id, owner).UpdateColumn("lists_version", gorm.Expr("lists_version + 1")).Error; err != nil {
		return err
	}
	return r.Ctx(ctx).Where("owner_id = ? AND list_id = ? AND wish_id IN ?", owner, id, ids).Delete(&models.WishlistWish{}).Error
}
func (r *Lists) Clear(ctx context.Context, owner, id string) error {
	if err := r.Ctx(ctx).Model(&models.Wish{}).Where("owner_id = ? AND id IN (SELECT wish_id FROM wishlist_wishes WHERE list_id = ? AND owner_id = ?)", owner, id, owner).UpdateColumn("lists_version", gorm.Expr("lists_version + 1")).Error; err != nil {
		return err
	}
	return r.Ctx(ctx).Where("owner_id = ? AND list_id = ?", owner, id).Delete(&models.WishlistWish{}).Error
}
func (r *Lists) Touch(ctx context.Context, owner string, ids []string, now time.Time) error {
	if len(ids) == 0 {
		return nil
	}
	return r.Ctx(ctx).Model(&models.Wishlist{}).Where("owner_id = ? AND id IN ?", owner, ids).Updates(map[string]any{"revision": gorm.Expr("revision + 1"), "updated_at": now}).Error
}
func (r *Lists) Copy(ctx context.Context, owner, source, target string, now time.Time) error {
	if err := r.Ctx(ctx).Exec(`INSERT INTO wishlist_wishes (list_id, wish_id, owner_id, added_at) SELECT ?::UUID, wish_id, owner_id, ? FROM wishlist_wishes WHERE list_id = ? AND owner_id = ?`, target, now, source, owner).Error; err != nil {
		return err
	}
	return r.Ctx(ctx).Model(&models.Wish{}).Where("owner_id = ? AND id IN (SELECT wish_id FROM wishlist_wishes WHERE list_id = ? AND owner_id = ?)", owner, source, owner).UpdateColumn("lists_version", gorm.Expr("lists_version + 1")).Error
}
func (r *Lists) Memberships(ctx context.Context, owner, wishID string) ([]string, int64, error) {
	var wishes []models.Wish
	err := r.Ctx(ctx).Select("id, lists_version").Where("owner_id = ? AND id = ?", owner, wishID).Limit(1).Find(&wishes).Error
	if err != nil || len(wishes) == 0 {
		return nil, 0, err
	}
	ids := make([]string, 0)
	err = r.Ctx(ctx).Model(&models.WishlistWish{}).Where("owner_id = ? AND wish_id = ?", owner, wishID).Order("list_id ASC").Pluck("list_id", &ids).Error
	return ids, wishes[0].ListsVersion, err
}
func (r *Lists) ReplaceMemberships(ctx context.Context, owner, wishID string, ids []string, version int64, now time.Time) (bool, error) {
	previous, current, err := r.Memberships(ctx, owner, wishID)
	if err != nil {
		return false, err
	}
	if current == 0 {
		return false, services.ErrWishNotFound
	}
	if current != version {
		return false, nil
	}
	var count int64
	if len(ids) > 0 {
		if err := r.Ctx(ctx).Model(&models.Wishlist{}).Where("owner_id = ? AND id IN ?", owner, ids).Count(&count).Error; err != nil {
			return false, err
		}
		if count != int64(len(ids)) {
			return false, services.ErrListSelection
		}
	}
	result := r.Ctx(ctx).Model(&models.Wish{}).Where("id = ? AND owner_id = ? AND lists_version = ?", wishID, owner, version).UpdateColumn("lists_version", gorm.Expr("lists_version + 1"))
	if result.Error != nil || result.RowsAffected != 1 {
		return false, result.Error
	}
	if err := r.Ctx(ctx).Where("owner_id = ? AND wish_id = ?", owner, wishID).Delete(&models.WishlistWish{}).Error; err != nil {
		return false, err
	}
	for _, id := range ids {
		if err := r.Ctx(ctx).Create(&models.WishlistWish{ListID: id, WishID: wishID, OwnerID: owner, AddedAt: now}).Error; err != nil {
			return false, err
		}
	}
	affected := append(previous, ids...)
	return true, r.Touch(ctx, owner, affected, now)
}
func (r *Lists) Shared(ctx context.Context, token string) (*models.Wishlist, string, error) {
	var rows []struct {
		models.Wishlist
		Username string
	}
	err := r.Ctx(ctx).Model(&models.Wishlist{}).Select(listColumns+", users.username").Joins("JOIN users ON users.id = wishlists.owner_id").Where("share_token = ? AND visibility IN ('link', 'public')", token).Limit(1).Find(&rows).Error
	if err != nil || len(rows) == 0 {
		return nil, "", err
	}
	return &rows[0].Wishlist, rows[0].Username, nil
}
func (r *Lists) SharedWishes(ctx context.Context, token string) ([]models.Wish, error) {
	wishes := make([]models.Wish, 0)
	err := r.Ctx(ctx).Select(wishColumns).Joins("JOIN wishlist_wishes m ON m.wish_id = wishes.id AND m.owner_id = wishes.owner_id").Joins("JOIN wishlists l ON l.id = m.list_id AND l.owner_id = m.owner_id").Where("l.share_token = ? AND l.visibility IN ('link', 'public')", token).Order("wishes.created_at DESC, wishes.id DESC").Preload("ShopLinks", func(q *gorm.DB) *gorm.DB { return q.Order("position ASC") }).Find(&wishes).Error
	return wishes, err
}
func (r *Lists) SharedImage(ctx context.Context, token, wishID string) (*models.WishImage, error) {
	var images []models.WishImage
	err := r.Ctx(ctx).Model(&models.WishImage{}).Select("wish_images.*").Joins("JOIN wishlist_wishes m ON m.wish_id = wish_images.wish_id").Joins("JOIN wishlists l ON l.id = m.list_id AND l.owner_id = m.owner_id").Where("l.share_token = ? AND l.visibility IN ('link', 'public') AND m.wish_id = ?", token, wishID).Limit(1).Find(&images).Error
	if err != nil || len(images) == 0 {
		return nil, err
	}
	return &images[0], nil
}

// Every query repeats the access condition, including image and membership reads.
const guestCondition = "l.id = ? AND (l.visibility = 'public' OR (l.visibility = 'link' AND l.share_token = ? AND ? <> ''))"

func (r *Lists) GuestList(ctx context.Context, id, token string) (*models.Wishlist, string, error) {
	var rows []struct {
		models.Wishlist
		Username string
	}
	err := r.Ctx(ctx).Table("wishlists AS l").Select("l.*, (SELECT count(*) FROM wishlist_wishes WHERE list_id = l.id) AS wish_count, users.username").Joins("JOIN users ON users.id = l.owner_id").Where(guestCondition, id, token, token).Limit(1).Find(&rows).Error
	if err != nil || len(rows) == 0 {
		return nil, "", err
	}
	return &rows[0].Wishlist, rows[0].Username, nil
}
func (r *Lists) GuestWishes(ctx context.Context, id, token string) ([]models.Wish, error) {
	wishes := make([]models.Wish, 0)
	err := r.Ctx(ctx).Select(wishColumns).Joins("JOIN wishlist_wishes m ON m.wish_id = wishes.id AND m.owner_id = wishes.owner_id").Joins("JOIN wishlists l ON l.id = m.list_id AND l.owner_id = m.owner_id").Where(guestCondition, id, token, token).Order("wishes.created_at DESC, wishes.id DESC").Preload("ShopLinks", func(q *gorm.DB) *gorm.DB { return q.Order("position ASC") }).Find(&wishes).Error
	return wishes, err
}
func (r *Lists) GuestImage(ctx context.Context, id, token, wishID string) (*models.WishImage, error) {
	var images []models.WishImage
	err := r.Ctx(ctx).Model(&models.WishImage{}).Select("wish_images.*").Joins("JOIN wishlist_wishes m ON m.wish_id = wish_images.wish_id").Joins("JOIN wishlists l ON l.id = m.list_id AND l.owner_id = m.owner_id").Where(guestCondition+" AND m.wish_id = ?", id, token, token, wishID).Limit(1).Find(&images).Error
	if err != nil || len(images) == 0 {
		return nil, err
	}
	return &images[0], nil
}
func (r *Lists) PublicProfile(ctx context.Context, username string) (string, bool, []models.Wishlist, error) {
	var users []models.User
	err := r.Ctx(ctx).Select("id, username, show_all_wishes").Where("username = ?", username).Limit(1).Find(&users).Error
	if err != nil || len(users) == 0 {
		return "", false, nil, err
	}
	lists := make([]models.Wishlist, 0)
	err = r.Ctx(ctx).Select(listColumns).Where("owner_id = ? AND visibility = 'public'", users[0].ID).Order("created_at DESC, id DESC").Find(&lists).Error
	return users[0].Username, users[0].ShowAllWishes, lists, err
}

func (r *Lists) CollectionSetting(ctx context.Context, owner string) (bool, error) {
	var user models.User
	err := r.Ctx(ctx).Select("show_all_wishes").Where("id = ?", owner).Take(&user).Error
	return user.ShowAllWishes, err
}
func (r *Lists) SetCollectionSetting(ctx context.Context, owner string, enabled bool) error {
	return r.Ctx(ctx).Model(&models.User{}).Where("id = ?", owner).Update("show_all_wishes", enabled).Error
}
func (r *Lists) PublicCollection(ctx context.Context, username string) (bool, []models.Wish, error) {
	var count int64
	if err := r.Ctx(ctx).Model(&models.User{}).Where("username = ? AND show_all_wishes = true", username).Count(&count).Error; err != nil || count == 0 {
		return false, nil, err
	}
	wishes := make([]models.Wish, 0)
	err := r.Ctx(ctx).Select(wishColumns).Joins("JOIN users ON users.id = wishes.owner_id").Where("users.username = ? AND users.show_all_wishes = true", username).Order("wishes.created_at DESC, wishes.id DESC").Preload("ShopLinks", func(q *gorm.DB) *gorm.DB { return q.Order("position ASC") }).Find(&wishes).Error
	return true, wishes, err
}
func (r *Lists) PublicCollectionImage(ctx context.Context, username, wishID string) (*models.WishImage, error) {
	var images []models.WishImage
	err := r.Ctx(ctx).Model(&models.WishImage{}).Select("wish_images.*").Joins("JOIN wishes ON wishes.id = wish_images.wish_id").Joins("JOIN users ON users.id = wishes.owner_id").Where("users.username = ? AND users.show_all_wishes = true AND wishes.id = ?", username, wishID).Limit(1).Find(&images).Error
	if err != nil || len(images) == 0 {
		return nil, err
	}
	return &images[0], nil
}
