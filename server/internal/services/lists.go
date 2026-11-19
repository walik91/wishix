package services

import (
	"context"
	"errors"
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	security "wishix/server/internal/auth"
	"wishix/server/internal/models"
	"wishix/server/internal/reservation"
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
	Title         string  `json:"title"`
	Description   string  `json:"description"`
	EventDate     *string `json:"event_date"`
	EventTimezone string  `json:"event_timezone"`
	Visibility    string  `json:"visibility"`
	Revision      int64   `json:"revision"`
}

type ListRepository interface {
	List(context.Context, string) ([]models.Wishlist, error)
	Get(context.Context, string, string) (*models.Wishlist, error)
	Create(context.Context, *models.Wishlist) error
	Update(context.Context, *models.Wishlist, int64) (bool, error)
	ReservationDateChanged(context.Context, *models.Wishlist, *time.Time, time.Time) error
	Lock(context.Context, string, string) (*models.Wishlist, error)
	Delete(context.Context, string, string, string) error
	Wishes(context.Context, string, string) ([]models.Wish, error)
	OwnedWishes(context.Context, string, []string) (bool, error)
	Add(context.Context, string, string, []string, time.Time) error
	Remove(context.Context, string, string, []string) error
	SetReceived(context.Context, string, string, string, bool) (bool, error)
	Clear(context.Context, string, string) error
	Touch(context.Context, string, []string, time.Time) error
	Copy(context.Context, string, string, string, time.Time) error
	Memberships(context.Context, string, string) ([]string, int64, error)
	ReplaceMemberships(context.Context, string, string, []string, int64, time.Time) (bool, error)
	Shared(context.Context, string) (*models.Wishlist, string, error)
	SharedWishes(context.Context, string) ([]models.Wish, error)
	SharedImage(context.Context, string, string) (*models.WishImage, error)
	GuestList(context.Context, string, string) (*models.Wishlist, string, error)
	GuestWishes(context.Context, string, string) ([]models.Wish, error)
	GuestImage(context.Context, string, string, string) (*models.WishImage, error)
	PublicProfile(context.Context, string) (string, string, bool, []models.Wishlist, error)
	SearchPeople(context.Context, string, string, int) ([]PublicPerson, error)
	ProfileName(context.Context, string) (string, error)
	SetProfileName(context.Context, string, string) error
	CollectionSetting(context.Context, string) (bool, error)
	SetCollectionSetting(context.Context, string, bool) error
	PublicCollection(context.Context, string) (bool, []models.Wish, error)
	PublicCollectionImage(context.Context, string, string) (*models.WishImage, error)
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

// ListExpired uses the stored author timezone rather than the viewer's clock.
func ListExpired(list *models.Wishlist, now time.Time) bool {
	if list == nil || list.EventDate == nil {
		return false
	}
	zone := list.EventTimezone
	if zone == "" {
		zone = "UTC"
	}
	end, err := reservation.EventEnd(list.EventDate.Format("2006-01-02"), zone)
	// Fail closed if stored timezone data is malformed.
	return err != nil || !now.Before(end)
}

func validEvent(date *time.Time, zone string, now time.Time) bool {
	if date == nil {
		return true
	}
	end, err := reservation.EventEnd(date.Format("2006-01-02"), zone)
	return err == nil && now.Before(end)
}

func validEventZone(zone string) bool {
	_, err := reservation.EventEnd("2026-01-01", zone)
	return err == nil
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
	if input.EventTimezone == "" {
		input.EventTimezone = "UTC"
	}
	if !validEventZone(input.EventTimezone) || !validEvent(date, input.EventTimezone, time.Now()) {
		return nil, ErrListDate
	}
	id, token, now := security.UUID(), security.Token(), time.Now().UTC()
	var result *models.Wishlist
	err = s.tx.WithTransaction(ctx, func(ctx context.Context) error {
		result = &models.Wishlist{ID: id, OwnerID: owner, Title: input.Title, Description: input.Description, EventDate: date, EventTimezone: input.EventTimezone, Visibility: input.Visibility, ShareToken: token, Revision: 1, CreatedAt: now, UpdatedAt: now}
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
		zone := input.EventTimezone
		if zone == "" {
			zone = list.EventTimezone
		}
		if zone == "" {
			zone = "UTC"
		}
		if !validEventZone(zone) {
			return ErrListDate
		}
		unchangedDate := date != nil && list.EventDate != nil && date.Equal(*list.EventDate) && zone == list.EventTimezone
		if !unchangedDate && !validEvent(date, zone, time.Now()) {
			return ErrListDate
		}
		dateChanged := (date == nil) != (list.EventDate == nil) || (date != nil && list.EventDate != nil && !date.Equal(*list.EventDate)) || (date != nil && zone != list.EventTimezone)
		list.Title, list.Description, list.EventDate, list.Visibility = input.Title, input.Description, date, input.Visibility
		list.EventTimezone = zone
		list.UpdatedAt = time.Now().UTC()
		ok, err := s.repo.Update(ctx, list, input.Revision)
		if err != nil {
			return err
		}
		if !ok {
			return ErrListChanged
		}
		if dateChanged {
			var end *time.Time
			if date != nil {
				value, err := reservation.EventEnd(date.Format("2006-01-02"), zone)
				if err != nil {
					return err
				}
				end = &value
			}
			if err := s.repo.ReservationDateChanged(ctx, list, end, list.UpdatedAt); err != nil {
				return err
			}
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
func (s *Lists) Destroy(ctx context.Context, owner, id string, revision int64, clearOnly bool, message string) error {
	if owner == "" {
		return ErrSession
	}
	if !wishIDPattern.MatchString(id) {
		return ErrListNotFound
	}
	if revision < 1 {
		return ErrListChanged
	}
	message = strings.TrimSpace(message)
	if !utf8.ValidString(message) || utf8.RuneCountInString(message) > 1000 || strings.ContainsRune(message, 0) || (clearOnly && message != "") {
		return ErrInvalidInput
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
		return s.repo.Delete(ctx, owner, id, message)
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
func (s *Lists) SetReceived(ctx context.Context, owner, id, wishID string, revision int64, received bool) error {
	if owner == "" {
		return ErrSession
	}
	if !wishIDPattern.MatchString(id) {
		return ErrListNotFound
	}
	if !wishIDPattern.MatchString(wishID) {
		return ErrListSelection
	}
	if revision < 1 {
		return ErrListChanged
	}
	return s.tx.WithTransaction(ctx, func(ctx context.Context) error {
		if _, err := s.lock(ctx, owner, id, revision); err != nil {
			return err
		}
		found, err := s.repo.SetReceived(ctx, owner, id, wishID, received)
		if err != nil {
			return err
		}
		if !found {
			return ErrWishNotFound
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
		result = &models.Wishlist{ID: newID, OwnerID: owner, Title: input.Title, Description: original.Description, EventTimezone: original.EventTimezone, Visibility: "link", ShareToken: token, Revision: 1, CreatedAt: now, UpdatedAt: now}
		if err := s.repo.Create(ctx, result); err != nil {
			return err
		}
		if err := s.repo.Copy(ctx, owner, id, newID, now); err != nil {
			return err
		}
		wishes, err := s.repo.Wishes(ctx, owner, newID)
		if err != nil {
			return err
		}
		result.WishCount = int64(len(wishes))
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

func validGuestList(id, token string) bool {
	return wishIDPattern.MatchString(id) && (token == "" || sharePattern.MatchString(token))
}

func (s *Lists) ResolveGuest(ctx context.Context, id, token string) (*models.Wishlist, string, error) {
	if !validGuestList(id, token) {
		return nil, "", ErrListNotFound
	}
	list, username, err := s.repo.GuestList(ctx, id, token)
	if err == nil && list == nil {
		err = ErrListNotFound
	}
	return list, username, err
}

func (s *Lists) Guest(ctx context.Context, id, token string) (*models.Wishlist, string, []models.Wish, error) {
	if !validGuestList(id, token) {
		return nil, "", nil, ErrListNotFound
	}
	var list *models.Wishlist
	var username string
	var wishes []models.Wish
	err := s.tx.WithTransaction(ctx, func(ctx context.Context) error {
		var err error
		list, username, err = s.ResolveGuest(ctx, id, token)
		if err != nil {
			return err
		}
		if ListExpired(list, time.Now()) {
			wishes = []models.Wish{}
			return nil
		}
		wishes, err = s.repo.GuestWishes(ctx, id, token)
		return err
	})
	return list, username, wishes, err
}

func (s *Lists) GuestImage(ctx context.Context, id, token, wishID string) (*models.WishImage, error) {
	if !validGuestList(id, token) || !wishIDPattern.MatchString(wishID) {
		return nil, ErrListNotFound
	}
	var image *models.WishImage
	err := s.tx.WithTransaction(ctx, func(ctx context.Context) error {
		list, _, err := s.ResolveGuest(ctx, id, token)
		if err != nil {
			return err
		}
		if ListExpired(list, time.Now()) {
			return ErrListNotFound
		}
		image, err = s.repo.GuestImage(ctx, id, token, wishID)
		return err
	})
	if err == nil && image == nil {
		err = ErrListNotFound
	}
	return image, err
}

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
		if ListExpired(list, time.Now()) {
			wishes = []models.Wish{}
			return nil
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
	var image *models.WishImage
	err := s.tx.WithTransaction(ctx, func(ctx context.Context) error {
		list, _, err := s.ResolveShare(ctx, token)
		if err != nil {
			return err
		}
		if ListExpired(list, time.Now()) {
			return ErrListNotFound
		}
		image, err = s.repo.SharedImage(ctx, token, wishID)
		return err
	})
	if err == nil && image == nil {
		err = ErrListNotFound
	}
	return image, err
}

type PublicPerson struct {
	Username    string `json:"username"`
	DisplayName string `json:"display_name,omitempty"`
	HasAvatar   bool   `json:"has_avatar,omitempty"`
}

func validProfileText(value string) bool {
	if !utf8.ValidString(value) || utf8.RuneCountInString(value) > 100 {
		return false
	}
	for _, r := range value {
		if unicode.IsControl(r) {
			return false
		}
	}
	return true
}

func (s *Lists) SearchPeople(ctx context.Context, owner, query, cursor string) ([]PublicPerson, string, error) {
	if owner == "" {
		return nil, "", ErrSession
	}
	query = strings.TrimSpace(query)
	if query == "" || !validProfileText(query) || (cursor != "" && !usernamePattern.MatchString(cursor)) {
		return nil, "", ErrInvalidInput
	}
	people, err := s.repo.SearchPeople(ctx, query, cursor, 21)
	if err != nil {
		return nil, "", err
	}
	next := ""
	if len(people) > 20 {
		people = people[:20]
		next = people[len(people)-1].Username
	}
	return people, next, nil
}

func (s *Lists) ProfileName(ctx context.Context, owner string) (string, error) {
	if owner == "" {
		return "", ErrSession
	}
	return s.repo.ProfileName(ctx, owner)
}
func (s *Lists) SetProfileName(ctx context.Context, owner, name string) error {
	if owner == "" {
		return ErrSession
	}
	name = strings.TrimSpace(name)
	if name == "" || !validProfileText(name) {
		return ErrInvalidInput
	}
	return s.repo.SetProfileName(ctx, owner, name)
}

func (s *Lists) PublicProfile(ctx context.Context, username string) (string, string, bool, []models.Wishlist, error) {
	if !regexp.MustCompile(`^[A-Za-z0-9_]{3,30}$`).MatchString(username) {
		return "", "", false, nil, ErrListNotFound
	}
	name, displayName, enabled, lists, err := s.repo.PublicProfile(ctx, strings.ToLower(username))
	if err == nil && name == "" {
		err = ErrListNotFound
	}
	return name, displayName, enabled, lists, err
}
func ListErrorCode(err error) string {
	var listError ListError
	if errors.As(err, &listError) {
		return string(listError)
	}
	return WishErrorCode(err)
}

func (s *Lists) CollectionSetting(ctx context.Context, owner string) (bool, error) {
	return s.repo.CollectionSetting(ctx, owner)
}
func (s *Lists) SetCollectionSetting(ctx context.Context, owner string, enabled bool) error {
	return s.repo.SetCollectionSetting(ctx, owner, enabled)
}
func (s *Lists) PublicCollection(ctx context.Context, username string) ([]models.Wish, error) {
	if !regexp.MustCompile("^[A-Za-z0-9_]{3,30}$").MatchString(username) {
		return nil, ErrListNotFound
	}
	var wishes []models.Wish
	err := s.tx.WithTransaction(ctx, func(ctx context.Context) error {
		found, result, err := s.repo.PublicCollection(ctx, strings.ToLower(username))
		if err != nil {
			return err
		}
		if !found {
			return ErrListNotFound
		}
		wishes = result
		return nil
	})
	return wishes, err
}
func (s *Lists) PublicCollectionImage(ctx context.Context, username, wishID string) (*models.WishImage, error) {
	if !regexp.MustCompile("^[A-Za-z0-9_]{3,30}$").MatchString(username) || !wishIDPattern.MatchString(wishID) {
		return nil, ErrListNotFound
	}
	image, err := s.repo.PublicCollectionImage(ctx, strings.ToLower(username), wishID)
	if err == nil && image == nil {
		err = ErrListNotFound
	}
	return image, err
}
