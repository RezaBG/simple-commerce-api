package main

import (
	"context"
	"encoding/hex"
	"errors"
	"math/rand"
	"sort"
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
	// always check context first
	if err := ctx.Err(); err != nil {
		return Order{}, false, err
	}

	// 1. Idempotency "Fast-Path" (read-only lock)
	if idempotencyKey != "" {
		s.mu.RLock()
		if orderID, ok := s.idempo[idempotencyKey]; ok {
			existing, ok := s.orders[orderID]
			s.mu.RUnlock() // Release read lock

			if !ok {
				// This indicates a data consistency issue.
				return Order{}, false, ErrIdempotency
			}
			return cloneOrder(existing), true, nil
		}
		s.mu.RUnlock()
	}

	// 2. Validate input
	if o.CustomerID == "" || o.Currency == "" || len(o.Lines) == 0 {
		return Order{}, false, ErrInvalidInput
	}

	// 3. Build, Enrich, and Calculate
	now := time.Now().UTC()
	newOrder := Order{
		ID:         newID(),
		CustomerID: o.CustomerID,
		Currency:   o.Currency,
		Lines:      make([]OrderLine, len(o.Lines)),
		Attributes: cloneMap(o.Attributes),
		Status:     StatusPending,
		Version:    1,
		CreatedAt:  now,
		UpdatedAt:  now,
	}

	var totalCents int64
	for i, line := range o.Lines {
		// Also validate the line times themselves
		if line.Quantity <= 0 || line.UnitPriceCents < 0 {
			return Order{}, false, ErrInvalidInput
		}
		line.LineTotalCents = int64(line.Quantity) * line.UnitPriceCents
		newOrder.Lines[i] = line
		totalCents += line.LineTotalCents

	}
	newOrder.TotalCents = totalCents

	// 4. Critical section: Lock, double-check idempotency, and save.
	s.mu.Lock()
	defer s.mu.Unlock()

	// Double-check idempotency inside the lock to prevent a race condition
	if idempotencyKey != "" {
		if orderID, ok := s.idempo[idempotencyKey]; ok {
			if existing, ok := s.orders[orderID]; ok {
				return cloneOrder(existing), true, nil
			}

			return Order{}, false, ErrIdempotency
		}
	}

	// Save the new order
	s.orders[newOrder.ID] = newOrder
	if idempotencyKey != "" {
		s.idempo[idempotencyKey] = newOrder.ID
	}

	return cloneOrder(newOrder), false, nil

}

func (s *InMemoryService) Get(ctx context.Context, id string, includeDeleted bool) (Order, error) {
	if err := ctxErr(ctx); err != nil {
		return Order{}, err
	}

	// Use a read lock for this read-only operation
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Look for the order in the map
	order, ok := s.orders[id]
	if !ok {
		return Order{}, ErrNotFound
	}

	if order.DeletedAt != nil && !includeDeleted {
		return Order{}, ErrNotFound
	}

	return cloneOrder(order), nil
}

func (s *InMemoryService) List(ctx context.Context, opts ListOptions) (ListResult[Order], error) {
	if err := ctxErr(ctx); err != nil {
		return ListResult[Order]{}, err
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	// 1. Convert map to slice for filtering and sorting
	allOrders := make([]Order, 0, len(s.orders))
	for _, order := range s.orders {
		allOrders = append(allOrders, order)
	}

	// 2. Filter the slice based on options
	filtered := make([]Order, 0)
	for _, order := range allOrders {
		if order.DeletedAt != nil {
			continue
		}
		if opts.Status != nil && order.Status != *opts.Status {
			continue
		}
		if opts.CustomerID != nil && order.CustomerID != *opts.CustomerID {
			continue
		}
		if opts.CreateFrom != nil && order.CreatedAt.Before(*opts.CreateFrom) {
			continue
		}
		if opts.CreateTo != nil && order.CreatedAt.After(*opts.CreateTo) {
			continue
		}
		filtered = append(filtered, order)
	}

	// 3. Sort by creation date, newest first, for consistent pagination
	sort.Slice(filtered, func(i, j int) bool {
		return filtered[i].CreatedAt.After(filtered[j].CreatedAt)
	})

	// 4. Paginate the results
	totalItems := len(filtered)
	if opts.Page <= 0 {
		opts.Page = 1
	}
	if opts.PageSize <= 0 {
		opts.PageSize = 10
	}

	start := (opts.Page - 1) * opts.PageSize
	end := start + opts.PageSize
	if start >= totalItems {
		return ListResult[Order]{Items: []Order{}, TotalItems: totalItems}, nil
	}
	if end > totalItems {
		end = totalItems
	}
	pageItems := filtered[start:end]

	// 5. Build and return the final paginated result with safe copies
	result := ListResult[Order]{
		Items:      make([]Order, len(pageItems)),
		Page:       opts.Page,
		PageSize:   opts.PageSize,
		TotalItems: totalItems,
		TotalPages: (totalItems + opts.PageSize - 1) / opts.PageSize,
	}
	for i, order := range pageItems {
		result.Items[i] = cloneOrder(order)
	}

	return result, nil
}

func (s *InMemoryService) UpdatedStatus(ctx context.Context, id string, newStatus OrderStatus, expectedVersion *int64) (Order, error) {
	return Order{}, nil
}

// --- Helper functions ---

func newID() string {
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
