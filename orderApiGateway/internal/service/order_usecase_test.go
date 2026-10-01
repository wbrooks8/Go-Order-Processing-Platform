// Tests how the service coordinates repository calls and handles errors.
// A fake repository records calls and supplies results without using PostgreSQL.

package service

import (
	"context"
	"errors"
	"testing"

	"github.com/wbrooks8/go_order_api_gateway/internal/domain"
)

type fakeOrderRepository struct {
	createdOrder  *domain.Order
	createErr     error
	getID         string
	getOrder      *domain.Order
	getErr        error
	updatedOrder  *domain.Order
	updateErr     error
	updatedStatus string
}

func (fake *fakeOrderRepository) CreateOrder(_ context.Context, order *domain.Order) error {
	fake.createdOrder = order
	return fake.createErr
}

func (fake *fakeOrderRepository) GetOrderByID(_ context.Context, id string) (*domain.Order, error) {
	fake.getID = id
	return fake.getOrder, fake.getErr
}

func (fake *fakeOrderRepository) UpdateOrder(_ context.Context, order *domain.Order) error {
	fake.updatedOrder = order
	fake.updatedStatus = order.Status
	return fake.updateErr
}

func TestCreateOrderDelegatesAndReturnsOrder(t *testing.T) {
	wantOrder := validServiceOrder(t)
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

	_, err := useCase.CreateOrder(context.Background(), validServiceOrder(t))
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

func TestCancelOrder(t *testing.T) {
	loadErr := errors.New("load failed")
	saveErr := errors.New("save failed")
	tests := []struct {
		name       string
		status     string
		getErr     error
		updateErr  error
		wantErr    error
		wantUpdate bool
	}{
		{name: "accepted", status: "Accepted", wantUpdate: true},
		{name: "already cancelled", status: "Cancelled", wantUpdate: true},
		{name: "paid", status: "Paid", wantErr: domain.ErrOrderCannotBeCancelled},
		{name: "missing", getErr: domain.ErrOrderNotFound, wantErr: domain.ErrOrderNotFound},
		{name: "loading fails", getErr: loadErr, wantErr: loadErr},
		{name: "saving fails", status: "Accepted", updateErr: saveErr, wantErr: saveErr, wantUpdate: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			order := &domain.Order{ID: "order-1", Status: test.status}
			repository := &fakeOrderRepository{getOrder: order, getErr: test.getErr, updateErr: test.updateErr}
			if test.getErr != nil {
				repository.getOrder = nil
			}
			got, err := NewOrderUseCase(repository).CancelOrder(context.Background(), order.ID)
			if !errors.Is(err, test.wantErr) {
				t.Fatalf("error = %v, want %v", err, test.wantErr)
			}
			if repository.getID != order.ID {
				t.Errorf("loaded ID = %q, want %q", repository.getID, order.ID)
			}
			if (repository.updatedOrder != nil) != test.wantUpdate {
				t.Fatalf("update called = %v, want %v", repository.updatedOrder != nil, test.wantUpdate)
			}
			if test.wantUpdate && (repository.updatedOrder != order || repository.updatedStatus != "Cancelled") {
				t.Error("update must receive the cancelled order")
			}
			if !test.wantUpdate && order.Status != test.status {
				t.Error("order changed despite failed load or rejected cancellation")
			}
			if test.wantErr != nil {
				if got != nil {
					t.Error("expected no returned order on failure")
				}
			} else if got != order || got.Status != "Cancelled" {
				t.Error("expected the cancelled order")
			}
		})
	}
}

func validServiceOrder(t *testing.T) *domain.Order {
	t.Helper()
	order, err := domain.NewOrder("customer-1", []domain.OrderItem{{ProductID: "product-1", Quantity: 1, UnitPrice: 10}}, domain.Address{}, domain.Address{})
	if err != nil {
		t.Fatalf("create test order: %v", err)
	}
	return order
}
func TestCreateOrderRejectsInvalidOrderBeforeSaving(t *testing.T) {
	repository := &fakeOrderRepository{}
	order, err := NewOrderUseCase(repository).CreateOrder(context.Background(), &domain.Order{})
	if !errors.Is(err, domain.ErrInvalidOrder) {
		t.Errorf("error = %v, want ErrInvalidOrder", err)
	}
	if order != nil {
		t.Error("expected nil order")
	}
	if repository.createdOrder != nil {
		t.Error("invalid order reached repository")
	}
}
