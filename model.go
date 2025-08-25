package main

import "time"

type OrderStatus string

const (
	StatusPending   OrderStatus = "PENDING"
	StatusConfirmed OrderStatus = "CONFIRMED"
	StatusShipped   OrderStatus = "SHIPPED"
	StatusDelivered OrderStatus = "DELIVERED"
	StatusCanceled  OrderStatus = "CANCELED"
)

type OrderLine struct {
	ProductID      string `json:"product_id"`
	Quantity       int    `json:"quantity"`
	UnitPriceCents int64  `json:"unit_price_cents"`
	LineTotalCents int64  `json:"line_total_cents"`
}

// Order is the main data structure for application
type Order struct {
	ID         string            `json:"id"`
	CustomerID string            `json:"customer_id"`
	Currency   string            `json:"currency"`
	Lines      []OrderLine       `json:"lines"`
	Attributes map[string]string `json:"attributes,omitempty"`
	TotalCents int64             `json:"total_cents"`
	Status     OrderStatus       `json:"status"`
	Version    int64             `json:"version"`
	CreatedAt  time.Time         `json:"created_at"`
	UpdatedAt  time.Time         `json:"updated_at"`
	DeletedAt  *time.Time        `json:"deleted_at,omitempty"`
}

// List holds all filtering paginated list of orders
type ListOptions struct {
	Status     *OrderStatus
	CustomerID *string
	CreateFrom *time.Time
	CreateTo   *time.Time
	Page       int
	PageSize   int
}

type ListResult[T any] struct {
	Items      []T `json:"items"`
	Page       int `json:"page"`
	PageSize   int `json:"page_size"`
	TotalItems int `json:"total_items"`
	TotalPages int `json:"total_pages"`
}
