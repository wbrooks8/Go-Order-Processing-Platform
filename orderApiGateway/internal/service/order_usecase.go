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
	if err := u.orders.CreateOrder(ctx, order); err != nil {
		return nil, err
	}

	return order, nil
}

func (u *OrderUseCase) GetOrderByID(ctx context.Context, id string) (*domain.Order, error) {
	return u.orders.GetOrderByID(ctx, id)
}

func (u *OrderUseCase) CancelOrder(ctx context.Context, id string) (*domain.Order, error) {
	return u.orders.CancelOrder(ctx, id)
}
