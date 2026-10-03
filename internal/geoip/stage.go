package geoip

import (
	"fmt"
	"path/filepath"

	"custom-rules/internal/types"
)

type GeoipStage struct{}

func NewGeoipStage() *GeoipStage {
	return &GeoipStage{}
}

func (s *GeoipStage) Name() string { return "geoip" }

func (s *GeoipStage) Execute(ctx *types.BuildContext) error {
	cfg := ctx.Config

	tags := make([]string, 0, len(cfg.GeoIPTags))
	for _, t := range cfg.GeoIPTags {
		tags = append(tags, t.Tag)
	}

	result, err := scanGeoIP(cfg.GeoIPInput, tags)
	if err != nil {
		return fmt.Errorf("failed to scan MMDB: %w", err)
	}
	fmt.Printf(" └─ GeoIP: Extracted %d IP ranges across %d tags\n", countTotalIPs(result), len(result.CountryMap))

	if err := buildSingboxDB(result, tags, filepath.Join(cfg.OutputDir, "geoip.db")); err != nil {
		return fmt.Errorf("failed to build geoip.db: %w", err)
	}

	if err := buildXrayDAT(result, filepath.Join(cfg.OutputDir, "geoip.dat")); err != nil {
		return fmt.Errorf("failed to build geoip.dat: %w", err)
	}

	if err := exportIPRuleSets(result, cfg.OutputDir); err != nil {
		return fmt.Errorf("failed to export rule-sets: %w", err)
	}

	return nil
}
