// Coordinates order operations for the HTTP handlers. For cancellation, it
// loads the order, asks the order to cancel itself, and saves the change.
// It returns an error immediately if any step fails.

package service

import (
	"context"

	"github.com/wbrooks8/go_order_api_gateway/internal/domain"
	"github.com/wbrooks8/go_order_api_gateway/internal/ports"
)

type OrderUseCase struct {
	orders ports.OrderRepository
}

func NewOrderUseCase(orders ports.OrderRepository) *OrderUseCase {
	return &OrderUseCase{orders: orders}
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
