package geosite

import (
	"fmt"
	"os"

	"custom-rules/internal/types"
)

type GeositeStage struct{}

func NewGeositeStage() *GeositeStage {
	return &GeositeStage{}
}

func (s *GeositeStage) Name() string { return "geosite" }

func (s *GeositeStage) Execute(ctx *types.BuildContext) error {
	cfg := ctx.Config

	data, err := os.ReadFile(cfg.GeositeInput)
	if err != nil {
		return fmt.Errorf("failed to read input file: %w", err)
	}
	if len(data) == 0 {
		return fmt.Errorf("input geosite.dat is empty")
	}

	filteredV2RayData, err := filterGeosite(data, cfg, ctx.Mods)
	if err != nil {
		return err
	}

	if err := os.WriteFile(cfg.RayDAT, filteredV2RayData, 0644); err != nil {
		return fmt.Errorf("failed to save filtered dat: %w", err)
	}

	domainMap, err := parse(filteredV2RayData)
	if err != nil {
		return err
	}

	totalDomains := 0
	for _, items := range domainMap {
		totalDomains += len(items)
	}
	fmt.Printf(" └─ Geosite: Processed %d domain rules across %d tags\n", totalDomains, len(domainMap))

	dbFile, err := os.Create(cfg.SingboxDB)
	if err != nil {
		return err
	}
	defer dbFile.Close()

	if err := writeGeositeDB(dbFile, domainMap); err != nil {
		return err
	}

	return writeRuleSets(cfg.OutputDir, domainMap)
}
