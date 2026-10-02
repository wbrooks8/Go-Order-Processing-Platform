// Connects each HTTP method and URL path to the handler that processes it.
// This is the list of order endpoints exposed by the application.

package http

import (
	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func RegisterRoutes(server *gin.Engine, handler *Handler) {
	promHandler := gin.WrapH(promhttp.Handler())
	server.Use(requestMetrics)
	server.POST("/api/v1/orders", handler.createOrder)
	server.GET("/api/v1/orders/:id", handler.getOrder)
	server.POST("/api/v1/orders/:id/cancel", handler.cancelOrder)
	server.GET("/metrics", promHandler)

}
