package config

import "fmt"

type TagConfig struct {
	Tag      string
	Strategy string
}

type Config struct {
	GeositeURL string
	GeoIPURL string
	GeositeInput string
	GeoIPInput string
	StrategyDir string
	AssetsDir string
	OutputDir string
	SingboxDB string
	RayDAT string
	TargetTags []TagConfig
	GeoIPTags []TagConfig
}

func DefaultConfig() *Config {
	outputDir := "output"
	return &Config{
		GeositeURL: "https://github.com/Loyalsoldier/v2ray-rules-dat/releases/latest/download/geosite.dat",
		GeoIPURL: "https://github.com/Loyalsoldier/geoip/releases/latest/download/Country.mmdb",
		GeositeInput: "assets/geosite.dat",
		GeoIPInput: "assets/geosite.dat",
		StrategyDir: "strategy",
		AssetsDir: "assets",
		OutputDir: outputDir,
		SingboxDB: outputDir + "/geosite.db",
		RayDAT: outputDir + "/geosite.dat",
		TargetTags: []TagConfig{
			{"category-ads-all", "reject"},
			{"win-spy", "reject"},
			{"gfw", "proxy"},
			{"geolocation-!cn", "proxy"},
			{"cn", "direct"},
		},
		GeoIPTags: []TagConfig{
			{"telegram", "proxy"},
			{"private", "direct"},
			{"cn", "direct"},
		},

	}
}

func (c *Config) Validate() error {
	if c.GeositeURL == "" || c.GeoIPURL == "" {
		return fmt.Errorf("critical URLs are missing")
	}
	return nil
}
