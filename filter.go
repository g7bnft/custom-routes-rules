package main

import (
	"fmt"
	"strings"
	"github.com/v2fly/v2ray-core/v5/app/router/routercommon"
	"google.golang.org/protobuf/proto"
)

func filterGeosite(vGeositeData []byte, targets []string, mods *configMods) ([]byte, error) {
	originalList := routercommon.GeoSiteList{}
	if err := proto.Unmarshal(vGeositeData, &originalList); err != nil {
		return nil, err
	}

	newList := &routercommon.GeoSiteList{
		Entry: make([]*routercommon.GeoSite, 0, len(targets)),
	}

	targetMap := make(map[string]bool)
	for _, t := range targets {
		targetMap[strings.ToLower(t)] = true
	}

	for _, entry := range originalList.Entry {
		tag := strings.ToLower(entry.CountryCode)
		if !targetMap[tag] {
			continue
		}

		applyModifications(entry, tag, mods)
		entry.Domain = deduplicateDomains(entry.Domain)
		newList.Entry = append(newList.Entry, entry)
	}

	return proto.Marshal(newList)
}

func applyModifications(entry *routercommon.GeoSite, tag string, mods *configMods) {
	categoryMap := map[string]string{
		"cn":               "direct",
		"gfw":              "proxy",
		"geolocation-!cn":  "proxy",
		"category-ads-all": "reject",
	}

	category := categoryMap[tag]
	if category == "" { return }

	if rmMap, ok := mods.rms[category]; ok {
		filtered := make([]*routercommon.Domain, 0, len(entry.Domain))
		for _, d := range entry.Domain {
			val := strings.ToLower(d.Value)
        
			// Check exact match first
			if rmMap[val] {
				continue 
			}

			// Check every parent level (Tree-Aware removal)
			isRemoved := false
			parts := strings.Split(val, ".")
			for i := 1; i < len(parts)-1; i++ { // Start from 1 to skip the full string, end before the TLD
				// 1. Safety check first: If we only have 1 part left (e.g., "com"), stop.
				// parts[i:] represents the remaining segments.
				if len(parts[i:]) < 2 { 
					break // Use break instead of continue here, as further i++ will only be shorter
				}

				// 2. Only join if it's a valid parent (e.g., "paypal.com")
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

	if addList, ok := mods.adds[category]; ok {
		for _, val := range addList {
			entry.Domain = append(entry.Domain, &routercommon.Domain{
				Type:  routercommon.Domain_RootDomain,
				Value: strings.ToLower(val),
			})
		}
	}
}

func deduplicateDomains(domains []*routercommon.Domain) []*routercommon.Domain {
	unique := make([]*routercommon.Domain, 0, len(domains))
	seen := make(map[domainKey]struct{})
	for _, d := range domains {
		d.Value = strings.ToLower(strings.TrimSpace(d.Value))
		key := domainKey{d.Type, d.Value}
		if _, exists := seen[key]; !exists {
			seen[key] = struct{}{}
			unique = append(unique, d)
		}
	}
	return unique
}

func reconcileConflicts(mods *configMods) {
    categories := []string{"reject", "proxy", "direct"}
    claimed := make(map[string]string)

    for _, currentCat := range categories {
        for _, domain := range mods.adds[currentCat] {
            
            if owner, exists := claimed[domain]; exists {
                fmt.Printf("    ⚠️  [Conflict] '%s' in %s is IGNORED (already claimed by %s)\n", domain, currentCat, owner)
                continue 
            }
            claimed[domain] = currentCat

            if mods.rms[currentCat][domain] {
                fmt.Printf("    🔧 [Self-Clean] Removed '%s' from %s-need-to-remove\n", domain, currentCat)
                delete(mods.rms[currentCat], domain)
            }

            // This ensures that even if you didn't put it in the -need-to-remove.txt,
            // the code INSERTS it into the other maps.
            for _, otherCat := range categories {
                if currentCat == otherCat { continue }
                
                if !mods.rms[otherCat][domain] {
                    fmt.Printf("    🔗 [Priority] Forced '%s' into %s-need-to-remove (overridden by %s)\n", domain, otherCat, currentCat)
                    mods.rms[otherCat][domain] = true
                }
            }
        }
    }
}