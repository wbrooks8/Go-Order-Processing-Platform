// Tests order creation, total calculation, and cancellation rules directly.
// These tests do not need an HTTP server or a database.

package domain

import (
	"errors"
	"testing"
)

func TestNewOrderInitializesOrderAndCalculatesTotal(t *testing.T) {
	items := []OrderItem{
		{ProductID: "a1b2c3d4-e5f6-4a8b-9c0d-112233445566", SKU: "SKU-1", Quantity: 2, UnitPrice: 125000},
		{ProductID: "a1b2c3d4-e5f6-4a8b-9c0d-112233445567", SKU: "SKU-2", Quantity: 1, UnitPrice: 32500},
	}
	shipping := Address{Street: "1 Main St", City: "Austin", State: "TX", PostalCode: "78701", Country: "USA"}
	billing := Address{Street: "2 Main St", City: "Austin", State: "TX", PostalCode: "78702", Country: "USA"}

	order, err := NewOrder("d8f3b2a1-0000-4a8a-8e2b-123456789abc", items, shipping, billing)

	if err != nil {
		t.Fatalf("Test failed with error %v", err)
	}
	if order.CustomerID != "d8f3b2a1-0000-4a8a-8e2b-123456789abc" {
		t.Errorf("CustomerID = %q, want valid customer UUID", order.CustomerID)
	}
	if order.Status != "Accepted" {
		t.Errorf("Status = %q, want Accepted", order.Status)
	}
	if order.Currency != "USD" {
		t.Errorf("Currency = %q, want USD", order.Currency)
	}
	if order.Version != 1 {
		t.Errorf("Version = %d, want 1", order.Version)
	}
	if order.TotalAmount != 282500 {
		t.Errorf("TotalAmount = %v, want 28.25", order.TotalAmount)
	}
	if len(order.Items) != len(items) {
		t.Fatalf("got %d items, want %d", len(order.Items), len(items))
	}
	if order.ShippingAddress != shipping {
		t.Errorf("ShippingAddress = %+v, want %+v", order.ShippingAddress, shipping)
	}
	if order.BillingAddress != billing {
		t.Errorf("BillingAddress = %+v, want %+v", order.BillingAddress, billing)
	}
}

// Each case changes one part of an otherwise valid order.
func TestNewOrderRejectsInvalidInput(t *testing.T) {
	tests := []struct {
		name   string
		change func(*string, *[]OrderItem)
	}{
		{"missing customer", func(customer *string, items *[]OrderItem) { *customer = "" }},
		{"nil items", func(customer *string, items *[]OrderItem) { *items = nil }},
		{"empty items", func(customer *string, items *[]OrderItem) { *items = []OrderItem{} }},
		{"missing product", func(customer *string, items *[]OrderItem) { (*items)[0].ProductID = "" }},
		{"zero quantity", func(customer *string, items *[]OrderItem) { (*items)[0].Quantity = 0 }},
		{"negative quantity", func(customer *string, items *[]OrderItem) { (*items)[0].Quantity = -1 }},
		{"zero price", func(customer *string, items *[]OrderItem) { (*items)[0].UnitPrice = 0 }},
		{"negative price", func(customer *string, items *[]OrderItem) { (*items)[0].UnitPrice = -1 }},
		{"invalid second item", func(customer *string, items *[]OrderItem) {
			*items = append(*items, OrderItem{ProductID: "a1b2c3d4-e5f6-4a8b-9c0d-112233445567", Quantity: 0, UnitPrice: 50000})
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			customer := "d8f3b2a1-0000-4a8a-8e2b-123456789abc"
			items := []OrderItem{{ProductID: "a1b2c3d4-e5f6-4a8b-9c0d-112233445566", Quantity: 2, UnitPrice: 125000}}
			test.change(&customer, &items)
			order, err := NewOrder(customer, items, Address{}, Address{})
			if !errors.Is(err, ErrInvalidOrder) {
				t.Errorf("error = %v, want ErrInvalidOrder", err)
			}
			if order != nil {
				t.Errorf("order = %+v, want nil", order)
			}
		})
	}
}

func TestCancelOrderTransitionsStatusCorrectly(t *testing.T) {
	order := &Order{Status: "Accepted"}

	if order.Cancel() != nil {
		t.Errorf("CancelOrder returned an error")
	}
	if order.Status != "Cancelled" {
		t.Errorf("Status = %q, want Cancelled", order.Status)
	}

	if order.Cancel() != nil {
		t.Errorf("CancelOrder returned an error")
	}
	if order.Status != "Cancelled" {
		t.Errorf("Status = %q after second cancel, want Cancelled", order.Status)
	}

	order.Status = "Paid"
	err := order.Cancel()
	if !errors.Is(err, ErrOrderCannotBeCancelled) {
		t.Errorf("CancelOrder did not return Order Cannot Be Cancelled Error")
	}
	if order.Status != "Paid" {
		t.Errorf("Status = %q after failed cancel, want Paid", order.Status)
	}
}
