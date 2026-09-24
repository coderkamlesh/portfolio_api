// Package server runs the HTTP server with sane timeouts and graceful
// shutdown. Both cmd/api (local/ECS) and cmd/lambda (Lambda Web Adapter) use
// it because the adapter expects a normal HTTP server on PORT.
package server

import (
	"context"
	"errors"
	"log"
	"net/http"
	"time"

	"github.com/coderkamlesh/portfolio_api/internal/config"
)

// Run starts the listener and blocks until the context is cancelled or the
// server fails. A cancelled context triggers a graceful drain.
func Run(ctx context.Context, cfg *config.Config, handler http.Handler) error {
	srv := &http.Server{
		Addr:              ":" + cfg.ServerPort,
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	shutdownDone := make(chan struct{})
	go func() {
		defer close(shutdownDone)
		<-ctx.Done()

		shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		log.Println("🛑 Shutting down HTTP server…")
		if err := srv.Shutdown(shutdownCtx); err != nil {
			log.Printf("⚠️  server: graceful shutdown failed: %v", err)
		}
	}()

	log.Printf("🚀 Server running on %s", srv.Addr)
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}

	<-shutdownDone
	return nil
}
