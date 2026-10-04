// Implements the repository interface using GORM and PostgreSQL. Creates,
// loads, and updates order records, prepares tables, and converts between
// database models and domain orders. Cancellation decisions belong to the domain.

package postgres

import (
	"context"
	"errors"

	"github.com/wbrooks8/go_order_api_gateway/internal/domain"
	"gorm.io/gorm"
)

type Repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) CreateOrder(ctx context.Context, order *domain.Order) error {
	model := orderToModel(order)
	if err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return tx.Create(&model).Error
	}); err != nil {
		return err
	}

	orderFromModel(order, model)
	return nil
}

func (r *Repository) Migrate() error {
	return r.db.AutoMigrate(&OrderModel{}, &OrderItemModel{})
}

func (r *Repository) GetOrderByID(ctx context.Context, id string) (*domain.Order, error) {
	var model OrderModel
	err := r.db.WithContext(ctx).Preload("Items").First(&model, "id = ?", id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, domain.ErrOrderNotFound
		}
		return nil, err
	}

	order := &domain.Order{}
	orderFromModel(order, model)
	return order, nil
}

// UpdateOrder updates order fields; order items are left unchanged.
func (r *Repository) UpdateOrder(ctx context.Context, order *domain.Order) error {
	model := orderToModel(order)
	model.Version = order.Version + 1
	// Explicit fields include zero values and avoid creating a missing order.
	result := r.db.WithContext(ctx).Model(&OrderModel{}).
		Where("id = ? AND version = ?", order.ID, order.Version).
		Select("CustomerID", "Status", "TotalAmount", "Currency", "Version", "ShippingAddress", "BillingAddress").
		Updates(&model)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		var count int64
		if err := r.db.WithContext(ctx).Model(&OrderModel{}).Where("id = ?", order.ID).Count(&count).Error; err != nil {
			return err
		}
		if count == 0 {
			return domain.ErrOrderNotFound
		}
		return domain.ErrOrderConflict
	}
	order.Version = model.Version
	return nil
}

func orderToModel(order *domain.Order) OrderModel {
	model := OrderModel{
		ID:              order.ID,
		CustomerID:      order.CustomerID,
		Status:          order.Status,
		TotalAmount:     order.TotalAmount,
		Currency:        order.Currency,
		Version:         order.Version,
		ShippingAddress: order.ShippingAddress,
		BillingAddress:  order.BillingAddress,
		Items:           make([]OrderItemModel, 0, len(order.Items)),
	}
	for _, item := range order.Items {
		model.Items = append(model.Items, OrderItemModel{
			ID:        item.ID,
			OrderID:   item.OrderID,
			ProductID: item.ProductID,
			SKU:       item.SKU,
			Quantity:  item.Quantity,
			UnitPrice: item.UnitPrice,
		})
	}

	return model
}

func orderFromModel(order *domain.Order, model OrderModel) {
	order.ID = model.ID
	order.CustomerID = model.CustomerID
	order.Status = model.Status
	order.TotalAmount = model.TotalAmount
	order.Currency = model.Currency
	order.Version = model.Version
	order.ShippingAddress = model.ShippingAddress
	order.BillingAddress = model.BillingAddress
	order.Items = make([]domain.OrderItem, 0, len(model.Items))
	for _, item := range model.Items {
		order.Items = append(order.Items, domain.OrderItem{
			ID:        item.ID,
			OrderID:   item.OrderID,
			ProductID: item.ProductID,
			SKU:       item.SKU,
			Quantity:  item.Quantity,
			UnitPrice: item.UnitPrice,
		})
	}
}
