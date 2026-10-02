// Reads HTTP requests, calls the order service, and sends JSON responses.
// It translates service errors into HTTP statuses such as 404 or 409.

package http

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/wbrooks8/go_order_api_gateway/internal/domain"
	"github.com/wbrooks8/go_order_api_gateway/internal/service"
)

type Handler struct {
	orders *service.OrderUseCase
}

func NewHandler(orders *service.OrderUseCase) *Handler {
	return &Handler{orders: orders}
}

func (h *Handler) createOrder(context *gin.Context) {
	var request CreateOrderRequest

	if err := context.ShouldBindJSON(&request); err != nil {
		context.JSON(http.StatusBadRequest, gin.H{"message": "Could not parse request data."})
		return
	}

	order, err := request.toDomain()

	if err == domain.ErrInvalidUUID {
		context.JSON(http.StatusBadRequest, gin.H{"message": "Invalid UUID."})
		return
	}
	if err != nil {
		context.JSON(http.StatusBadRequest, gin.H{"message": "Could not create order."})
		return
	}

	order, err = h.orders.CreateOrder(context.Request.Context(), order)

	if err != nil {
		context.JSON(http.StatusInternalServerError, gin.H{"message": "Could not create order."})
		return
	}

	context.JSON(http.StatusAccepted, toOrderResponse(order))
}

func (h *Handler) getOrder(context *gin.Context) {
	orderID := context.Param("id")

	order, err := h.orders.GetOrderByID(context.Request.Context(), orderID)
	if errors.Is(err, domain.ErrOrderNotFound) {
		context.JSON(http.StatusNotFound, gin.H{"message": "Order not found."})
		return
	}
	if errors.Is(err, domain.ErrInvalidUUID) {
		context.JSON(http.StatusBadRequest, gin.H{"message": "UUID is invalid"})
		return
	}
	if err != nil {
		context.JSON(http.StatusInternalServerError, gin.H{"message": "Could not retrieve order."})
		return
	}
	context.JSON(http.StatusOK, toOrderResponse(order))
}

func (h *Handler) cancelOrder(context *gin.Context) {
	orderID := context.Param("id")

	order, err := h.orders.CancelOrder(context.Request.Context(), orderID)

	if errors.Is(err, domain.ErrOrderNotFound) {
		context.JSON(http.StatusNotFound, gin.H{"message": "Order not found."})
		return
	}
	if errors.Is(err, domain.ErrInvalidUUID) {
		context.JSON(http.StatusBadRequest, gin.H{"message": "UUID is invalid"})
		return
	}
	if errors.Is(err, domain.ErrOrderCannotBeCancelled) {
		context.JSON(http.StatusConflict, gin.H{"message": "Order cannot be cancelled in its current state."})
		return
	}

	if err != nil {
		context.JSON(http.StatusInternalServerError, gin.H{"message": "Could not cancel order."})
		return
	}

	context.JSON(http.StatusOK, toOrderResponse(order))
}
