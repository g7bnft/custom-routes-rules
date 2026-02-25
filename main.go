package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
)

const (
	GeositeURL		= "https://gh.jsdelivr.fyi/https://github.com/Loyalsoldier/v2ray-rules-dat/releases/latest/download/geosite.dat"
	GeoIPURL		= "https://gh.jsdelivr.fyi/https://github.com/Loyalsoldier/geoip/releases/latest/download/Country.mmdb"
	GeositeInput    = "assets/geosite.dat"
	GeoIPInput    	= "assets/Country.mmdb"
	StrategyDir		= "strategy"
	OutputDir   	= "output"
	SingboxDB = OutputDir + "/geosite.db"
	RayDAT = OutputDir + "/geosite.dat"
)

var (
	targetTags = []string{"category-ads-all", "cn", "geolocation-!cn", "gfw", "win-spy"}
	geoipTags = []string{"cn", "private", "telegram"}
)

func main() {
	// Sync Upstream Files (NEW)
    // We do this first so we have the raw data to work with.
    fmt.Println("🔄 Syncing Upstream Data...")
    if err := syncAssets(); err != nil {
        log.Fatalf("❌ Sync error: %v", err)
    }

	// Setup Environment
	fmt.Println("🚀 Starting build process...")

	// FIRST: Remove the old folder if it exists
	os.RemoveAll(OutputDir)

	// SECOND: Create a fresh one
    if err := os.MkdirAll(OutputDir, 0755); err != nil {
        log.Fatalf("Failed to create output directory: %v", err)
    }
	
	// Handle Geosite Logic
	fmt.Println("🌐 Processing Geosite...")
	if err := processGeosite(); err != nil {
		log.Fatalf("❌ Geosite error: %v", err)
	}

	// Handle GeoIP Logic
	fmt.Println("📍 Processing GeoIP...")
	if err := processGeoIP(); err != nil {
		log.Fatalf("❌ GeoIP error: %v", err)
	}

	fmt.Println("✨ Build Complete! Files available in:", OutputDir)
}

func processGeosite() error {

	data, err := os.ReadFile(GeositeInput)
	if err != nil {
        return fmt.Errorf("failed to read input file: %w", err)
    }
	mods := loadAllModules()

	if len(data) == 0 { return fmt.Errorf("input geosite.dat is empty") }
	filteredV2RayData, err := filterGeosite(data, targetTags, mods)
	if err != nil { return err }

	// os.WriteFile(RayDAT, filteredV2RayData, 0644)
	if err := os.WriteFile(RayDAT, filteredV2RayData, 0644); err != nil {
    return fmt.Errorf("failed to save filtered dat: %w", err)
	}

	domainMap, err := parse(filteredV2RayData)
	if err != nil { return err }
	totalDomains := 0
	for _, items := range domainMap {
    	totalDomains += len(items)
	}
	fmt.Printf("  └─ Geosite: Processed %d domain rules across %d tags\n", totalDomains, len(domainMap))

	dbFile, err := os.Create(SingboxDB)
	if err != nil { return err }
	defer dbFile.Close()
	writeGeositeDB(dbFile, domainMap)

	return writeRuleSets(OutputDir, domainMap)
}

func processGeoIP() error {
	// 1. Scan (The Scanner logic)
	result, err := scanGeoIP(GeoIPInput, geoipTags)
	if err != nil {
		return fmt.Errorf("failed to scan MMDB: %w", err)
	}
	fmt.Printf("  └─ GeoIP: Extracted %d IP ranges across %d tags\n", countTotalIPs(result), len(result.CountryMap))

	// 2. Build sing-box geoip.db
	if err := buildSingboxDB(result, filepath.Join(OutputDir, "geoip.db")); err != nil {
		return fmt.Errorf("failed to build geoip.db: %w", err)
	}

	// 3. Build Xray geoip.dat
	if err := buildXrayDAT(result, filepath.Join(OutputDir, "geoip.dat")); err != nil {
		return fmt.Errorf("failed to build geoip.dat: %w", err)
	}

	// 4. Export SRS/JSON/TXT details
	if err := exportIPRuleSets(result, OutputDir); err != nil {
		return fmt.Errorf("failed to export rule-sets: %w", err)
	}

	return nil
}

func loadAllModules() *configMods {
	mods := &configMods{
		adds: make(map[string][]string),
		rms:  make(map[string]map[string]bool),
	}

	categories := []string{"reject", "proxy", "direct"}
	for _, k := range categories {
		// Load additions
		mods.adds[k] = loadList(filepath.Join(StrategyDir, k + ".txt"))

		// Load removals
		rmMap := make(map[string]bool)
		for _, item := range loadList(filepath.Join(StrategyDir, k + "-need-to-remove.txt")) {
			// Clean the string to prevent matching failures
			cleanItem := strings.ToLower(strings.TrimSpace(item))
			if cleanItem != "" {
				rmMap[cleanItem] = true 
			}
		}
		mods.rms[k] = rmMap
		fmt.Printf("  └─ Module [%s]: %d adds, %d removals loaded\n", k, len(mods.adds[k]), len(mods.rms[k]))
	}
	// NEW: Perform the Deductive Reconcile
    reconcileConflicts(mods)

	// 3. Final Count Summary
	fmt.Print("  └─ Final Strategy: ")
	summary := []string{}
	for _, k := range categories {
		summary = append(summary, fmt.Sprintf("[%s: %d(+)/%d(-)]", k, len(mods.adds[k]), len(mods.rms[k])))
	}
	fmt.Println(strings.Join(summary, " "))
	
	return mods
}