// Wires the API and database, then drains HTTP requests on Ctrl+C before closing SQL.
package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	httpadapter "github.com/wbrooks8/go_order_api_gateway/internal/adapters/http"
	postgresadapter "github.com/wbrooks8/go_order_api_gateway/internal/adapters/postgres"
	"github.com/wbrooks8/go_order_api_gateway/internal/config"
	"github.com/wbrooks8/go_order_api_gateway/internal/service"
)

func main() {
	if err := run(); err != nil {
		log.Print(err)
		os.Exit(1)
	}
}

func run() (err error) {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	db, err := config.Connect()
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, config.Close(db)) }()
	repository := postgresadapter.NewRepository(db)
	if err := repository.Migrate(); err != nil {
		return err
	}
	router := gin.Default()
	httpadapter.RegisterRoutes(router, httpadapter.NewHandler(service.NewOrderUseCase(repository)))
	server := &http.Server{Addr: ":8080", Handler: router, ReadHeaderTimeout: 5 * time.Second}
	done := make(chan error, 1)
	go func() { done <- server.ListenAndServe() }()
	select {
	case err := <-done:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			return errors.Join(err, server.Close())
		}
		return nil
	}
}
