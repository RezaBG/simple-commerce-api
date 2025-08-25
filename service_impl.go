package main

import (
	"context"
	"encoding/hex"
	"errors"
	"math/rand"
	"sync"
	"time"
)

var (
	ErrNotFound     = errors.New("not found")
	ErrConflict     = errors.New("conflict")
	ErrInvalidInput = errors.New("invalid input")
	ErrInvalidState = errors.New("invalid state")
	ErrIdempotency  = errors.New("idempotency error")
)

type InMemoryService struct {
	mu     sync.RWMutex
	orders map[string]Order
	idempo map[string]string
}

func NewInMemoryService() *InMemoryService {
	return &InMemoryService{
		orders: make(map[string]Order),
		idempo: make(map[string]string),
	}
}

func (s *InMemoryService) Create(ctx context.Context, o Order, idempotencyKey string) (Order, bool, error) {
	// TODO: implement this logic
	return Order{}, false, nil
}

func (s *InMemoryService) Get(ctx context.Context, id string, includeDeleted bool) (Order, error) {
	return Order{}, nil
}

func (s *InMemoryService) List(ctx context.Context, opts ListOptions) (ListResult[Order], error) {
	// TODO: implement this logic
	return ListResult[Order]{}, nil
}

func (s *InMemoryService) UpdatedStatus(ctx context.Context, id string, newStatus OrderStatus, expectedVersion *int64) (Order, error) {
	return Order{}, nil
}

// --- Helper functions ---

func NewID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return time.Now().UTC().Format("20060102150405.000000")
	}
	return hex.EncodeToString(b)
}

func cloneOrder(o Order) Order {
	lines := make([]OrderLine, len(o.Lines))
	copy(lines, o.Lines)
	attrs := cloneMap(o.Attributes)
	var del *time.Time
	if o.DeletedAt != nil {
		t := *o.DeletedAt
		del = &t
	}
	return Order{
		ID:         o.ID,
		CustomerID: o.CustomerID,
		Currency:   o.Currency,
		Lines:      lines,
		Attributes: attrs,
		TotalCents: o.TotalCents,
		Status:     o.Status,
		Version:    o.Version,
		CreatedAt:  o.CreatedAt,
		UpdatedAt:  o.UpdatedAt,
		DeletedAt:  del,
	}
}

func cloneMap(m map[string]string) map[string]string {
	if m == nil {
		return nil
	}
	cp := make(map[string]string, len(m))
	for k, v := range m {
		cp[k] = v
	}
	return cp
}

func ctxErr(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
		return nil
	}
}

func isTransitionAllowed(from, to OrderStatus) bool {
	switch from {
	case StatusPending:
		return to == StatusConfirmed || to == StatusCanceled
	case StatusConfirmed:
		return to == StatusShipped || to == StatusCanceled
	case StatusShipped:
		return to == StatusDelivered
	}
	return false
}

func isValidStatus(s OrderStatus) bool {
	switch s {
	case StatusPending, StatusConfirmed, StatusShipped, StatusDelivered, StatusCanceled:
		return true
	default:
		return false
	}
}
