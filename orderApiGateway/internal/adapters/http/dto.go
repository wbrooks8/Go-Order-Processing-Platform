package http

import (
	"github.com/wbrooks8/go_order_api_gateway/internal/domain"
	"github.com/wbrooks8/go_order_api_gateway/internal/service"
)

type CreateOrderRequest struct {
	CustomerID      string                   `json:"customerId" binding:"required"`
	Items           []CreateOrderItemRequest `json:"items" binding:"required,min=1,dive"`
	ShippingAddress AddressRequest           `json:"shippingAddress" binding:"required"`
	BillingAddress  AddressRequest           `json:"billingAddress" binding:"required"`
}

type CreateOrderItemRequest struct {
	ProductID string  `json:"productId" binding:"required"`
	SKU       string  `json:"sku" binding:"required"`
	Quantity  int     `json:"quantity" binding:"required,gt=0"`
	UnitPrice float64 `json:"unitPrice" binding:"required,gt=0"`
}

type AddressRequest struct {
	Street     string `json:"street" binding:"required"`
	City       string `json:"city" binding:"required"`
	State      string `json:"state" binding:"required"`
	PostalCode string `json:"postalCode" binding:"required"`
	Country    string `json:"country" binding:"required"`
}

type OrderResponse struct {
	ID              string              `json:"orderId"`
	CustomerID      string              `json:"customerId"`
	Status          string              `json:"status"`
	TotalAmount     float64             `json:"totalAmount"`
	Currency        string              `json:"currency"`
	Version         int                 `json:"version"`
	ShippingAddress AddressRequest      `json:"shippingAddress"`
	BillingAddress  AddressRequest      `json:"billingAddress"`
	Items           []OrderItemResponse `json:"items"`
}

type OrderItemResponse struct {
	ID        uint    `json:"id"`
	OrderID   string  `json:"orderId"`
	ProductID string  `json:"productId"`
	SKU       string  `json:"sku"`
	Quantity  int     `json:"quantity"`
	UnitPrice float64 `json:"unitPrice"`
}

func (request CreateOrderRequest) toDomain() (*domain.Order, error) {
	err := service.IsValidUUID(request.CustomerID)
	if err != nil {
		return nil, domain.ErrInvalidUUID
	}
	items := make([]domain.OrderItem, 0, len(request.Items))
	for _, item := range request.Items {
		err := service.IsValidUUID(item.ProductID)
		if err != nil {
			return nil, domain.ErrInvalidUUID
		}
		items = append(items, domain.OrderItem{
			ProductID: item.ProductID,
			SKU:       item.SKU,
			Quantity:  item.Quantity,
			UnitPrice: item.UnitPrice,
		})
	}

	return domain.NewOrder(
		request.CustomerID,
		items,
		request.ShippingAddress.toDomain(),
		request.BillingAddress.toDomain(),
	)
}

func (address AddressRequest) toDomain() domain.Address {
	return domain.Address{
		Street:     address.Street,
		City:       address.City,
		State:      address.State,
		PostalCode: address.PostalCode,
		Country:    address.Country,
	}
}

func toOrderResponse(order *domain.Order) OrderResponse {
	items := make([]OrderItemResponse, 0, len(order.Items))
	for _, item := range order.Items {
		items = append(items, OrderItemResponse{
			ID:        item.ID,
			OrderID:   item.OrderID,
			ProductID: item.ProductID,
			SKU:       item.SKU,
			Quantity:  item.Quantity,
			UnitPrice: item.UnitPrice,
		})
	}

	return OrderResponse{
		ID:              order.ID,
		CustomerID:      order.CustomerID,
		Status:          order.Status,
		TotalAmount:     order.TotalAmount,
		Currency:        order.Currency,
		Version:         order.Version,
		ShippingAddress: addressFromDomain(order.ShippingAddress),
		BillingAddress:  addressFromDomain(order.BillingAddress),
		Items:           items,
	}
}

func addressFromDomain(address domain.Address) AddressRequest {
	return AddressRequest{
		Street:     address.Street,
		City:       address.City,
		State:      address.State,
		PostalCode: address.PostalCode,
		Country:    address.Country,
	}
}
