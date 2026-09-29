package service

import (
	"context"
	"errors"
	"testing"

	"github.com/wbrooks8/go_order_api_gateway/internal/domain"
)

type fakeOrderRepository struct {
	createdOrder *domain.Order
	createErr    error
	getID        string
	getOrder     *domain.Order
	getErr       error
	cancelID     string
	cancelOrder  *domain.Order
	cancelErr    error
}

func (fake *fakeOrderRepository) CreateOrder(_ context.Context, order *domain.Order) error {
	fake.createdOrder = order
	return fake.createErr
}

func (fake *fakeOrderRepository) GetOrderByID(_ context.Context, id string) (*domain.Order, error) {
	fake.getID = id
	return fake.getOrder, fake.getErr
}

func (fake *fakeOrderRepository) CancelOrder(_ context.Context, id string) (*domain.Order, error) {
	fake.cancelID = id
	return fake.cancelOrder, fake.cancelErr
}

func TestCreateOrderDelegatesAndReturnsOrder(t *testing.T) {
	wantOrder := &domain.Order{CustomerID: "customer-1"}
	repository := &fakeOrderRepository{}
	useCase := NewOrderUseCase(repository)

	gotOrder, err := useCase.CreateOrder(context.Background(), wantOrder)
	if err != nil {
		t.Fatalf("CreateOrder returned error: %v", err)
	}
	if repository.createdOrder != wantOrder {
		t.Fatal("repository did not receive the supplied order")
	}
	if gotOrder != wantOrder {
		t.Fatal("CreateOrder did not return the supplied order")
	}
}

func TestCreateOrderPropagatesRepositoryError(t *testing.T) {
	wantErr := errors.New("database unavailable")
	useCase := NewOrderUseCase(&fakeOrderRepository{createErr: wantErr})

	_, err := useCase.CreateOrder(context.Background(), &domain.Order{})
	if !errors.Is(err, wantErr) {
		t.Fatalf("CreateOrder error = %v, want %v", err, wantErr)
	}
}

func TestGetOrderByIDDelegatesAndPropagatesResult(t *testing.T) {
	wantOrder := &domain.Order{ID: "order-1"}
	repository := &fakeOrderRepository{getOrder: wantOrder}
	useCase := NewOrderUseCase(repository)

	gotOrder, err := useCase.GetOrderByID(context.Background(), "order-1")
	if err != nil {
		t.Fatalf("GetOrderByID returned error: %v", err)
	}
	if repository.getID != "order-1" || gotOrder != wantOrder {
		t.Fatalf("GetOrderByID got (%q, %p), want (order-1, %p)", repository.getID, gotOrder, wantOrder)
	}

	wantErr := domain.ErrOrderNotFound
	repository.getErr = wantErr
	_, err = useCase.GetOrderByID(context.Background(), "missing")
	if !errors.Is(err, wantErr) {
		t.Fatalf("GetOrderByID error = %v, want %v", err, wantErr)
	}
}

func TestCancelOrderDelegatesAndPropagatesResult(t *testing.T) {
	wantOrder := &domain.Order{ID: "order-1", Status: "CANCELLED"}
	repository := &fakeOrderRepository{cancelOrder: wantOrder}
	useCase := NewOrderUseCase(repository)

	gotOrder, err := useCase.CancelOrder(context.Background(), "order-1")
	if err != nil {
		t.Fatalf("CancelOrder returned error: %v", err)
	}
	if repository.cancelID != "order-1" || gotOrder != wantOrder {
		t.Fatalf("CancelOrder got (%q, %p), want (order-1, %p)", repository.cancelID, gotOrder, wantOrder)
	}

	wantErr := domain.ErrOrderCannotBeCancelled
	repository.cancelErr = wantErr
	_, err = useCase.CancelOrder(context.Background(), "order-1")
	if !errors.Is(err, wantErr) {
		t.Fatalf("CancelOrder error = %v, want %v", err, wantErr)
	}
}
