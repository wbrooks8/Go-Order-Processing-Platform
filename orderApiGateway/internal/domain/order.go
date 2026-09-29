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

func NewOrder(customerID string, items []OrderItem, shippingAddress, billingAddress Address) *Order {
	order := &Order{
		CustomerID:      customerID,
		Status:          "Accepted",
		Currency:        "USD",
		Version:         1,
		ShippingAddress: shippingAddress,
		BillingAddress:  billingAddress,
		Items:           items,
	}

	for index := range order.Items {
		order.TotalAmount += float64(order.Items[index].Quantity) * order.Items[index].UnitPrice
	}

	return order
}
