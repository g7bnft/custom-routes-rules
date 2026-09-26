package main

import (
	"fmt"
	"log"
	"os"

	"custom-rules/internal/config"

	"custom-rules/internal/pipeline"
	"custom-rules/internal/sync"
	"custom-rules/internal/strategy"
	"custom-rules/internal/geosite"
	"custom-rules/internal/geoip"
	"custom-rules/internal/verify"
)


func main()  {
	fmt.Println("Starting build process...")

	// 1. Setup Config
	cfg := config.DefaultConfig()
	if err := cfg.Validate(); err != nil {
		log.Fatalf("Config validation failed: %v", err)
	}

	// 2. Setup Environment
	if err := os.RemoveAll(cfg.OutputDir); err != nil {
		log.Fatalf("Failed to clean output directory: %v", err)
	}
	if err := os.RemoveAll(cfg.AssetsDir); err != nil {
		log.Fatalf("Failed to clean assets directory: %v", err)
	}
	if err := os.MkdirAll(cfg.OutputDir, 0755); err != nil {
		log.Fatalf("Failed to create output directory: %v", err)
	}
	if err := os.MkdirAll(cfg.AssetsDir, 0755); err != nil {
		log.Fatalf("Failed to create assets directory: %v", err)
	}

	// 3. Initialize Pipeline
	p := pipeline.NewPipeline()

	// 4. Add Stages
	p.AddStage(sync.NewSyncStage())
	p.AddStage(strategy.NewStrategyStage())
	p.AddStage(geosite.NewGeositeStage())
	p.AddStage(geoip.NewGeoipStage())
	p.AddStage(verify.NewVerifyStage())




}