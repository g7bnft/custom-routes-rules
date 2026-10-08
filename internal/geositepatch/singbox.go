package geositepatch

import (
	"bufio"
	// "encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"custom-rules/internal/types"

	singeosite "github.com/sagernet/sing-box/common/geosite"
	"github.com/sagernet/sing-box/common/srs"
	singconstant "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing/common"
	"github.com/v2fly/v2ray-core/v5/app/router/routercommon"
)

func ExportSingboxFormats(ctx *types.BuildContext, geoList *routercommon.GeoSiteList) error {
	fmt.Println("    └─ Generating Sing-box formats...")

	domainMap := buildDomainMap(geoList)

	// Optimize domainMap (remove redundant exact domains covered by suffixes)
	// to match the output of .srs/.json/.txt rule-sets
	for code, items := range domainMap {
		domainMap[code] = combItems(items)
	}

	dbFile, err := os.Create(ctx.Config.SingboxDB)
	if err != nil {
		return fmt.Errorf("failed to create singbox db file: %w", err)
	}
	defer dbFile.Close()

	bw := bufio.NewWriter(dbFile)
	if err := singeosite.Write(bw, domainMap); err != nil {
		return fmt.Errorf("failed to write singbox db: %w", err)
	}
	bw.Flush()

	if err := writeRuleSets(ctx.Config.OutputDir, domainMap); err != nil {
		return fmt.Errorf("failed to write singbox rulesets: %w", err)
	}

	return nil
}
// 1. It loops through each V2Ray entry (e.g. Code apple).
// 2. It initializes:
//      - mainDomains: Domains that belong to the base category (apple).
//      - attrBuckets: A temporary map grouping domains by their attributes (e.g., apple@ads, apple@cn).
// 3. For each domain inside the entry:
// 	    - It appends the domain to mainDomains.
//      - If the domain has attributes (like d.Attribute = ["ads"]), it appends that same domain to attrBuckets["ads"].
// 4. It then converts both the base category and all attribute categories into Sing-box rules:
//      - domainMap["apple"] = convertEntry(mainDomains)
//      - domainMap["apple@ads"] = convertEntry(domainsWithAds)
func buildDomainMap(geoList *routercommon.GeoSiteList) map[string][]singeosite.Item {
	domainMap := make(map[string][]singeosite.Item)

	for _, entry := range geoList.Entry {
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
	return domainMap
}
// This function maps V2Ray Domain types to Sing-box singeosite.Item types.
// Note how Domain_RootDomain splits into an exact match (RuleTypeDomain)
//  and a suffix match (RuleTypeDomainSuffix starting with .):

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
			items = append(items, singeosite.Item{Type: singeosite.RuleTypeDomain, Value: val}) // exact
		case routercommon.Domain_RootDomain:
			// exact + subdomain
			items = append(items, singeosite.Item{Type: singeosite.RuleTypeDomain, Value: val})
			suffix := val
			if !strings.HasPrefix(val, ".") {
				suffix = "." + val
			}
			items = append(items, singeosite.Item{Type: singeosite.RuleTypeDomainSuffix, Value: suffix}) // subdomain
		}
	}
	return common.Uniq(items)
}
// Redundancy Optimization (combItems)
// This is the helper function that optimizes the rule list before compiling into Sing-box binary (.srs) or writing JSON.

// The Problem: If you have a suffix rule like .example.com (which matches example.com and all subdomains), then an exact domain rule like www.example.com or blog.example.com is completely redundant.
// How combItems solves it:
// It builds a lookup set (suffixes) of all suffix rules (e.g., {"example.com": true}).
// It iterates through all exact domain rules (RuleTypeDomain).
// For each exact domain (e.g., blog.example.com), it splits the domain by . and checks parent domains from right to left:
// Check example.com $\rightarrow$ Found in suffixes!
// Since a parent suffix covers it, it marks isCovered = true.
// It filters out all isCovered exact domain rules, drastically reducing the final rule-set size.
func combItems(items []singeosite.Item) []singeosite.Item {
	// create empty lookup set for suffixes
	suffixes := make(map[string]bool)
	// collect all suffix rules (e.g., ".example.com")
	for _, item := range items {
		if item.Type == singeosite.RuleTypeDomainSuffix {
			suffixes[strings.TrimPrefix(item.Value, ".")] = true
		}
	}

	var result []singeosite.Item
	// check domain's every part against suffixes
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

func writeRuleSets(dir string, domainMap map[string][]singeosite.Item) error {
	for code, optimizedItems := range domainMap {
		compiled := singeosite.Compile(optimizedItems)

		plainRuleSet := option.PlainRuleSet{
			Rules: []option.HeadlessRule{{
				Type: singconstant.RuleTypeDefault,
				DefaultOptions: option.DefaultHeadlessRule{
					Domain:        compiled.Domain,
					DomainSuffix:  compiled.DomainSuffix,
					DomainKeyword: compiled.DomainKeyword,
					DomainRegex:   compiled.DomainRegex,
				},
			}},
		}

		// .srs
		sf, _ := os.Create(filepath.Join(dir, "geosite-"+code+".srs"))
		sbw := bufio.NewWriter(sf)
		srs.Write(sbw, plainRuleSet, 1)
		sbw.Flush()
		sf.Close()

		// .json
		// jf, _ := os.Create(filepath.Join(dir, "geosite-"+code+".json"))
		// enc := json.NewEncoder(jf)
		// enc.SetIndent("", " ")
		// enc.Encode(plainRuleSet)
		// jf.Close()

		// .txt
		// tf, _ := os.Create(filepath.Join(dir, "geosite-"+code+".txt"))
		// tbw := bufio.NewWriter(tf)
		// for _, item := range optimizedItems {
		// 	if item.Type == singeosite.RuleTypeDomainSuffix {
		// 		val := strings.TrimPrefix(item.Value, ".")
		// 		tbw.WriteByte('.')
		// 		tbw.WriteString(val)
		// 		tbw.WriteByte('\n')
		// 	}
		// }
		// tbw.Flush()
		// tf.Close()
	}

	return nil
}