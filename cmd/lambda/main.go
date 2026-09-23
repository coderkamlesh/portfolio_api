package main

import (
	"log"
	"net/http"

	"github.com/coderkamlesh/portfolio_api/internal/config"
	"github.com/coderkamlesh/portfolio_api/internal/database"
	"github.com/go-chi/chi/v5"
)

func main() {
	cfg := config.Load()

	db, err := database.Connect(cfg.TursoURL, cfg.TursoToken)
	if err != nil {
		log.Fatalf("❌ DB connect failed: %v", err)
	}
	defer db.Close()

	r := chi.NewRouter()

	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok","service":"portfolio-api"}`))
	})

	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"Health is good","message":"portfolio api project running"}`))
	})

	addr := ":" + cfg.ServerPort
	log.Printf("🚀 Server running on %s", addr)
	if err := http.ListenAndServe(addr, r); err != nil {
		log.Fatalf("❌ Server failed: %v", err)
	}
}
