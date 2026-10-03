package types

import (
	"net"

	"custom-rules/internal/config"

	"github.com/oschwald/maxminddb-golang"
	"github.com/v2fly/v2ray-core/v5/app/router/routercommon"
)

// DomainKey is used to deduplicate domain rules by (type, value).
type DomainKey struct {
	Type  routercommon.Domain_Type
	Value string
}

// ConfigMods holds the strategy add/remove lists per category
// (reject/proxy/direct), loaded from the strategy/*.txt files.
type ConfigMods struct {
	Adds map[string][]string
	Rms  map[string]map[string]bool
}

// GeoIPResult holds the scanned MMDB metadata plus each matched
// country/tag's IP ranges.
type GeoIPResult struct {
	Metadata   maxminddb.Metadata
	CountryMap map[string][]*net.IPNet
}

// BuildContext carries shared state between pipeline stages.
type BuildContext struct {
	Config *config.Config
	Mods   *ConfigMods // populated by the strategy stage, consumed by the geosite stage
}
