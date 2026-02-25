package main

import (
	"strings"
	"github.com/sagernet/sing-box/common/geosite"
	"github.com/sagernet/sing/common"
	"github.com/v2fly/v2ray-core/v5/app/router/routercommon"
	"google.golang.org/protobuf/proto"
)

func parse(vGeositeData []byte) (map[string][]geosite.Item, error) {
	vGeositeList := routercommon.GeoSiteList{}
	if err := proto.Unmarshal(vGeositeData, &vGeositeList); err != nil {
		return nil, err
	}

	domainMap := make(map[string][]geosite.Item)
	for _, entry := range vGeositeList.Entry {
		code := strings.ToLower(entry.CountryCode)
		attrBuckets := make(map[string][]*routercommon.Domain)
		mainDomains := make([]*routercommon.Domain, 0, len(entry.Domain))

		for _, d := range entry.Domain {
			mainDomains = append(mainDomains, d)
			for _, attr := range d.Attribute {
				attrBuckets[attr.Key] = append(attrBuckets[attr.Key], d)
			}
		}

		domainMap[code] = convertEntry(mainDomains)
		for attrName, domains := range attrBuckets {
			domainMap[code+"@"+attrName] = convertEntry(domains)
		}
	}
	return domainMap, nil
}

func convertEntry(vDomains []*routercommon.Domain) []geosite.Item {
	items := make([]geosite.Item, 0, len(vDomains)*2)
	for _, d := range vDomains {
		val := strings.ToLower(strings.TrimSpace(d.Value))
		switch d.Type {
		case routercommon.Domain_Plain:
			items = append(items, geosite.Item{Type: geosite.RuleTypeDomainKeyword, Value: val})
		case routercommon.Domain_Regex:
			items = append(items, geosite.Item{Type: geosite.RuleTypeDomainRegex, Value: val})
		case routercommon.Domain_Full:
			items = append(items, geosite.Item{Type: geosite.RuleTypeDomain, Value: val})
		case routercommon.Domain_RootDomain:
			items = append(items, geosite.Item{Type: geosite.RuleTypeDomain, Value: val})
			suffix := val
			if !strings.HasPrefix(val, ".") { suffix = "." + val }
			items = append(items, geosite.Item{Type: geosite.RuleTypeDomainSuffix, Value: suffix})
		}
	}
	return common.Uniq(items)
}

// combItems reduces the rule count by removing full domains 
// that are already covered by a suffix rule.
func combItems(items []geosite.Item) []geosite.Item {
	suffixes := make(map[string]bool)
	for _, item := range items {
		if item.Type == geosite.RuleTypeDomainSuffix {
			// Store suffix without leading dot for easy matching
			suffixes[strings.TrimPrefix(item.Value, ".")] = true
		}
	}

	var result []geosite.Item
	for _, item := range items {
		// Only try to "comb" full domain rules
		if item.Type == geosite.RuleTypeDomain {
			parts := strings.Split(item.Value, ".")
			isCovered := false
			
			// Check every possible parent suffix
			// e.g. for "a.b.google.com", check "b.google.com", "google.com", "com"
			for i := 1; i < len(parts); i++ {
				parent := strings.Join(parts[i:], ".")
				if suffixes[parent] {
					isCovered = true
					break
				}
			}
			if isCovered {
				continue // Skip this item, it's redundant
			}
		}
		result = append(result, item)
	}
	return result
}