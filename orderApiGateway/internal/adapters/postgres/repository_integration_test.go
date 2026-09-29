package postgres

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/wbrooks8/go_order_api_gateway/internal/domain"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func TestRepositoryIntegrationCreateGetAndCancel(t *testing.T) {
	repository, db := newIntegrationRepository(t)
	ctx := context.Background()
	wantOrder := integrationOrder("Accepted")

	if err := repository.CreateOrder(ctx, wantOrder); err != nil {
		t.Fatalf("CreateOrder returned error: %v", err)
	}
	if wantOrder.ID == "" {
		t.Fatal("CreateOrder did not populate the generated UUID")
	}
	t.Cleanup(func() {
		db.Where("id = ?", wantOrder.ID).Delete(&OrderModel{})
	})

	gotOrder, err := repository.GetOrderByID(ctx, wantOrder.ID)
	if err != nil {
		t.Fatalf("GetOrderByID returned error: %v", err)
	}
	if gotOrder.ID != wantOrder.ID || gotOrder.CustomerID != wantOrder.CustomerID {
		t.Errorf("retrieved order identity mismatch: got %+v", gotOrder)
	}
	if gotOrder.ShippingAddress != wantOrder.ShippingAddress || gotOrder.BillingAddress != wantOrder.BillingAddress {
		t.Errorf("retrieved addresses mismatch: got %+v", gotOrder)
	}
	if len(gotOrder.Items) != 1 || gotOrder.Items[0].SKU != "SKU-INTEGRATION" {
		t.Fatalf("retrieved items = %+v, want one SKU-INTEGRATION item", gotOrder.Items)
	}

	cancelledOrder, err := repository.CancelOrder(ctx, wantOrder.ID)
	if err != nil {
		t.Fatalf("CancelOrder returned error: %v", err)
	}
	if cancelledOrder.Status != "CANCELLED" {
		t.Fatalf("cancelled status = %q, want CANCELLED", cancelledOrder.Status)
	}

	retrievedAfterCancel, err := repository.GetOrderByID(ctx, wantOrder.ID)
	if err != nil {
		t.Fatalf("GetOrderByID after cancellation returned error: %v", err)
	}
	if retrievedAfterCancel.Status != "CANCELLED" {
		t.Fatalf("persisted status = %q, want CANCELLED", retrievedAfterCancel.Status)
	}

	repeatedCancel, err := repository.CancelOrder(ctx, wantOrder.ID)
	if err != nil {
		t.Fatalf("repeated CancelOrder returned error: %v", err)
	}
	if repeatedCancel.Status != "CANCELLED" {
		t.Fatalf("repeated cancel status = %q, want CANCELLED", repeatedCancel.Status)
	}

	_, err = repository.GetOrderByID(ctx, "00000000-0000-0000-0000-000000000000")
	if !errors.Is(err, domain.ErrOrderNotFound) {
		t.Fatalf("missing order error = %v, want ErrOrderNotFound", err)
	}
}

func TestRepositoryIntegrationRejectsCancellationForPaidOrder(t *testing.T) {
	repository, db := newIntegrationRepository(t)
	ctx := context.Background()
	order := integrationOrder("PAID")

	if err := repository.CreateOrder(ctx, order); err != nil {
		t.Fatalf("CreateOrder returned error: %v", err)
	}
	t.Cleanup(func() {
		db.Where("id = ?", order.ID).Delete(&OrderModel{})
	})

	_, err := repository.CancelOrder(ctx, order.ID)
	if !errors.Is(err, domain.ErrOrderCannotBeCancelled) {
		t.Fatalf("CancelOrder error = %v, want ErrOrderCannotBeCancelled", err)
	}

	unchanged, err := repository.GetOrderByID(ctx, order.ID)
	if err != nil {
		t.Fatalf("GetOrderByID returned error: %v", err)
	}
	if unchanged.Status != "PAID" {
		t.Fatalf("persisted status = %q, want PAID", unchanged.Status)
	}
}

func newIntegrationRepository(t *testing.T) (*Repository, *gorm.DB) {
	t.Helper()

	dsn := os.Getenv("TEST_DATABASE_DSN")
	if dsn == "" {
		t.Skip("set TEST_DATABASE_DSN to run PostgreSQL integration tests")
	}

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("connect to PostgreSQL: %v", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("get sql.DB: %v", err)
	}
	t.Cleanup(func() {
		if err := sqlDB.Close(); err != nil {
			t.Errorf("close PostgreSQL connection: %v", err)
		}
	})

	repository := NewRepository(db)
	if err := repository.Migrate(); err != nil {
		t.Fatalf("migrate test database: %v", err)
	}

	return repository, db
}

func integrationOrder(status string) *domain.Order {
	order := domain.NewOrder(
		"d8f3b2a1-0000-4a8a-8e2b-123456789abc",
		[]domain.OrderItem{{
			ProductID: "a1b2c3d4-e5f6-7a8b-9c0d-112233445566",
			SKU:       "SKU-INTEGRATION",
			Quantity:  2,
			UnitPrice: 12.50,
		}},
		domain.Address{Street: "1 Test Street", City: "Austin", State: "TX", PostalCode: "78701", Country: "USA"},
		domain.Address{Street: "2 Test Street", City: "Austin", State: "TX", PostalCode: "78702", Country: "USA"},
	)
	order.Status = status
	return order
}
