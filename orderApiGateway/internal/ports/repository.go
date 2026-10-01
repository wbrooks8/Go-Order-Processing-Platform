// Defines the database operations the service can call through a Go interface.
// The PostgreSQL repository and the fake repositories used in tests provide
// these methods; this file does not run database queries itself.

package ports

import (
	"context"

	"github.com/wbrooks8/go_order_api_gateway/internal/domain"
)

type OrderRepository interface {
	CreateOrder(ctx context.Context, order *domain.Order) error
	GetOrderByID(ctx context.Context, id string) (*domain.Order, error)
	UpdateOrder(ctx context.Context, order *domain.Order) error
}
