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
	getCalls      int
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
	fake.getCalls++
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
	wantOrder := &domain.Order{ID: "44f551b5-0c28-4132-bcd4-d09b048dfe61"}
	repository := &fakeOrderRepository{getOrder: wantOrder}
	useCase := NewOrderUseCase(repository)

	gotOrder, err := useCase.GetOrderByID(context.Background(), "44f551b5-0c28-4132-bcd4-d09b048dfe61")
	if err != nil {
		t.Fatalf("GetOrderByID returned error: %v", err)
	}
	if repository.getID != "44f551b5-0c28-4132-bcd4-d09b048dfe61" || gotOrder != wantOrder {
		t.Fatalf("GetOrderByID got (%q, %p), want (44f551b5-0c28-4132-bcd4-d09b048dfe61, %p)", repository.getID, gotOrder, wantOrder)
	}

	wantErr := domain.ErrOrderNotFound
	repository.getErr = wantErr
	_, err = useCase.GetOrderByID(context.Background(), "44f551b5-0c28-4132-bcd4-d09b048dfe62")
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
			order := &domain.Order{ID: "44f551b5-0c28-4132-bcd4-d09b048dfe61", Status: test.status}
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
	order, err := domain.NewOrder("d8f3b2a1-0000-4a8a-8e2b-123456789abc", []domain.OrderItem{{ProductID: "a1b2c3d4-e5f6-4a8b-9c0d-112233445566", Quantity: 1, UnitPrice: 100000}}, domain.Address{}, domain.Address{})
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

// Rejected IDs must not cause a database read or update.
func TestOrderOperationsRejectInvalidUUID(t *testing.T) {
	for _, operation := range []string{"get", "cancel"} {
		for _, id := range []string{"", "banana", "44f551b5-0c28-4132-bcd4-d09b048dfe6z"} {
			t.Run(operation+"/"+id, func(t *testing.T) {
				repository := &fakeOrderRepository{getOrder: &domain.Order{Status: "Accepted"}}
				useCase := NewOrderUseCase(repository)
				call := useCase.GetOrderByID
				if operation == "cancel" {
					call = useCase.CancelOrder
				}
				order, err := call(context.Background(), id)
				if !errors.Is(err, domain.ErrInvalidUUID) {
					t.Errorf("error = %v, want ErrInvalidUUID", err)
				}
				if order != nil {
					t.Error("expected nil order")
				}
				if repository.getCalls != 0 || repository.updatedOrder != nil {
					t.Error("invalid ID reached repository")
				}
			})
		}
	}
}

func TestCreateOrderRejectsInvalidDataFromNonHTTPCallers(t *testing.T) {
	for _, test := range []struct {
		name   string
		change func(*domain.Order)
		want   error
	}{
		{"customer UUID", func(o *domain.Order) { o.CustomerID = "banana" }, domain.ErrInvalidUUID},
		{"product UUID", func(o *domain.Order) { o.Items[0].ProductID = "banana" }, domain.ErrInvalidUUID},
		{"total", func(o *domain.Order) { o.TotalAmount++ }, domain.ErrInvalidOrder},
	} {
		t.Run(test.name, func(t *testing.T) {
			order := validServiceOrder(t)
			test.change(order)
			repository := &fakeOrderRepository{}
			_, err := NewOrderUseCase(repository).CreateOrder(context.Background(), order)
			if !errors.Is(err, test.want) {
				t.Fatalf("error = %v, want %v", err, test.want)
			}
			if repository.createdOrder != nil {
				t.Fatal("invalid order reached storage")
			}
		})
	}
}
