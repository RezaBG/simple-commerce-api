package main

import "context"

type OrderService interface {
	Create(ctx context.Context, o Order, idempotencyKey string) (created Order, reused bool, err error)

	Get(ctx context.Context, id string, includeDeleted bool) (Order, error)

	List(ctx context.Context, opts ListOptions) (ListResult[Order], error)

	UpdatedStatus(ctx context.Context, id string, newStatus OrderStatus, expectedVersion *int64) (Order, error)

	Delete(ctx context.Context, id string) error
}
