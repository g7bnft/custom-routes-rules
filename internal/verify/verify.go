package verify

import (
	"fmt"
	"os"

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
	fmt.Println("🔍 Verifying generated assets...")

	datBytes, err := os.ReadFile(cfg.RayDAT)
	if err != nil {
		return fmt.Errorf("geosite.dat missing: %w", err)
	}
	var siteList routercommon.GeoSiteList
	if err := proto.Unmarshal(datBytes, &siteList); err != nil {
		return fmt.Errorf("geosite.dat is corrupted: %w", err)
	}
	fmt.Printf(" ✅ geosite.dat is valid (%d entries)\n", len(siteList.Entry))

	info, err := os.Stat(cfg.SingboxDB)
	if err != nil {
		return fmt.Errorf("geosite.db missing: %w", err)
	}
	if info.Size() == 0 {
		return fmt.Errorf("geosite.db is empty")
	}
	fmt.Printf(" ✅ geosite.db exists (%d bytes)\n", info.Size())

	ipPath := cfg.OutputDir + "/geoip.db"
	ipdb, err := maxminddb.Open(ipPath)
	if err != nil {
		return fmt.Errorf("geoip.db is corrupted: %w", err)
	}
	if ipdb.Metadata.DatabaseType != "sing-geoip" {
		ipdb.Close()
		return fmt.Errorf("geoip.db missing 'sing-geoip' metadata signature")
	}
	ipdb.Close()
	fmt.Println(" ✅ geoip.db is valid and signed for sing-box")

	return nil
}
