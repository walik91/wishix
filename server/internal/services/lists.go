package services

import (
	"context"
	"errors"
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	security "wishix/server/internal/auth"
	"wishix/server/internal/models"
)

type ListError string

func (e ListError) Error() string { return string(e) }

const (
	ErrListNotFound    ListError = "list_not_found"
	ErrListTitle       ListError = "list_title_invalid"
	ErrListDescription ListError = "list_description_invalid"
	ErrListDate        ListError = "list_date_invalid"
	ErrListVisibility  ListError = "list_visibility_invalid"
	ErrListSelection   ListError = "list_selection_invalid"
	ErrListChanged     ListError = "list_changed"
)

type ListInput struct {
	Title       string  `json:"title"`
	Description string  `json:"description"`
	EventDate   *string `json:"event_date"`
	Visibility  string  `json:"visibility"`
	Revision    int64   `json:"revision"`
}

type ListRepository interface {
	List(context.Context, string) ([]models.Wishlist, error)
	Get(context.Context, string, string) (*models.Wishlist, error)
	Create(context.Context, *models.Wishlist) error
	Update(context.Context, *models.Wishlist, int64) (bool, error)
	Lock(context.Context, string, string) (*models.Wishlist, error)
	Delete(context.Context, string, string) error
	Wishes(context.Context, string, string) ([]models.Wish, error)
	OwnedWishes(context.Context, string, []string) (bool, error)
	Add(context.Context, string, string, []string, time.Time) error
	Remove(context.Context, string, string, []string) error
	Clear(context.Context, string, string) error
	Touch(context.Context, string, []string, time.Time) error
	Copy(context.Context, string, string, string, time.Time) error
	Memberships(context.Context, string, string) ([]string, int64, error)
	ReplaceMemberships(context.Context, string, string, []string, int64, time.Time) (bool, error)
	Shared(context.Context, string) (*models.Wishlist, string, error)
	SharedWishes(context.Context, string) ([]models.Wish, error)
	SharedImage(context.Context, string, string) (*models.WishImage, error)
	PublicProfile(context.Context, string) (string, []models.Wishlist, error)
}

type Lists struct {
	repo ListRepository
	tx   Transactor
}

func NewLists(repo ListRepository, tx Transactor) *Lists { return &Lists{repo: repo, tx: tx} }

func validateList(input ListInput) (ListInput, *time.Time, error) {
	input.Title = strings.TrimSpace(input.Title)
	input.Description = strings.TrimSpace(input.Description)
	if !utf8.ValidString(input.Title) || utf8.RuneCountInString(input.Title) < 1 || utf8.RuneCountInString(input.Title) > 120 || strings.ContainsRune(input.Title, 0) {
		return input, nil, ErrListTitle
	}
	if !utf8.ValidString(input.Description) || utf8.RuneCountInString(input.Description) > 2000 || strings.ContainsRune(input.Description, 0) {
		return input, nil, ErrListDescription
	}
	if input.Visibility == "" {
		input.Visibility = "link"
	}
	if input.Visibility != "private" && input.Visibility != "link" && input.Visibility != "public" {
		return input, nil, ErrListVisibility
	}
	var date *time.Time
	if input.EventDate != nil && *input.EventDate != "" {
		parsed, err := time.Parse("2006-01-02", *input.EventDate)
		if err != nil || parsed.Year() < 1 || parsed.Year() > 9999 {
			return input, nil, ErrListDate
		}
		date = &parsed
	}
	return input, date, nil
}

func validIDs(ids []string) ([]string, error) {
	if len(ids) > 1000 {
		return nil, ErrListSelection
	}
	set := map[string]bool{}
	result := make([]string, 0, len(ids))
	for _, id := range ids {
		if !wishIDPattern.MatchString(id) {
			return nil, ErrListSelection
		}
		id = strings.ToLower(id)
		if !set[id] {
			set[id] = true
			result = append(result, id)
		}
	}
	sort.Strings(result)
	return result, nil
}

func (s *Lists) List(ctx context.Context, owner string) ([]models.Wishlist, error) {
	if owner == "" {
		return nil, ErrSession
	}
	return s.repo.List(ctx, owner)
}
func (s *Lists) Get(ctx context.Context, owner, id string) (*models.Wishlist, []models.Wish, error) {
	if owner == "" {
		return nil, nil, ErrSession
	}
	if !wishIDPattern.MatchString(id) {
		return nil, nil, ErrListNotFound
	}
	var list *models.Wishlist
	var wishes []models.Wish
	err := s.tx.WithTransaction(ctx, func(ctx context.Context) error {
		var err error
		list, err = s.repo.Get(ctx, owner, id)
		if err != nil {
			return err
		}
		if list == nil {
			return ErrListNotFound
		}
		wishes, err = s.repo.Wishes(ctx, owner, id)
		return err
	})
	return list, wishes, err
}
func (s *Lists) Create(ctx context.Context, owner string, input ListInput) (*models.Wishlist, error) {
	if owner == "" {
		return nil, ErrSession
	}
	input, date, err := validateList(input)
	if err != nil {
		return nil, err
	}
	id, token, now := security.UUID(), security.Token(), time.Now().UTC()
	var result *models.Wishlist
	err = s.tx.WithTransaction(ctx, func(ctx context.Context) error {
		result = &models.Wishlist{ID: id, OwnerID: owner, Title: input.Title, Description: input.Description, EventDate: date, Visibility: input.Visibility, ShareToken: token, Revision: 1, CreatedAt: now, UpdatedAt: now}
		return s.repo.Create(ctx, result)
	})
	return result, err
}
func (s *Lists) Update(ctx context.Context, owner, id string, input ListInput) (*models.Wishlist, error) {
	if owner == "" {
		return nil, ErrSession
	}
	if !wishIDPattern.MatchString(id) {
		return nil, ErrListNotFound
	}
	input, date, err := validateList(input)
	if err != nil {
		return nil, err
	}
	var result *models.Wishlist
	err = s.tx.WithTransaction(ctx, func(ctx context.Context) error {
		list, err := s.lock(ctx, owner, id, input.Revision)
		if err != nil {
			return err
		}
		list.Title, list.Description, list.EventDate, list.Visibility = input.Title, input.Description, date, input.Visibility
		list.UpdatedAt = time.Now().UTC()
		ok, err := s.repo.Update(ctx, list, input.Revision)
		if err != nil {
			return err
		}
		if !ok {
			return ErrListChanged
		}
		list.Revision++
		result = list
		return nil
	})
	return result, err
}
func (s *Lists) lock(ctx context.Context, owner, id string, revision int64) (*models.Wishlist, error) {
	list, err := s.repo.Lock(ctx, owner, id)
	if err != nil {
		return nil, err
	}
	if list == nil {
		return nil, ErrListNotFound
	}
	if revision > 0 && revision != list.Revision {
		return nil, ErrListChanged
	}
	return list, nil
}
func (s *Lists) Destroy(ctx context.Context, owner, id string, revision int64, clearOnly bool) error {
	if owner == "" {
		return ErrSession
	}
	if !wishIDPattern.MatchString(id) {
		return ErrListNotFound
	}
	if revision < 1 {
		return ErrListChanged
	}
	return s.tx.WithTransaction(ctx, func(ctx context.Context) error {
		if _, err := s.lock(ctx, owner, id, revision); err != nil {
			return err
		}
		if clearOnly {
			if err := s.repo.Clear(ctx, owner, id); err != nil {
				return err
			}
			return s.repo.Touch(ctx, owner, []string{id}, time.Now().UTC())
		}
		return s.repo.Delete(ctx, owner, id)
	})
}
func (s *Lists) Add(ctx context.Context, owner, id string, ids []string) error {
	if owner == "" {
		return ErrSession
	}
	if !wishIDPattern.MatchString(id) {
		return ErrListNotFound
	}
	ids, err := validIDs(ids)
	if err != nil || len(ids) == 0 {
		return ErrListSelection
	}
	return s.tx.WithTransaction(ctx, func(ctx context.Context) error {
		if _, err := s.lock(ctx, owner, id, 0); err != nil {
			return err
		}
		owned, err := s.repo.OwnedWishes(ctx, owner, ids)
		if err != nil {
			return err
		}
		if !owned {
			return ErrListSelection
		}
		now := time.Now().UTC()
		if err := s.repo.Add(ctx, owner, id, ids, now); err != nil {
			return err
		}
		return s.repo.Touch(ctx, owner, []string{id}, now)
	})
}
func (s *Lists) Remove(ctx context.Context, owner, id, wishID string) error {
	if owner == "" {
		return ErrSession
	}
	if !wishIDPattern.MatchString(id) {
		return ErrListNotFound
	}
	if !wishIDPattern.MatchString(wishID) {
		return ErrListSelection
	}
	return s.tx.WithTransaction(ctx, func(ctx context.Context) error {
		if _, err := s.lock(ctx, owner, id, 0); err != nil {
			return err
		}
		if err := s.repo.Remove(ctx, owner, id, []string{wishID}); err != nil {
			return err
		}
		return s.repo.Touch(ctx, owner, []string{id}, time.Now().UTC())
	})
}
func (s *Lists) Copy(ctx context.Context, owner, id, title string) (*models.Wishlist, error) {
	if owner == "" {
		return nil, ErrSession
	}
	if !wishIDPattern.MatchString(id) {
		return nil, ErrListNotFound
	}
	input, _, err := validateList(ListInput{Title: title})
	if err != nil {
		return nil, err
	}
	newID, token, now := security.UUID(), security.Token(), time.Now().UTC()
	var result *models.Wishlist
	err = s.tx.WithTransaction(ctx, func(ctx context.Context) error {
		original, err := s.lock(ctx, owner, id, 0)
		if err != nil {
			return err
		}
		result = &models.Wishlist{ID: newID, OwnerID: owner, Title: input.Title, Description: original.Description, Visibility: "link", ShareToken: token, Revision: 1, CreatedAt: now, UpdatedAt: now}
		if err := s.repo.Create(ctx, result); err != nil {
			return err
		}
		if err := s.repo.Copy(ctx, owner, id, newID, now); err != nil {
			return err
		}
		result.WishCount = original.WishCount
		return nil
	})
	return result, err
}
func (s *Lists) Rotate(ctx context.Context, owner, id string, revision int64) (*models.Wishlist, error) {
	if owner == "" {
		return nil, ErrSession
	}
	if !wishIDPattern.MatchString(id) {
		return nil, ErrListNotFound
	}
	if revision < 1 {
		return nil, ErrListChanged
	}
	var result *models.Wishlist
	token := security.Token()
	err := s.tx.WithTransaction(ctx, func(ctx context.Context) error {
		list, err := s.lock(ctx, owner, id, revision)
		if err != nil {
			return err
		}
		list.ShareToken = token
		list.UpdatedAt = time.Now().UTC()
		ok, err := s.repo.Update(ctx, list, revision)
		if err != nil {
			return err
		}
		if !ok {
			return ErrListChanged
		}
		list.Revision++
		result = list
		return nil
	})
	return result, err
}
func (s *Lists) Memberships(ctx context.Context, owner, wishID string) ([]string, int64, error) {
	if owner == "" {
		return nil, 0, ErrSession
	}
	if !wishIDPattern.MatchString(wishID) {
		return nil, 0, ErrWishNotFound
	}
	var ids []string
	var version int64
	err := s.tx.WithTransaction(ctx, func(ctx context.Context) error {
		var err error
		ids, version, err = s.repo.Memberships(ctx, owner, wishID)
		return err
	})
	if err == nil && version == 0 {
		err = ErrWishNotFound
	}
	return ids, version, err
}
func (s *Lists) SetMemberships(ctx context.Context, owner, wishID string, ids []string, version int64) error {
	if owner == "" {
		return ErrSession
	}
	if !wishIDPattern.MatchString(wishID) {
		return ErrWishNotFound
	}
	ids, err := validIDs(ids)
	if err != nil {
		return err
	}
	if version < 1 {
		return ErrListChanged
	}
	return s.tx.WithTransaction(ctx, func(ctx context.Context) error {
		ok, err := s.repo.ReplaceMemberships(ctx, owner, wishID, ids, version, time.Now().UTC())
		if err != nil {
			return err
		}
		if !ok {
			return ErrListChanged
		}
		return nil
	})
}

var sharePattern = regexp.MustCompile(`^[A-Za-z0-9_-]{43}$`)

// ResolveShare checks visibility before exposing a canonical owner in a redirect.
func (s *Lists) ResolveShare(ctx context.Context, token string) (*models.Wishlist, string, error) {
	if !sharePattern.MatchString(token) {
		return nil, "", ErrListNotFound
	}
	list, username, err := s.repo.Shared(ctx, token)
	if err == nil && list == nil {
		return nil, "", ErrListNotFound
	}
	return list, username, err
}

func (s *Lists) Shared(ctx context.Context, token string) (*models.Wishlist, string, []models.Wish, error) {
	if !sharePattern.MatchString(token) {
		return nil, "", nil, ErrListNotFound
	}
	var list *models.Wishlist
	var username string
	var wishes []models.Wish
	err := s.tx.WithTransaction(ctx, func(ctx context.Context) error {
		var err error
		list, username, err = s.repo.Shared(ctx, token)
		if err != nil {
			return err
		}
		if list == nil {
			return ErrListNotFound
		}
		wishes, err = s.repo.SharedWishes(ctx, token)
		return err
	})
	return list, username, wishes, err
}
func (s *Lists) SharedImage(ctx context.Context, token, wishID string) (*models.WishImage, error) {
	if !sharePattern.MatchString(token) || !wishIDPattern.MatchString(wishID) {
		return nil, ErrListNotFound
	}
	image, err := s.repo.SharedImage(ctx, token, wishID)
	if err == nil && image == nil {
		err = ErrListNotFound
	}
	return image, err
}
func (s *Lists) PublicProfile(ctx context.Context, username string) (string, []models.Wishlist, error) {
	if !regexp.MustCompile(`^[A-Za-z0-9_]{3,30}$`).MatchString(username) {
		return "", nil, ErrListNotFound
	}
	name, lists, err := s.repo.PublicProfile(ctx, strings.ToLower(username))
	if err == nil && name == "" {
		err = ErrListNotFound
	}
	return name, lists, err
}
func ListErrorCode(err error) string {
	var listError ListError
	if errors.As(err, &listError) {
		return string(listError)
	}
	return WishErrorCode(err)
}
