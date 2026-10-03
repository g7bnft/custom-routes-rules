package geosite

import (
	"strings"

	"custom-rules/internal/config"
	"custom-rules/internal/strategy"
	"custom-rules/internal/types"

	"github.com/v2fly/v2ray-core/v5/app/router/routercommon"
	"google.golang.org/protobuf/proto"
)

// filterGeosite keeps only the entries matching the configured target tags,
// applies strategy add/remove modifications to each, and deduplicates domains.
func filterGeosite(vGeositeData []byte, cfg *config.Config, mods *types.ConfigMods) ([]byte, error) {
	originalList := routercommon.GeoSiteList{}
	if err := proto.Unmarshal(vGeositeData, &originalList); err != nil {
		return nil, err
	}

	targetMap := make(map[string]bool)
	for _, t := range cfg.TargetTags {
		targetMap[strings.ToLower(t.Tag)] = true
	}

	newList := &routercommon.GeoSiteList{
		Entry: make([]*routercommon.GeoSite, 0, len(cfg.TargetTags)),
	}

	for _, entry := range originalList.Entry {
		tag := strings.ToLower(entry.CountryCode)
		if !targetMap[tag] {
			continue
		}

		strategy.Apply(entry, tag, cfg, mods)
		entry.Domain = deduplicateDomains(entry.Domain)

		newList.Entry = append(newList.Entry, entry)
	}

	return proto.Marshal(newList)
}

func deduplicateDomains(domains []*routercommon.Domain) []*routercommon.Domain {
	unique := make([]*routercommon.Domain, 0, len(domains))
	seen := make(map[types.DomainKey]struct{})

	for _, d := range domains {
		d.Value = strings.ToLower(strings.TrimSpace(d.Value))
		key := types.DomainKey{Type: d.Type, Value: d.Value}
		if _, exists := seen[key]; !exists {
			seen[key] = struct{}{}
			unique = append(unique, d)
		}
	}

	return unique
}
