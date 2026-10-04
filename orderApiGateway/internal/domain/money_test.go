// Exercises exact decimal boundaries and JSON/SQL round trips.
package domain

import (
	"encoding/json"
	"errors"
	"testing"
)

func TestMoneyRoundTrip(t *testing.T) {
	for _, test := range []struct {
		input string
		want  Money
	}{
		{"0.1", 1000}, {"12.50", 125000}, {"0.0001", 1}, {"1e-4", 1},
		{"99999999.9999", MaxMoney}, {"-12.50", -125000},
	} {
		t.Run(test.input, func(t *testing.T) {
			var m Money
			if err := json.Unmarshal([]byte(test.input), &m); err != nil {
				t.Fatal(err)
			}
			if m != test.want {
				t.Fatalf("got %d units, want %d", m, test.want)
			}
			data, err := json.Marshal(m)
			if err != nil {
				t.Fatal(err)
			}
			var decoded Money
			if err := json.Unmarshal(data, &decoded); err != nil || decoded != m {
				t.Fatalf("JSON round trip: %v, %v", decoded, err)
			}
			value, err := m.Value()
			if err != nil {
				t.Fatal(err)
			}
			var scanned Money
			if err := scanned.Scan(value); err != nil || scanned != m {
				t.Fatalf("SQL round trip: %v, %v", scanned, err)
			}
		})
	}
}

func TestMoneyRejectsUnrepresentableInput(t *testing.T) {
	for _, input := range []string{"0.00001", "100000000", "1e10000000", `"12.50"`, "null", "true", "NaN", "[]"} {
		t.Run(input, func(t *testing.T) {
			var value Money
			if err := json.Unmarshal([]byte(input), &value); err == nil {
				t.Fatal("expected rejection")
			}
		})
	}
}

func TestValidateRejectsIncorrectTotalAndInvalidIDs(t *testing.T) {
	order, err := NewOrder("d8f3b2a1-0000-4a8a-8e2b-123456789abc", []OrderItem{
		{ProductID: "a1b2c3d4-e5f6-4a8b-9c0d-112233445566", Quantity: 3, UnitPrice: 1000},
	}, Address{}, Address{})
	if err != nil {
		t.Fatal(err)
	}
	if order.TotalAmount != 3000 {
		t.Fatalf("exact 3 * 0.1 = %v, want 0.3000", order.TotalAmount)
	}
	order.TotalAmount++
	if !errors.Is(order.Validate(), ErrInvalidOrder) {
		t.Fatal("expected inconsistent total rejection")
	}
	order.TotalAmount--
	order.CustomerID = "banana"
	if !errors.Is(order.Validate(), ErrInvalidUUID) {
		t.Fatal("expected customer UUID rejection")
	}
	order.CustomerID = "d8f3b2a1-0000-4a8a-8e2b-123456789abc"
	order.Items[0].ProductID = "banana"
	if !errors.Is(order.Validate(), ErrInvalidUUID) {
		t.Fatal("expected product UUID rejection")
	}
	var absent *Order
	if !errors.Is(absent.Validate(), ErrInvalidOrder) {
		t.Fatal("expected nil rejection")
	}
}

func TestOrderTotalCannotOverflowDatabaseRange(t *testing.T) {
	_, err := NewOrder("d8f3b2a1-0000-4a8a-8e2b-123456789abc", []OrderItem{
		{ProductID: "a1b2c3d4-e5f6-4a8b-9c0d-112233445566", Quantity: 2, UnitPrice: MaxMoney},
	}, Address{}, Address{})
	if !errors.Is(err, ErrInvalidOrder) {
		t.Fatalf("overflow error = %v", err)
	}
}
