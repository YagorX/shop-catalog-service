package domain

import (
	"errors"
	"fmt"
	"strings"
)

var (
	ErrProductNotFound   = errors.New("product not found")
	ErrInvalidPagination = errors.New("invalid pagination")
	ErrProductAlreadyExists = errors.New("product already exists")
    ErrInvalidProduct       = errors.New("invalid product data")
    ErrInsufficientStock    = errors.New("insufficient stock")
)

type Product struct {
	ID          string
	SKU         string
	Name        string
	Description string
	PriceCents  int64
	Currency    string
	Stock       int32
	Active      bool
}

type CreateProductCommand struct {
    SKU         string
    Name        string
    Description string
    PriceCents  int64
    Currency    string
    Stock       int32
}

type UpdateProductCommand struct {
    ID          string
    Name        *string
    Description *string
    PriceCents  *int64
    Active      *bool
}

func (p Product) Validate() error {
	if p.ID == "" {
		return errors.New("product id is required")
	}
	if p.SKU == "" {
		return errors.New("product sku is required")
	}
	if p.Name == "" {
		return errors.New("product name is required")
	}
	if p.PriceCents < 0 {
		return errors.New("product price must be >= 0")
	}
	if p.Currency == "" {
		return errors.New("product currency is required")
	}
	if p.Stock < 0 {
		return errors.New("product stock must be >= 0")
	}
	return nil
}

func (c CreateProductCommand) Validate() error {
    if strings.TrimSpace(c.SKU) == "" {
        return fmt.Errorf("%w: sku is required", ErrInvalidProduct)
    }
    if strings.TrimSpace(c.Name) == "" {
        return fmt.Errorf("%w: name is required", ErrInvalidProduct)
    }
    if c.PriceCents < 0 {
        return fmt.Errorf("%w: price must be >= 0", ErrInvalidProduct)
    }
    if strings.TrimSpace(c.Currency) == "" {
        return fmt.Errorf("%w: currency is required", ErrInvalidProduct)
    }
    if c.Stock < 0 {
        return fmt.Errorf("%w: stock must be >= 0", ErrInvalidProduct)
    }
    return nil
}

func (c UpdateProductCommand) Validate() error {
    if strings.TrimSpace(c.ID) == "" {
        return fmt.Errorf("%w: id is required", ErrInvalidProduct)
    }
    if c.Name != nil && strings.TrimSpace(*c.Name) == "" {
        return fmt.Errorf("%w: name cannot be empty", ErrInvalidProduct)
    }
    if c.PriceCents != nil && *c.PriceCents < 0 {
        return fmt.Errorf("%w: price must be >= 0", ErrInvalidProduct)
    }
    return nil
}