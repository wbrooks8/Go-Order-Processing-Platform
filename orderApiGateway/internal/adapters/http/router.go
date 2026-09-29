package http

import "github.com/gin-gonic/gin"

func RegisterRoutes(server *gin.Engine, handler *Handler) {
	server.POST("/api/v1/orders", handler.createOrder)
	server.GET("/api/v1/orders/:id", handler.getOrder)
	server.POST("/api/v1/orders/:id/cancel", handler.cancelOrder)
}
