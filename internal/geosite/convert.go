package geosite

import (
	"strings"

	singeosite "github.com/sagernet/sing-box/common/geosite"
	"github.com/sagernet/sing/common"
	"github.com/v2fly/v2ray-core/v5/app/router/routercommon"
	"google.golang.org/protobuf/proto"
)

// parse expands a filtered geosite protobuf list into a map of
// tag -> rule items, including "tag@attribute" buckets for entries
// carrying attributes (e.g. "cn@ads").
func parse(vGeositeData []byte) (map[string][]singeosite.Item, error) {
	vGeositeList := routercommon.GeoSiteList{}
	if err := proto.Unmarshal(vGeositeData, &vGeositeList); err != nil {
		return nil, err
	}

	domainMap := make(map[string][]singeosite.Item)

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

func convertEntry(vDomains []*routercommon.Domain) []singeosite.Item {
	items := make([]singeosite.Item, 0, len(vDomains)*2)

	for _, d := range vDomains {
		val := strings.ToLower(strings.TrimSpace(d.Value))

		switch d.Type {
		case routercommon.Domain_Plain:
			items = append(items, singeosite.Item{Type: singeosite.RuleTypeDomainKeyword, Value: val})
		case routercommon.Domain_Regex:
			items = append(items, singeosite.Item{Type: singeosite.RuleTypeDomainRegex, Value: val})
		case routercommon.Domain_Full:
			items = append(items, singeosite.Item{Type: singeosite.RuleTypeDomain, Value: val})
		case routercommon.Domain_RootDomain:
			items = append(items, singeosite.Item{Type: singeosite.RuleTypeDomain, Value: val})
			suffix := val
			if !strings.HasPrefix(val, ".") {
				suffix = "." + val
			}
			items = append(items, singeosite.Item{Type: singeosite.RuleTypeDomainSuffix, Value: suffix})
		}
	}

	return common.Uniq(items)
}

// combItems reduces the rule count by removing full domains
// that are already covered by a suffix rule.
func combItems(items []singeosite.Item) []singeosite.Item {
	suffixes := make(map[string]bool)
	for _, item := range items {
		if item.Type == singeosite.RuleTypeDomainSuffix {
			suffixes[strings.TrimPrefix(item.Value, ".")] = true
		}
	}

	var result []singeosite.Item
	for _, item := range items {
		if item.Type == singeosite.RuleTypeDomain {
			parts := strings.Split(item.Value, ".")
			isCovered := false
			for i := 1; i < len(parts); i++ {
				parent := strings.Join(parts[i:], ".")
				if suffixes[parent] {
					isCovered = true
					break
				}
			}
			if isCovered {
				continue
			}
		}
		result = append(result, item)
	}

	return result
}
