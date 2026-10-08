package verify

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/oschwald/maxminddb-golang"
	"github.com/v2fly/v2ray-core/v5/app/router/routercommon"
	"google.golang.org/protobuf/proto"

	"custom-rules/internal/types"
)

type VerifyStage struct{}

func NewVerifyStage() *VerifyStage {
	return &VerifyStage{}
}

func (s *VerifyStage) Name() string { return "verify" }

func (s *VerifyStage) Execute(ctx *types.BuildContext) error {
	cfg := ctx.Config
	fmt.Println("  [Verify] Checking generated assets...")

	// geosite.dat (V2Ray protobuf)
	datBytes, err := os.ReadFile(cfg.RayDAT)
	if err != nil {
		return fmt.Errorf("geosite.dat missing: %w", err)
	}
	var siteList routercommon.GeoSiteList
	if err := proto.Unmarshal(datBytes, &siteList); err != nil {
		return fmt.Errorf("geosite.dat is corrupted: %w", err)
	}
	fmt.Printf("    └─ geosite.dat valid (%d entries)\n", len(siteList.Entry))

	// geosite.db (sing-box's own binary format — not MMDB, so a
	// size check is the strongest check we can do without reaching
	// into sing-box's internal reader)
	info, err := os.Stat(cfg.SingboxDB)
	if err != nil {
		return fmt.Errorf("geosite.db missing: %w", err)
	}
	if info.Size() == 0 {
		return fmt.Errorf("geosite.db is empty")
	}
	fmt.Printf("    └─ geosite.db exists (%d bytes)\n", info.Size())

	// geoip.dat (V2Ray protobuf)
	geoipDatPath := filepath.Join(cfg.OutputDir, "geoip.dat")
	geoipDatBytes, err := os.ReadFile(geoipDatPath)
	if err != nil {
		return fmt.Errorf("geoip.dat missing: %w", err)
	}
	var ipList routercommon.GeoIPList
	if err := proto.Unmarshal(geoipDatBytes, &ipList); err != nil {
		return fmt.Errorf("geoip.dat is corrupted: %w", err)
	}
	fmt.Printf("    └─ geoip.dat valid (%d entries)\n", len(ipList.Entry))

	// geoip.db (real MMDB, signed for sing-box)
	ipdbPath := filepath.Join(cfg.OutputDir, "geoip.db")
	ipdb, err := maxminddb.Open(ipdbPath)
	if err != nil {
		return fmt.Errorf("geoip.db is corrupted: %w", err)
	}
	if ipdb.Metadata.DatabaseType != "sing-geoip" {
		ipdb.Close()
		return fmt.Errorf("geoip.db missing 'sing-geoip' metadata signature")
	}
	ipdb.Close()
	fmt.Println("    └─ geoip.db valid and signed for sing-box")

	return nil
}