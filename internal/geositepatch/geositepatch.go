package geositepatch

import (
	"fmt"
	"os"

	"custom-rules/internal/types"

	"google.golang.org/protobuf/proto"
)

type GeositeStage struct{}

func NewGeositeStage() *GeositeStage {
	return &GeositeStage{}
}

func (s *GeositeStage) Name() string { return "geosite" }

func (s *GeositeStage) Execute(ctx *types.BuildContext) error {
	fmt.Println("  [Geosite] Starting Geosite processing workflows...")

	// Workflow 1: read upstream, merge by strategy, apply mods, dedupe
	geoList, err := BuildPatchedGeoSiteList(ctx)
	if err != nil {
		return fmt.Errorf("repack geosite.dat (filter&merge&mod) failed: %w", err)
	}

	v2rayBytes, err := proto.Marshal(geoList)
	if err != nil {
		return fmt.Errorf("repack geosite.dat (marshal) failed: %w", err)
	}
	if err := os.WriteFile(ctx.Config.RayDAT, v2rayBytes, 0644); err != nil {
		return fmt.Errorf("repack geosite.dat (write dat) failed: %w", err)
	}
	fmt.Printf("    └─ Wrote patched V2Ray dat: %s\n", ctx.Config.RayDAT)

	// Workflow 2 & 3: sing-box formats, built directly from geoList — no re-parse
	if err := ExportSingboxFormats(ctx, geoList); err != nil {
		return fmt.Errorf("transform geosite.dat (sing-box export) failed: %w", err)
	}

	fmt.Println("  [Geosite] All workflows completed successfully.")
	return nil
}