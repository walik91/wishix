package services

import (
	"context"
	"wishix/server/internal/productimport"
)

type ProductImport struct {
	importer *productimport.Importer
	auth     *Auth
}

func NewProductImport(importer *productimport.Importer, auth *Auth) *ProductImport {
	return &ProductImport{importer: importer, auth: auth}
}

func (s *ProductImport) Preview(ctx context.Context, ownerID, raw, locale string) (productimport.Preview, error) {
	if _, err := productimport.ValidateURL(raw); err != nil {
		return productimport.Preview{}, err
	}
	if err := s.auth.LimitProductImport(ctx, ownerID); err != nil {
		return productimport.Preview{}, err
	}
	return s.importer.Preview(ctx, raw, locale)
}
