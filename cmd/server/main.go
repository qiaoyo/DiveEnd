package main

import (
	"fmt"
	"log"
	"net/http"

	"github.com/qiaoyo/DiveEnd/internal/api"
	"github.com/qiaoyo/DiveEnd/internal/config"
	"github.com/qiaoyo/DiveEnd/internal/data"
	"github.com/qiaoyo/DiveEnd/internal/sync"
)

func main() {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Printf("Warning: Failed to load config: %v, using defaults", err)
		cfg = config.Default()
	}

	// Initialize database
	db, err := data.InitDB(cfg.DataPath)
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer db.Close()

	// Initialize Baidu Cloud sync
	if cfg.BaiduCloud.Enabled {
		go sync.StartBackgroundSync(cfg.BaiduCloud, cfg.DataPath)
	}

	// Setup routes
	handler := api.SetupRoutes(db, cfg)

	// Start server
	log.Printf("DiveEnd server starting on http://localhost:%d", cfg.Port)
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", cfg.Port), handler))
}
