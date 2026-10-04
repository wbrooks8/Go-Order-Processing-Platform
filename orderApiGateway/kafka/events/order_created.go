// Package events defines the JSON messages shared by the Kafka demos.
// These message types are separate from the API's database-backed domain order.
package events

// OrderCreatedEvent is the demo payload. The production event contract will
// add metadata and order items before inventory processing is implemented.
type OrderCreatedEvent struct {
	OrderID    string `json:"orderId"`
	CustomerID string `json:"customerId"`
}
