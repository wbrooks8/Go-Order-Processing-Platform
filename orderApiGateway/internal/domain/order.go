// Defines orders, items, and addresses, calculates a new order's total, and
// enforces the cancellation rule. These rules work without HTTP or a database.

package domain

import "github.com/google/uuid"

type Address struct {
	Street     string
	City       string
	State      string
	PostalCode string
	Country    string
}

type OrderItem struct {
	ID        uint
	OrderID   string
	ProductID string
	SKU       string
	Quantity  int
	UnitPrice Money
}

type Order struct {
	ID              string
	CustomerID      string
	Status          string
	TotalAmount     Money
	Currency        string
	Version         int
	ShippingAddress Address
	BillingAddress  Address
	Items           []OrderItem
}

func NewOrder(customerID string, items []OrderItem, shippingAddress, billingAddress Address) (*Order, error) {
	order := &Order{
		CustomerID:      customerID,
		Status:          OrderStatusAccepted,
		Currency:        "USD",
		Version:         1,
		ShippingAddress: shippingAddress,
		BillingAddress:  billingAddress,
		Items:           items,
	}

	total, err := order.calculatedTotal()
	if err != nil {
		return nil, err
	}
	order.TotalAmount = total

	return order, nil
}

func (order *Order) Cancel() error {
	switch order.Status {
	case OrderStatusAccepted:
		order.Status = OrderStatusCancelled
		return nil
	case OrderStatusCancelled:
		return nil
	default:
		return ErrOrderCannotBeCancelled
	}
}

// Validate is shared by constructors and services, including non-HTTP callers.
func (order *Order) Validate() error {
	total, err := order.calculatedTotal()
	if err != nil {
		return err
	}
	if order.TotalAmount != total || order.Currency != "USD" {
		return ErrInvalidOrder
	}
	return nil
}

func ValidateUUID(id string) error {
	if uuid.Validate(id) != nil {
		return ErrInvalidUUID
	}
	return nil
}

func (order *Order) calculatedTotal() (Money, error) {
	if order == nil || order.CustomerID == "" || len(order.Items) == 0 {
		return 0, ErrInvalidOrder
	}
	if err := ValidateUUID(order.CustomerID); err != nil {
		return 0, err
	}
	var total Money
	for _, item := range order.Items {
		if item.ProductID == "" || item.Quantity <= 0 || item.UnitPrice <= 0 {
			return 0, ErrInvalidOrder
		}
		if err := ValidateUUID(item.ProductID); err != nil {
			return 0, err
		}
		// Check before multiplying so large quantities cannot overflow.
		if item.UnitPrice > MaxMoney || int64(item.Quantity) > int64((MaxMoney-total)/item.UnitPrice) {
			return 0, ErrInvalidOrder
		}
		total += Money(item.Quantity) * item.UnitPrice
	}
	return total, nil
}
