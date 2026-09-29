package ports

import (
	"context"

	"github.com/wbrooks8/go_order_api_gateway/internal/domain"
)

type OrderRepository interface {
	CreateOrder(ctx context.Context, order *domain.Order) error
	GetOrderByID(ctx context.Context, id string) (*domain.Order, error)
	CancelOrder(ctx context.Context, id string) (*domain.Order, error)
}
