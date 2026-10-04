// Checks that converting between database models and domain orders preserves
// order fields, addresses, and items. These tests do not connect to PostgreSQL.

package postgres

import (
	"reflect"
	"testing"

	"github.com/wbrooks8/go_order_api_gateway/internal/domain"
)

func TestOrderModelConversionRoundTrip(t *testing.T) {
	want := &domain.Order{
		ID:          "44f551b5-0c28-4132-bcd4-d09b048dfe61",
		CustomerID:  "d8f3b2a1-0000-4a8a-8e2b-123456789abc",
		Status:      "Accepted",
		TotalAmount: 12999900,
		Currency:    "USD",
		Version:     1,
		ShippingAddress: domain.Address{
			Street: "123 Tech Boulevard", City: "Austin", State: "TX", PostalCode: "78701", Country: "USA",
		},
		BillingAddress: domain.Address{
			Street: "456 Main Street", City: "Austin", State: "TX", PostalCode: "78702", Country: "USA",
		},
		Items: []domain.OrderItem{{
			ID: 2, OrderID: "44f551b5-0c28-4132-bcd4-d09b048dfe61",
			ProductID: "a1b2c3d4-e5f6-7a8b-9c0d-112233445566", SKU: "PROD-LPT-001",
			Quantity: 1, UnitPrice: 12999900,
		}},
	}

	model := orderToModel(want)
	got := &domain.Order{}
	orderFromModel(got, model)

	if !reflect.DeepEqual(got, want) {
		t.Errorf("round-trip order mismatch\ngot:  %#v\nwant: %#v", got, want)
	}
}

func TestOrderFromModelCopiesAllFields(t *testing.T) {
	model := OrderModel{
		ID: "order-1", CustomerID: "d8f3b2a1-0000-4a8a-8e2b-123456789abc", Status: "Cancelled", TotalAmount: 425000,
		Currency: "USD", Version: 2,
		ShippingAddress: domain.Address{Street: "1 First St", City: "Austin", State: "TX", PostalCode: "78701", Country: "USA"},
		BillingAddress:  domain.Address{Street: "2 Second St", City: "Austin", State: "TX", PostalCode: "78702", Country: "USA"},
		Items: []OrderItemModel{{
			ID: 4, OrderID: "order-1", ProductID: "a1b2c3d4-e5f6-4a8b-9c0d-112233445566", SKU: "SKU-1", Quantity: 2, UnitPrice: 212500,
		}},
	}
	order := &domain.Order{}

	orderFromModel(order, model)

	if order.ID != model.ID || order.CustomerID != model.CustomerID || order.Status != model.Status {
		t.Errorf("order identity/status fields not copied: %+v", order)
	}
	if order.TotalAmount != model.TotalAmount || order.Currency != model.Currency || order.Version != model.Version {
		t.Errorf("order amount/version fields not copied: %+v", order)
	}
	if order.ShippingAddress != model.ShippingAddress || order.BillingAddress != model.BillingAddress {
		t.Errorf("order addresses not copied: %+v", order)
	}
	if len(order.Items) != 1 || order.Items[0].ID != 4 || order.Items[0].SKU != "SKU-1" {
		t.Errorf("order items not copied: %+v", order.Items)
	}
}
