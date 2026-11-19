package productimport

import (
	"context"
	"encoding/base64"
	"time"

	"wishix/server/internal/wishimage"
)

type Image struct {
	MIME string `json:"mime"`
	Data string `json:"data_base64"`
}

type Preview struct {
	Status          string `json:"status"`
	Title           string `json:"title,omitempty"`
	Description     string `json:"description,omitempty"`
	Price           string `json:"price,omitempty"`
	Currency        string `json:"currency,omitempty"`
	ShopName        string `json:"shop_name,omitempty"`
	Image           *Image `json:"image,omitempty"`
	ImageReason     string `json:"-"`
	ImageHTTPStatus int    `json:"-"`
}

type Importer struct {
	fetch    *fetcher
	images   *wishimage.Processor
	adapters []Adapter
	slots    chan struct{}
}

// New uses the common extractor. Register narrowly matched adapters only when needed.
func New(adapters ...Adapter) *Importer {
	return &Importer{fetch: newFetcher(), images: wishimage.New(), adapters: append([]Adapter(nil), adapters...), slots: make(chan struct{}, 2)}
}

func (i *Importer) Preview(ctx context.Context, raw, locale string) (Preview, error) {
	if _, err := ValidateURL(raw); err != nil {
		return Preview{}, err
	}
	select {
	case i.slots <- struct{}{}:
		defer func() { <-i.slots }()
	default:
		return Preview{}, ErrBusy
	}
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	if locale != "ru" && locale != "ro" {
		locale = "en"
	}
	body, page, err := i.fetch.get(ctx, raw, locale, MaxHTML, false)
	if err != nil {
		return Preview{}, err
	}
	metadata, err := extract(body, page, i.adapters)
	if err != nil {
		return Preview{}, err
	}
	if metadata.NotProduct {
		return Preview{Status: "not_product"}, nil
	}
	result := Preview{Status: "partial", Title: metadata.Title, Description: metadata.Description, Price: metadata.Price, Currency: metadata.Currency, ShopName: metadata.ShopName, ImageReason: "no_candidate"}
	for index, candidate := range metadata.Images {
		if index >= 3 || ctx.Err() != nil {
			break
		}
		rawImage, _, err := i.fetch.get(ctx, candidate, locale, wishimage.MaxUploadBytes, true)
		if err != nil {
			result.ImageReason, result.ImageHTTPStatus = FailureDetails(err)
			continue
		}
		data, err := i.images.Process(ctx, rawImage)
		if err != nil {
			result.ImageReason, result.ImageHTTPStatus = "invalid_image", 0
			continue
		}
		result.Image = &Image{MIME: "image/jpeg", Data: base64.StdEncoding.EncodeToString(data)}
		result.ImageReason, result.ImageHTTPStatus = "", 0
		break
	}
	if ctx.Err() != nil && result.Title == "" && result.Description == "" && result.Price == "" {
		return Preview{}, ErrFetch
	}
	if result.Title != "" && result.Price != "" && result.Image != nil {
		result.Status = "ok"
	}
	if result.Title == "" && result.Description == "" && result.Price == "" && result.Image == nil {
		result.Status = "empty"
	}
	return result, nil
}
