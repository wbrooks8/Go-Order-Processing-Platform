// Defines orders, items, and addresses, calculates a new order's total, and
// enforces the cancellation rule. These rules work without HTTP or a database.

package domain

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
	UnitPrice float64
}

type Order struct {
	ID              string
	CustomerID      string
	Status          string
	TotalAmount     float64
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

	if err := order.IsValidOrder(); err != nil {
		return nil, ErrInvalidOrder
	}

	for index := range order.Items {
		order.TotalAmount += float64(order.Items[index].Quantity) * order.Items[index].UnitPrice
	}

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

func (order *Order) IsValidOrder() error {
	if order.CustomerID == "" || len(order.Items) < 1 {
		return ErrInvalidOrder
	}

	for _, item := range order.Items {
		if item.ProductID == "" || item.Quantity <= 0 || item.UnitPrice <= 0.0 {
			return ErrInvalidOrder
		}
	}

	return nil
}
