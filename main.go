package main

import (
	"context"
	_ "embed"
	"errors"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"time"

	"github.com/SeaRoll/oapi-sqlc/api"
	"github.com/SeaRoll/oapi-sqlc/database"
	"github.com/SeaRoll/oapi-sqlc/server"
	"github.com/labstack/echo/v4"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, os.Kill)
	defer cancel()

	db := database.NewDatabase("postgres://postgres:mysecretpassword@localhost:5432/foodie?sslmode=disable")
	defer db.Disconnect()

	e := echo.New()
	api.ServeDocs(e)

	server := server.NewServer(db)

	api.RegisterHandlers(e, api.NewStrictHandler(server, nil))

	srv := http.Server{
		Handler:           e,
		ReadHeaderTimeout: 5 * time.Second,
	}

	// Start the server.
	go func() {
		err := srv.ListenAndServe()
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatal(err)
		}
	}()

	// Wait for interrupt signal to gracefully shutdown the server.
	<-ctx.Done()

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer shutdownCancel()

	err := srv.Shutdown(shutdownCtx)
	if err != nil {
		slog.Error("Failed to shutdown server", "error", err)
	}
}
