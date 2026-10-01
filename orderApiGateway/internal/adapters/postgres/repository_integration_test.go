// Tests creating, loading, and updating orders against a real PostgreSQL
// database, including updates to missing orders. Requires TEST_DATABASE_DSN;
// these tests skip when it is unset and clean up the orders they create.

package postgres

import (
	"context"
	"errors"
	"os"
	"reflect"
	"testing"

	"github.com/wbrooks8/go_order_api_gateway/internal/domain"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func TestRepositoryIntegrationCreateGetAndUpdate(t *testing.T) {
	repository, db := newIntegrationRepository(t)
	ctx := context.Background()
	wantOrder := integrationOrder("Accepted", t)

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

	gotOrder.Status = "Paid"
	gotOrder.TotalAmount = 0
	gotOrder.ShippingAddress.City = "Dallas"
	if err := repository.UpdateOrder(ctx, gotOrder); err != nil {
		t.Fatalf("UpdateOrder returned error: %v", err)
	}
	retrieved, err := repository.GetOrderByID(ctx, wantOrder.ID)
	if err != nil {
		t.Fatalf("GetOrderByID after update: %v", err)
	}
	if !reflect.DeepEqual(retrieved, gotOrder) {
		t.Fatalf("persisted order = %+v, want %+v", retrieved, gotOrder)
	}
	if err := repository.UpdateOrder(ctx, gotOrder); err != nil {
		t.Fatalf("unchanged update returned error: %v", err)
	}

	_, err = repository.GetOrderByID(ctx, "00000000-0000-0000-0000-000000000000")
	if !errors.Is(err, domain.ErrOrderNotFound) {
		t.Fatalf("missing order error = %v, want ErrOrderNotFound", err)
	}
}

func TestRepositoryIntegrationUpdateMissingOrder(t *testing.T) {
	repository, db := newIntegrationRepository(t)
	ctx := context.Background()
	order := integrationOrder("Accepted", t)
	// Generate a unique ID, then delete the order so this test owns the missing ID.
	if err := repository.CreateOrder(ctx, order); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Where("id = ?", order.ID).Delete(&OrderModel{}) })
	if err := db.Where("id = ?", order.ID).Delete(&OrderModel{}).Error; err != nil {
		t.Fatal(err)
	}
	if err := repository.UpdateOrder(ctx, order); !errors.Is(err, domain.ErrOrderNotFound) {
		t.Fatalf("UpdateOrder error = %v, want ErrOrderNotFound", err)
	}
	if _, err := repository.GetOrderByID(ctx, order.ID); !errors.Is(err, domain.ErrOrderNotFound) {
		t.Fatalf("order must remain missing, got error %v", err)
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

func integrationOrder(status string, t *testing.T) *domain.Order {
	t.Helper()
	order, err := domain.NewOrder(
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

	if err != nil {
		t.Fatalf("New order not created %v.", err)
	}
	order.Status = status
	return order
}
