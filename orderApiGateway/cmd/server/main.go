package main

import (
	"log"

	"github.com/gin-gonic/gin"
	httpadapter "github.com/wbrooks8/go_order_api_gateway/internal/adapters/http"
	postgresadapter "github.com/wbrooks8/go_order_api_gateway/internal/adapters/postgres"
	"github.com/wbrooks8/go_order_api_gateway/internal/config"
	"github.com/wbrooks8/go_order_api_gateway/internal/service"
)

func main() {
	db, err := config.Connect()
	if err != nil {
		log.Fatal(err)
	}
	defer config.Close(db)

	orderRepository := postgresadapter.NewRepository(db)
	if err := orderRepository.Migrate(); err != nil {
		log.Fatal(err)
	}

	orderService := service.NewOrderUseCase(orderRepository)
	server := gin.Default()
	httpadapter.RegisterRoutes(server, httpadapter.NewHandler(orderService))

	if err := server.Run(":8080"); err != nil {
		log.Fatal(err)
	}
}
