package main

import (
	"fmt"
	"os"

	"github.com/oschwald/maxminddb-golang"
	"github.com/v2fly/v2ray-core/v5/app/router/routercommon"
	"google.golang.org/protobuf/proto"
)

func verifyOutputs() error {
	fmt.Println("🔍 Verifying generated assets...")

	// 1. Verify geosite.dat (Protobuf)
	datPath := RayDAT
	datBytes, err := os.ReadFile(datPath)
	if err != nil {
		return fmt.Errorf("geosite.dat missing: %w", err)
	}
	var siteList routercommon.GeoSiteList
	if err := proto.Unmarshal(datBytes, &siteList); err != nil {
		return fmt.Errorf("geosite.dat is corrupted: %w", err)
	}
	fmt.Printf("  ✅ geosite.dat is valid (%d entries)\n", len(siteList.Entry))

	// 2. Verify geosite.db (sing-box MMDB)
	dbPath := SingboxDB
	db, err := maxminddb.Open(dbPath)
	if err != nil {
		return fmt.Errorf("geosite.db is corrupted/invalid: %w", err)
	}
	db.Close()
	fmt.Println("  ✅ geosite.db is a valid MMDB")

	// 3. Verify geoip.db (sing-box GeoIP)
	ipPath := OutputDir + "/geoip.db"
	ipdb, err := maxminddb.Open(ipPath)
	if err != nil {
		return fmt.Errorf("geoip.db is corrupted: %w", err)
	}
	// Verify the sing-box signature
	if ipdb.Metadata.DatabaseType != "sing-geoip" {
		ipdb.Close()
		return fmt.Errorf("geoip.db missing 'sing-geoip' metadata signature")
	}
	ipdb.Close()
	fmt.Println("  ✅ geoip.db is valid and signed for sing-box")

	return nil
}