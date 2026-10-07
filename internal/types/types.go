package types

import (
	"context"
	"custom-rules/internal/config"
	"net"

	"github.com/oschwald/maxminddb-golang"
)

type ConfigMods struct {
	Adds map[string][]string
	Rms  map[string]map[string]bool
}

type BuildContext struct {
	context.Context
	Config *config.Config
	Mods   *ConfigMods
}

// GeoIPResult holds the scanned MMDB metadata plus each matched
// country/tag's IP ranges.
type GeoIPResult struct {
	Metadata   maxminddb.Metadata
	CountryMap map[string][]*net.IPNet
}

// NewBuildContext initializes and returns a ready-to-use BuildContext.
func NewBuildContext(parentCtx context.Context, cfg *config.Config) *BuildContext {
	// 1. Fallback if parentCtx is nil
	if parentCtx == nil {
		parentCtx = context.Background()
	}

	// 2. Pre-initialize maps in ConfigMods to prevent nil map assignment panics
	return &BuildContext{
		Context: parentCtx,
		Config:  cfg,
		Mods: &ConfigMods{
			Adds: make(map[string][]string),
			Rms:  make(map[string]map[string]bool),
		},
	}
}