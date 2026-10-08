package config

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// StrategyRule defines a named strategy and all the geosite/geoip tags assigned to it.
type StrategyRule struct {
	Name string   // e.g. "reject", "proxy", "direct"
	Tags []string // e.g. ["category-ads-all", "win-spy"]
}

// Config holds all file paths and URLs for the project.
type Config struct {
	GeositeURL string
	GeoIPURL string
	GeositeInput string
	GeoIPInput string
	UserStrategyDir string
	AssetsDir string
	OutputDir string
	SingboxDB string
	RayDAT string
	GeositeStrategies []StrategyRule // Clean, grouped strategy declarations
	GeoIPStrategies   []StrategyRule
	DownloadTimeout time.Duration
}

// DefaultConfig returns a ready-to-use configuration.
func DefaultConfig() *Config {
	// Base GitHub mirror prefix (can be set to "" for direct downloads)
	mirrorPrefix := "https://gh.jsdelivr.fyi/"
	// assets directory
	assets := "assets"
	// Custom output folder
	outputDir := "build"
	// User strategy directory
	userStrategyDir := "userstrategy"
	// Download timeout
	downloadTimeout := 1 * time.Minute

	return &Config{
		GeositeURL:      mirrorPrefix + "https://github.com/Loyalsoldier/v2ray-rules-dat/releases/latest/download/geosite.dat",
		GeoIPURL:        mirrorPrefix + "https://github.com/Loyalsoldier/geoip/releases/latest/download/Country.mmdb",
		GeositeInput:    filepath.Join(assets, "geosite.dat"),
		GeoIPInput:      filepath.Join(assets, "Country.mmdb"),
		UserStrategyDir: userStrategyDir,
		AssetsDir:       assets,
		OutputDir:       outputDir,
		SingboxDB:       filepath.Join(outputDir, "geosite.db"),
		RayDAT:          filepath.Join(outputDir, "geosite.dat"),
		// 1. Every strategy appears exactly once
		// 2. Adding a new tag or a new strategy is done strictly in config
		GeositeStrategies: []StrategyRule{
			{Name: "reject", Tags: []string{"category-ads-all", "win-spy"}},
			{Name: "proxy", Tags: []string{"gfw"}},
			{Name: "direct", Tags: []string{"cn"}},
		},
		GeoIPStrategies: []StrategyRule{
			{Name: "IPproxy", Tags: []string{"telegram"}},
			{Name: "IPdirect", Tags: []string{"private", "cn"}},
		},
		DownloadTimeout: downloadTimeout,
	}
}

// Validate creates required workspace folders before pipeline execution.
func (c *Config) Validate() error {
	if c.GeositeURL == "" || c.GeoIPURL == "" || c.GeositeInput == "" ||
	  c.UserStrategyDir == "" || c.AssetsDir == "" || c.OutputDir == "" ||
	  c.GeositeStrategies == nil || c.GeoIPStrategies == nil {
		return fmt.Errorf("geosite.dat or Country.mmdb URL is not set")
	}
	return nil
}

// Prepares the woorkspace for pipeline execution.
func (c *Config) PrepareWorkspace() error {
	dirs := []string{c.AssetsDir, c.OutputDir, c.UserStrategyDir}
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return err
		}
	}
	return nil

}