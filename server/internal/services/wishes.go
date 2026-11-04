package services

import (
	"context"
	"errors"
	"net/url"
	"regexp"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	security "wishix/server/internal/auth"
	"wishix/server/internal/models"
)

type WishError string

func (e WishError) Error() string { return string(e) }

const (
	ErrWishTitle       WishError = "wish_title_invalid"
	ErrWishDescription WishError = "wish_description_invalid"
	ErrWishPrice       WishError = "wish_price_invalid"
	ErrWishPriority    WishError = "wish_priority_invalid"
	ErrWishLinks       WishError = "wish_links_invalid"
	ErrWishNotFound    WishError = "wish_not_found"
)

type WishInput struct {
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Price       *string  `json:"price"`
	Currency    string   `json:"currency"`
	Priority    string   `json:"priority"`
	ShopLinks   []string `json:"shop_links"`
}

type WishRepository interface {
	List(context.Context, string) ([]models.Wish, error)
	Create(context.Context, *models.Wish) error
	Delete(context.Context, string, string) (bool, error)
}

type Wishes struct {
	repo WishRepository
	tx   Transactor
}

func NewWishes(repo WishRepository, tx Transactor) *Wishes {
	return &Wishes{repo: repo, tx: tx}
}

var wishPricePattern = regexp.MustCompile(`^(0|[1-9][0-9]{0,9})(\.[0-9]{1,2})?$`)
var wishCurrencyPattern = regexp.MustCompile(`^[A-Z]{3}$`)
var wishIDPattern = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

func validateWish(input WishInput) (WishInput, error) {
	input.Title = strings.TrimSpace(input.Title)
	input.Description = strings.TrimSpace(input.Description)
	if !utf8.ValidString(input.Title) || utf8.RuneCountInString(input.Title) < 1 || utf8.RuneCountInString(input.Title) > 200 || strings.ContainsRune(input.Title, 0) {
		return input, ErrWishTitle
	}
	if !utf8.ValidString(input.Description) || utf8.RuneCountInString(input.Description) > 5000 || strings.ContainsRune(input.Description, 0) {
		return input, ErrWishDescription
	}
	input.Currency = strings.ToUpper(strings.TrimSpace(input.Currency))
	if input.Price == nil {
		// Currency has no meaning without the optional price.
		input.Currency = ""
	} else {
		price := strings.TrimSpace(*input.Price)
		if !wishPricePattern.MatchString(price) || !wishCurrencyPattern.MatchString(input.Currency) {
			return input, ErrWishPrice
		}
		parts := strings.SplitN(price, ".", 2)
		fraction := "00"
		if len(parts) == 2 {
			fraction = (parts[1] + "0")[:2]
		}
		price = parts[0] + "." + fraction
		input.Price = &price
	}
	if input.Priority == "" {
		input.Priority = "medium"
	}
	if input.Priority != "low" && input.Priority != "medium" && input.Priority != "high" {
		return input, ErrWishPriority
	}
	if len(input.ShopLinks) > 10 {
		return input, ErrWishLinks
	}
	links := make([]string, 0, len(input.ShopLinks))
	for _, raw := range input.ShopLinks {
		raw = strings.TrimSpace(raw)
		link, err := url.Parse(raw)
		if err != nil || len(raw) > 2048 || !utf8.ValidString(raw) || strings.IndexFunc(raw, func(r rune) bool { return unicode.IsSpace(r) || unicode.IsControl(r) }) >= 0 {
			return input, ErrWishLinks
		}
		if (link.Scheme != "https" && link.Scheme != "http") || link.Hostname() == "" || link.User != nil {
			return input, ErrWishLinks
		}
		links = append(links, raw)
	}
	input.ShopLinks = links
	return input, nil
}

func (s *Wishes) List(ctx context.Context, ownerID string) ([]models.Wish, error) {
	if ownerID == "" {
		return nil, ErrSession
	}
	return s.repo.List(ctx, ownerID)
}

func (s *Wishes) Create(ctx context.Context, ownerID string, input WishInput) (*models.Wish, error) {
	if ownerID == "" {
		return nil, ErrSession
	}
	input, err := validateWish(input)
	if err != nil {
		return nil, err
	}
	id, createdAt := security.UUID(), time.Now().UTC()
	var result *models.Wish
	err = s.tx.WithTransaction(ctx, func(txCtx context.Context) error {
		// Fresh model and associations on every CockroachDB retry.
		wish := &models.Wish{ID: id, OwnerID: ownerID, Title: input.Title, Description: input.Description, Price: input.Price, Currency: input.Currency, Priority: input.Priority, CreatedAt: createdAt}
		for position, link := range input.ShopLinks {
			wish.ShopLinks = append(wish.ShopLinks, models.WishShopLink{WishID: id, Position: position, URL: link})
		}
		if err := s.repo.Create(txCtx, wish); err != nil {
			return err
		}
		result = wish
		return nil
	})
	return result, err
}

func (s *Wishes) Delete(ctx context.Context, ownerID, id string) error {
	if ownerID == "" {
		return ErrSession
	}
	if !wishIDPattern.MatchString(id) {
		return ErrWishNotFound
	}
	return s.tx.WithTransaction(ctx, func(txCtx context.Context) error {
		deleted, err := s.repo.Delete(txCtx, ownerID, id)
		if err != nil {
			return err
		}
		if !deleted {
			// The same response for another owner's wish and an absent wish.
			return ErrWishNotFound
		}
		return nil
	})
}

func WishErrorCode(err error) string {
	var wishError WishError
	if errors.As(err, &wishError) {
		return string(wishError)
	}
	return ErrorCode(err)
}
