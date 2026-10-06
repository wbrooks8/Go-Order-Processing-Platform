// Coordinates order operations for the HTTP handlers. For cancellation, it
// loads the order, asks the order to cancel itself, and saves the change.
// It returns an error immediately if any step fails.

package service

import (
	"context"

	"github.com/wbrooks8/go_order_api_gateway/internal/domain"
	"github.com/wbrooks8/go_order_api_gateway/internal/ports"
)

type OrderEventPublisher interface {
	PublishOrderCreated(ctx context.Context, orderID, customerID string) error
}

type noopOrderEventPublisher struct{}

func (noopOrderEventPublisher) PublishOrderCreated(context.Context, string, string) error {
	return nil
}

type OrderUseCase struct {
	orders    ports.OrderRepository
	publisher OrderEventPublisher
}

func NewOrderUseCase(orders ports.OrderRepository, publishers ...OrderEventPublisher) *OrderUseCase {
	var publisher OrderEventPublisher = noopOrderEventPublisher{}
	if len(publishers) > 0 && publishers[0] != nil {
		publisher = publishers[0]
	}
	return &OrderUseCase{
		orders:    orders,
		publisher: publisher,
	}
}

func (u *OrderUseCase) CreateOrder(ctx context.Context, order *domain.Order) (*domain.Order, error) {
	if err := order.Validate(); err != nil {
		return nil, err
	}

	if err := u.orders.CreateOrder(ctx, order); err != nil {
		return nil, err
	}

	return order, nil
}

func (u *OrderUseCase) GetOrderByID(ctx context.Context, id string) (*domain.Order, error) {
	err := domain.ValidateUUID(id)

	if err != nil {
		return nil, err
	}

	order, err := u.orders.GetOrderByID(ctx, id)

	if err != nil {
		return nil, err
	}

	return order, nil
}

func (u *OrderUseCase) CancelOrder(ctx context.Context, id string) (*domain.Order, error) {
	err := domain.ValidateUUID(id)

	if err != nil {
		return nil, err
	}

	order, err := u.orders.GetOrderByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if err := order.Cancel(); err != nil {
		return nil, err
	}
	if err := u.orders.UpdateOrder(ctx, order); err != nil {
		return nil, err
	}
	return order, nil
}
