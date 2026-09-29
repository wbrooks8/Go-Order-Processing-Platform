package http

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/wbrooks8/go_order_api_gateway/internal/domain"
	"github.com/wbrooks8/go_order_api_gateway/internal/service"
)

type fakeOrderRepository struct {
	createdOrder *domain.Order
	createErr    error
	getOrder     *domain.Order
	getErr       error
	cancelOrder  *domain.Order
	cancelErr    error
}

func (fake *fakeOrderRepository) CreateOrder(_ context.Context, order *domain.Order) error {
	fake.createdOrder = order
	if fake.createErr == nil {
		order.ID = "44f551b5-0c28-4132-bcd4-d09b048dfe61"
	}
	return fake.createErr
}

func (fake *fakeOrderRepository) GetOrderByID(_ context.Context, _ string) (*domain.Order, error) {
	return fake.getOrder, fake.getErr
}

func (fake *fakeOrderRepository) CancelOrder(_ context.Context, _ string) (*domain.Order, error) {
	return fake.cancelOrder, fake.cancelErr
}

func newTestRouter(repository *fakeOrderRepository) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	useCase := service.NewOrderUseCase(repository)
	RegisterRoutes(router, NewHandler(useCase))
	return router
}

func TestCreateOrderReturnsAcceptedAndMappedOrder(t *testing.T) {
	repository := &fakeOrderRepository{}
	router := newTestRouter(repository)
	body := `{"customerId":"d8f3b2a1-0000-4a8a-8e2b-123456789abc","items":[{"productId":"a1b2c3d4-e5f6-7a8b-9c0d-112233445566","sku":"PROD-LPT-001","quantity":1,"unitPrice":1299.99}],"shippingAddress":{"street":"123 Tech Boulevard","city":"Austin","state":"TX","postalCode":"78701","country":"USA"},"billingAddress":{"street":"123 Tech Boulevard","city":"Austin","state":"TX","postalCode":"78701","country":"USA"}}`
	request := httptest.NewRequest(http.MethodPost, "/api/v1/orders", strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	if response.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want %d; body: %s", response.Code, http.StatusAccepted, response.Body.String())
	}
	if repository.createdOrder == nil {
		t.Fatal("repository did not receive the created order")
	}
	if !strings.Contains(response.Body.String(), `"orderId":"44f551b5-0c28-4132-bcd4-d09b048dfe61"`) {
		t.Errorf("response does not contain generated order ID: %s", response.Body.String())
	}
	if !strings.Contains(response.Body.String(), `"totalAmount":1299.99`) {
		t.Errorf("response does not contain calculated total: %s", response.Body.String())
	}
	if !strings.Contains(response.Body.String(), `"productId":"a1b2c3d4-e5f6-7a8b-9c0d-112233445566"`) {
		t.Errorf("response does not contain mapped item: %s", response.Body.String())
	}
}

func TestCreateOrderRejectsInvalidJSON(t *testing.T) {
	router := newTestRouter(&fakeOrderRepository{})
	request := httptest.NewRequest(http.MethodPost, "/api/v1/orders", strings.NewReader(`{"customerId":"customer-1","items":[]}`))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d; body: %s", response.Code, http.StatusBadRequest, response.Body.String())
	}
}

func TestCreateOrderReturnsInternalServerErrorOnRepositoryFailure(t *testing.T) {
	router := newTestRouter(&fakeOrderRepository{createErr: errors.New("database unavailable")})
	request := httptest.NewRequest(http.MethodPost, "/api/v1/orders", strings.NewReader(validCreateOrderJSON))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	if response.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusInternalServerError)
	}
}

func TestGetOrderReturnsOrder(t *testing.T) {
	order := testOrder()
	router := newTestRouter(&fakeOrderRepository{getOrder: order})
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/v1/orders/44f551b5-0c28-4132-bcd4-d09b048dfe61", nil)

	router.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", response.Code, http.StatusOK, response.Body.String())
	}
	if !strings.Contains(response.Body.String(), `"status":"Accepted"`) {
		t.Errorf("response does not contain order status: %s", response.Body.String())
	}
	if !strings.Contains(response.Body.String(), `"sku":"PROD-LPT-001"`) {
		t.Errorf("response does not contain order item: %s", response.Body.String())
	}
}

func TestGetOrderMapsNotFoundAndInternalErrors(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		wantStatus int
	}{
		{name: "not found", err: domain.ErrOrderNotFound, wantStatus: http.StatusNotFound},
		{name: "repository failure", err: errors.New("database unavailable"), wantStatus: http.StatusInternalServerError},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			router := newTestRouter(&fakeOrderRepository{getErr: test.err})
			response := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodGet, "/api/v1/orders/order-1", nil)

			router.ServeHTTP(response, request)

			if response.Code != test.wantStatus {
				t.Fatalf("status = %d, want %d; body: %s", response.Code, test.wantStatus, response.Body.String())
			}
		})
	}
}

func TestCancelOrderMapsSuccessNotFoundConflictAndInternalErrors(t *testing.T) {
	tests := []struct {
		name       string
		order      *domain.Order
		err        error
		wantStatus int
		wantBody   string
	}{
		{name: "cancelled", order: testOrderWithStatus("CANCELLED"), wantStatus: http.StatusOK, wantBody: `"status":"CANCELLED"`},
		{name: "not found", err: domain.ErrOrderNotFound, wantStatus: http.StatusNotFound, wantBody: `"message":"Order not found."`},
		{name: "cannot cancel", err: domain.ErrOrderCannotBeCancelled, wantStatus: http.StatusConflict, wantBody: `"message":"Order cannot be cancelled in its current state."`},
		{name: "repository failure", err: errors.New("database unavailable"), wantStatus: http.StatusInternalServerError, wantBody: `"message":"Could not cancel order."`},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			router := newTestRouter(&fakeOrderRepository{cancelOrder: test.order, cancelErr: test.err})
			response := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodPost, "/api/v1/orders/order-1/cancel", nil)

			router.ServeHTTP(response, request)

			if response.Code != test.wantStatus {
				t.Fatalf("status = %d, want %d; body: %s", response.Code, test.wantStatus, response.Body.String())
			}
			if !strings.Contains(response.Body.String(), test.wantBody) {
				t.Errorf("body %q does not contain %q", response.Body.String(), test.wantBody)
			}
		})
	}
}

const validCreateOrderJSON = `{"customerId":"d8f3b2a1-0000-4a8a-8e2b-123456789abc","items":[{"productId":"a1b2c3d4-e5f6-7a8b-9c0d-112233445566","sku":"PROD-LPT-001","quantity":1,"unitPrice":1299.99}],"shippingAddress":{"street":"123 Tech Boulevard","city":"Austin","state":"TX","postalCode":"78701","country":"USA"},"billingAddress":{"street":"123 Tech Boulevard","city":"Austin","state":"TX","postalCode":"78701","country":"USA"}}`

func testOrder() *domain.Order {
	order := testOrderWithStatus("Accepted")
	order.Items = []domain.OrderItem{{
		ID:        2,
		OrderID:   order.ID,
		ProductID: "a1b2c3d4-e5f6-7a8b-9c0d-112233445566",
		SKU:       "PROD-LPT-001",
		Quantity:  1,
		UnitPrice: 1299.99,
	}}
	return order
}

func testOrderWithStatus(status string) *domain.Order {
	return &domain.Order{
		ID:          "44f551b5-0c28-4132-bcd4-d09b048dfe61",
		CustomerID:  "d8f3b2a1-0000-4a8a-8e2b-123456789abc",
		Status:      status,
		TotalAmount: 1299.99,
		Currency:    "USD",
		Version:     1,
	}
}
