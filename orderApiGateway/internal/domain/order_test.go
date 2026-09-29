package domain

import (
	"math"
	"testing"
)

func TestNewOrderInitializesOrderAndCalculatesTotal(t *testing.T) {
	items := []OrderItem{
		{ProductID: "product-1", SKU: "SKU-1", Quantity: 2, UnitPrice: 12.50},
		{ProductID: "product-2", SKU: "SKU-2", Quantity: 1, UnitPrice: 3.25},
	}
	shipping := Address{Street: "1 Main St", City: "Austin", State: "TX", PostalCode: "78701", Country: "USA"}
	billing := Address{Street: "2 Main St", City: "Austin", State: "TX", PostalCode: "78702", Country: "USA"}

	order := NewOrder("customer-1", items, shipping, billing)

	if order.CustomerID != "customer-1" {
		t.Errorf("CustomerID = %q, want customer-1", order.CustomerID)
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
	if math.Abs(order.TotalAmount-28.25) > 1e-9 {
		t.Errorf("TotalAmount = %f, want 28.25", order.TotalAmount)
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

func TestNewOrderWithNoItemsHasZeroTotal(t *testing.T) {
	order := NewOrder("customer-1", nil, Address{}, Address{})

	if order.TotalAmount != 0 {
		t.Errorf("TotalAmount = %f, want 0", order.TotalAmount)
	}
	if len(order.Items) != 0 {
		t.Errorf("got %d items, want 0", len(order.Items))
	}
}
