package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/coderkamlesh/portfolio_api/internal/config"
	"github.com/coderkamlesh/portfolio_api/internal/database"
	"github.com/coderkamlesh/portfolio_api/internal/router"
	"github.com/coderkamlesh/portfolio_api/internal/server"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	cfg := config.Load()

	db, err := database.Connect(cfg.TursoURL, cfg.TursoToken)
	if err != nil {
		log.Fatalf("❌ DB connect failed: %v", err)
	}
	defer db.Close()

	handler, err := router.New(ctx, cfg, db)
	if err != nil {
		log.Fatalf("❌ Router setup failed: %v", err)
	}

	if err := server.Run(ctx, cfg, handler); err != nil {
		log.Fatalf("❌ Server failed: %v", err)
	}
	log.Println("👋 Server stopped")
}
