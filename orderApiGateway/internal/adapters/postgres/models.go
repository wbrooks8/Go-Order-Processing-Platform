// Defines how orders and items are stored in PostgreSQL, including table
// names, column types, keys, and their relationship. GORM uses these structs
// and tags when creating tables and reading or writing records.

package postgres

import "github.com/wbrooks8/go_order_api_gateway/internal/domain"

type OrderModel struct {
	ID              string           `gorm:"type:uuid;default:gen_random_uuid();primaryKey"`
	CustomerID      string           `gorm:"type:uuid;not null"`
	Status          string           `gorm:"type:varchar(32);not null"`
	TotalAmount     float64          `gorm:"type:decimal(12,4);not null"`
	Currency        string           `gorm:"type:varchar(3);not null;default:USD"`
	Version         int              `gorm:"not null;default:1"`
	ShippingAddress domain.Address   `gorm:"type:jsonb;not null;serializer:json"`
	BillingAddress  domain.Address   `gorm:"type:jsonb;not null;serializer:json"`
	Items           []OrderItemModel `gorm:"foreignKey:OrderID;constraint:OnDelete:CASCADE"`
}

func (OrderModel) TableName() string { return "orders" }

type OrderItemModel struct {
	ID        uint    `gorm:"primaryKey"`
	OrderID   string  `gorm:"type:uuid;not null;index"`
	ProductID string  `gorm:"type:uuid;not null"`
	SKU       string  `gorm:"type:varchar(64);not null"`
	Quantity  int     `gorm:"not null"`
	UnitPrice float64 `gorm:"type:decimal(12,4);not null"`
}

func (OrderItemModel) TableName() string { return "order_items" }
