// Package domain holds the plain data types shared by every feature. No behaviour to implement here.
package domain

import (
	"errors"
	"time"
)

// ErrNotFound is returned (possibly wrapped) when something asked for does not exist.
// Check for it with errors.Is(err, domain.ErrNotFound).
var ErrNotFound = errors.New("not found")

type Product struct {
	ID         int64
	SKU        string
	Name       string
	PriceCents int64
	Stock      int
	Category   string
}

type CustomerTier string

const (
	TierStandard CustomerTier = "Standard"
	TierSilver   CustomerTier = "Silver"
	TierGold     CustomerTier = "Gold"
)

type Customer struct {
	ID               int64
	Name             string
	Email            string
	Tier             CustomerTier
	Active           bool
	CreditLimitCents int64
}

type OrderStatus string

const (
	StatusPending   OrderStatus = "Pending"
	StatusConfirmed OrderStatus = "Confirmed"
	StatusShipped   OrderStatus = "Shipped"
	StatusCancelled OrderStatus = "Cancelled"
)

type OrderLine struct {
	ProductID      int64
	Quantity       int
	UnitPriceCents int64
}

func (l OrderLine) TotalCents() int64 { return int64(l.Quantity) * l.UnitPriceCents }

// Order is a persisted order.
type Order struct {
	ID         int64
	CustomerID int64
	Lines      []OrderLine
	TotalCents int64
	Status     OrderStatus
	CreatedAt  time.Time
}

// RequestedLine / OrderRequest are what a customer asks for, before validation and pricing.
type RequestedLine struct {
	ProductID int64
	Quantity  int
}

type OrderRequest struct {
	CustomerID int64
	Lines      []RequestedLine
}
