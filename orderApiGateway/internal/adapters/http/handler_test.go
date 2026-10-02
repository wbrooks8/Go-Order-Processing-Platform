// Sends simulated HTTP requests through the router and checks response
// statuses and JSON bodies. Uses the real service with a fake repository,
// so no running HTTP server or PostgreSQL database is required.

package http

import (
	"context"
	"encoding/json"
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
	getCalls     int
	updateCalls  int
	createCalls  int
	createdOrder *domain.Order
	createErr    error
	getOrder     *domain.Order
	getErr       error
	updateErr    error
}

func (fake *fakeOrderRepository) CreateOrder(_ context.Context, order *domain.Order) error {
	fake.createCalls++
	fake.createdOrder = order
	if fake.createErr == nil {
		order.ID = "44f551b5-0c28-4132-bcd4-d09b048dfe61"
	}
	return fake.createErr
}

func (fake *fakeOrderRepository) GetOrderByID(_ context.Context, _ string) (*domain.Order, error) {
	fake.getCalls++
	return fake.getOrder, fake.getErr
}

func (fake *fakeOrderRepository) UpdateOrder(_ context.Context, _ *domain.Order) error {
	fake.updateCalls++
	return fake.updateErr
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

func TestCreateOrderRejectsInvalidRequests(t *testing.T) {
	tests := []struct {
		name   string
		change func(*CreateOrderRequest)
	}{
		{"malformed customer UUID", func(r *CreateOrderRequest) { r.CustomerID = "banana" }},
		{"malformed product UUID", func(r *CreateOrderRequest) { r.Items[0].ProductID = "banana" }},
		{"malformed second product UUID", func(r *CreateOrderRequest) {
			item := r.Items[0]
			item.ProductID = "banana"
			r.Items = append(r.Items, item)
		}},
		{"missing customer", func(r *CreateOrderRequest) { r.CustomerID = "" }},
		{"nil items", func(r *CreateOrderRequest) { r.Items = nil }},
		{"empty items", func(r *CreateOrderRequest) { r.Items = []CreateOrderItemRequest{} }},
		{"missing product", func(r *CreateOrderRequest) { r.Items[0].ProductID = "" }},
		{"zero quantity", func(r *CreateOrderRequest) { r.Items[0].Quantity = 0 }},
		{"negative quantity", func(r *CreateOrderRequest) { r.Items[0].Quantity = -1 }},
		{"zero price", func(r *CreateOrderRequest) { r.Items[0].UnitPrice = 0 }},
		{"negative price", func(r *CreateOrderRequest) { r.Items[0].UnitPrice = -1 }},
		{"invalid second item", func(r *CreateOrderRequest) {
			item := r.Items[0]
			item.Quantity = 0
			r.Items = append(r.Items, item)
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var input CreateOrderRequest
			if err := json.Unmarshal([]byte(validCreateOrderJSON), &input); err != nil {
				t.Fatal(err)
			}
			test.change(&input)
			body, err := json.Marshal(input)
			if err != nil {
				t.Fatal(err)
			}
			assertCreateRejected(t, string(body))
		})
	}
	t.Run("malformed JSON", func(t *testing.T) { assertCreateRejected(t, `{"customerId":`) })
}

func assertCreateRejected(t *testing.T, body string) {
	t.Helper()
	repository := &fakeOrderRepository{}
	router := newTestRouter(repository)
	request := httptest.NewRequest(http.MethodPost, "/api/v1/orders", strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400; body: %s", response.Code, response.Body.String())
	}
	if repository.createCalls != 0 {
		t.Error("invalid request reached the repository")
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
			request := httptest.NewRequest(http.MethodGet, "/api/v1/orders/44f551b5-0c28-4132-bcd4-d09b048dfe61", nil)

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
		updateErr  error
		wantStatus int
		wantBody   string
	}{
		{name: "accepted", order: testOrderWithStatus("Accepted"), wantStatus: http.StatusOK, wantBody: `"status":"Cancelled"`},
		{name: "Cancelled", order: testOrderWithStatus("Cancelled"), wantStatus: http.StatusOK, wantBody: `"status":"Cancelled"`},
		{name: "not found", err: domain.ErrOrderNotFound, wantStatus: http.StatusNotFound, wantBody: `"message":"Order not found."`},
		{name: "cannot cancel", order: testOrderWithStatus("Paid"), wantStatus: http.StatusConflict, wantBody: `"message":"Order cannot be cancelled in its current state."`},
		{name: "repository failure", err: errors.New("database unavailable"), wantStatus: http.StatusInternalServerError, wantBody: `"message":"Could not cancel order."`},
		{name: "saving fails", order: testOrderWithStatus("Accepted"), updateErr: errors.New("save failed"), wantStatus: http.StatusInternalServerError, wantBody: `"message":"Could not cancel order."`},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			router := newTestRouter(&fakeOrderRepository{getOrder: test.order, getErr: test.err, updateErr: test.updateErr})
			response := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodPost, "/api/v1/orders/44f551b5-0c28-4132-bcd4-d09b048dfe61/cancel", nil)

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

func TestOrderEndpointsRejectInvalidUUIDBeforeRepository(t *testing.T) {
	for _, endpoint := range []struct{ method, suffix string }{
		{http.MethodGet, ""}, {http.MethodPost, "/cancel"},
	} {
		for _, id := range []string{"banana", "44f551b5-0c28-4132-bcd4-d09b048dfe6z"} {
			t.Run(endpoint.method+"/"+id, func(t *testing.T) {
				repository := &fakeOrderRepository{getOrder: testOrder()}
				router := newTestRouter(repository)
				response := httptest.NewRecorder()
				request := httptest.NewRequest(endpoint.method, "/api/v1/orders/"+id+endpoint.suffix, nil)
				router.ServeHTTP(response, request)
				if response.Code != http.StatusBadRequest {
					t.Errorf("status = %d, want 400; body: %s", response.Code, response.Body.String())
				}
				if repository.getCalls != 0 || repository.updateCalls != 0 {
					t.Error("invalid ID reached repository")
				}
			})
		}
	}
}
