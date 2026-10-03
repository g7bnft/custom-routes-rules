package strategy

import (
	"strings"

	"custom-rules/internal/config"
	"custom-rules/internal/types"

	"github.com/v2fly/v2ray-core/v5/app/router/routercommon"
)

// Apply mutates entry.Domain in place: removing domains marked for
// removal under the entry's mapped category, and appending any
// manually-added domains for that category.
func Apply(entry *routercommon.GeoSite, tag string, cfg *config.Config, mods *types.ConfigMods) {
	category := categoryFor(tag, cfg)
	if category == "" {
		return
	}

	if rmMap, ok := mods.Rms[category]; ok {
		filtered := make([]*routercommon.Domain, 0, len(entry.Domain))
		for _, d := range entry.Domain {
			val := strings.ToLower(d.Value)

			if rmMap[val] {
				continue
			}

			isRemoved := false
			parts := strings.Split(val, ".")
			for i := 1; i < len(parts)-1; i++ {
				if len(parts[i:]) < 2 {
					break
				}
				parent := strings.Join(parts[i:], ".")
				if rmMap[parent] {
					isRemoved = true
					break
				}
			}

			if !isRemoved {
				filtered = append(filtered, d)
			}
		}
		entry.Domain = filtered
	}

	if addList, ok := mods.Adds[category]; ok {
		for _, val := range addList {
			entry.Domain = append(entry.Domain, &routercommon.Domain{
				Type:  routercommon.Domain_RootDomain,
				Value: strings.ToLower(val),
			})
		}
	}
}

// categoryFor looks up which strategy category (reject/proxy/direct) a
// geosite tag belongs to, based on the TargetTags configured for this build.
func categoryFor(tag string, cfg *config.Config) string {
	for _, t := range cfg.TargetTags {
		if strings.ToLower(t.Tag) == tag {
			return t.Strategy
		}
	}
	return ""
}
