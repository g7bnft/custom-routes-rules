package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"custom-rules/internal/config"
	"custom-rules/internal/geoip"
	"custom-rules/internal/geositepatch"
	"custom-rules/internal/pipeline"
	"custom-rules/internal/strategy"
	"custom-rules/internal/sync"
	"custom-rules/internal/types"
	"custom-rules/internal/verify"
)


func main() {
	fmt.Println("Starting build process...")

	// 1. Setup Config
	cfg := config.DefaultConfig()
	if err := cfg.Validate(); err != nil {
		log.Fatalf("Config validation failed: %v", err)
	}

	// 2. Setup Environment
	// if err := os.RemoveAll(cfg.AssetsDir); err != nil {
	// 	log.Fatalf("Failed to clean assets directory: %v", err)
	// }
	if err := os.RemoveAll(cfg.OutputDir); err != nil {
		log.Fatalf("Failed to clean output directory: %v", err)
	}
	if err := cfg.PrepareWorkspace(); err != nil {
		log.Fatalf("Failed to prepare workspace: %v", err)
	}

	// 3. Initialize Pipeline
	p := pipeline.NewPipeline()

	// 4. Add Stages
	p.AddStage(sync.NewSyncStage()) // real sync stage plugin
	p.AddStage(strategy.NewStrategyStage()) // real strategy stage plugin
	p.AddStage(geositepatch.NewGeositeStage()) // Real Protobuf patch stage!
	p.AddStage(geoip.NewGeoipStage()) // Real GeoIP stage!
	p.AddStage(verify.NewVerifyStage()) // Real GeoIP stage!

	// 5. Execute Pipeline
	pCtx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	ctx := types.NewBuildContext(pCtx, cfg)

	if err := p.Run(ctx); err != nil {
		log.Fatalf("Pipeline failed: %v", err)
	}

	fmt.Println("Build Complete! Files available in: ", cfg.OutputDir)
}