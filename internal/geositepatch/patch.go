package geositepatch

import (
	"fmt"
	"os"
	"strings"

	"custom-rules/internal/types"

	"github.com/v2fly/v2ray-core/v5/app/router/routercommon"
	"google.golang.org/protobuf/proto"
)

// BuildPatchedGeoSiteList reads upstream geosite data, merges each
// strategy's upstream tags into one entry, applies user Adds/Rms,
// dedupes, and returns the resulting structured list (unmarshaled —
// callers decide if/how to serialize it).
//
// Implements a unified Priority Cascade (Upstream + User):
// Categories processed in order (reject > proxy > direct).
// Each category has U (upstream domains), A (user adds), R (user removes).
// - Category A (Forced): Domain in A OR (U and not in R) -> claims for this category
// - Category B (Fallback): Domain in U but caught by R -> evicted, falls to next
// - Category C (Direct Entry): Missed higher, but in A or (U and not in R)
// - Category D (Void): Caught by R with no fallback -> dropped
func BuildPatchedGeoSiteList(ctx *types.BuildContext) (*routercommon.GeoSiteList, error) {
	fmt.Println("    └─ Patching upstream Geosite database...")

	data, err := os.ReadFile(ctx.Config.GeositeInput)
	if err != nil {
		return nil, fmt.Errorf("failed to read input: %w", err)
	}

	upstreamList := &routercommon.GeoSiteList{}
	if err := proto.Unmarshal(data, upstreamList); err != nil {
		return nil, fmt.Errorf("failed to unmarshal upstream geosite: %w", err)
	}

	upstreamMap := make(map[string]*routercommon.GeoSite)
	for _, site := range upstreamList.Entry {
		upstreamMap[strings.ToLower(site.CountryCode)] = site
	}

	// Collect upstream domains per strategy (U sets)
	upstreamDomains := make(map[string][]*routercommon.Domain)
	for _, strat := range ctx.Config.GeositeStrategies {
		var raw []*routercommon.Domain
		for _, tag := range strat.Tags {
			if upSite, exists := upstreamMap[strings.ToLower(tag)]; exists {
				raw = append(raw, upSite.Domain...)
			}
		}
		upstreamDomains[strat.Name] = raw
	}

	// Unified Priority Cascade: Upstream (U) + User Adds (A) + User Removes (R)
	// Process categories in priority order
	finalDomains := make(map[string][]*routercommon.Domain)
	claimed := make(map[string]string) // domain -> owning category

	for _, strat := range ctx.Config.GeositeStrategies {
		cat := strat.Name
		uSet := upstreamDomains[cat]  // Upstream domains for this category
		aSet := ctx.Mods.Adds[cat]    // User adds (pre-cascaded from Strategy Stage)
		rSet := ctx.Mods.Rms[cat]     // User removes

		// First: Handle upstream domains (U) for this category
		for _, dom := range uSet {
			cleanVal := strings.ToLower(strings.TrimSpace(dom.Value))

			// Check if already claimed by higher priority
			if owner, exists := claimed[cleanVal]; exists {
				fmt.Printf("    └─ [%s] upstream '%s' skipped — already claimed by [%s]\n", cat, cleanVal, owner)
				continue
			}
			// Check if this category's remove list has it (Category B: evict)
			if rSet != nil && rSet[cleanVal] {
				// Evicted from this category — allow fallback to lower
				continue
			}
			// Category A: Claim it
			claimed[cleanVal] = cat
			if finalDomains[cat] == nil {
				finalDomains[cat] = []*routercommon.Domain{}
			}
			finalDomains[cat] = append(finalDomains[cat], dom)
		}

		// Second: Handle user adds (A) for this category
		for _, addDomain := range aSet {
			cleanVal := strings.ToLower(strings.TrimSpace(addDomain))

			// Check if already claimed
			if owner, exists := claimed[cleanVal]; exists {
				if owner == cat {
					// Same category: already present from upstream (U), user add is redundant
					fmt.Printf("    └─ [%s] user add '%s' already present from upstream, skipped\n", cat, cleanVal)
				} else {
					// Different category: cross-category interception
					fmt.Printf("    └─ [%s] user add '%s' intercepted by [%s] (priority), dropped\n", cat, cleanVal, owner)
				}
				continue
			}
			// Check if this category's remove has it
			if rSet != nil && rSet[cleanVal] {
				continue
			}
			// Claim it
			claimed[cleanVal] = cat
			if finalDomains[cat] == nil {
				finalDomains[cat] = []*routercommon.Domain{}
			}
			finalDomains[cat] = append(finalDomains[cat], &routercommon.Domain{
				Type:  routercommon.Domain_RootDomain,
				Value: cleanVal,
			})
		}

		// Process removes: release ownership for fallback to lower categories
		for domain := range rSet {
			if claimed[domain] == cat {
				delete(claimed, domain) // Allow fallback
			}
			// If owned by higher priority, keep it (higher priority wins)
		}
	}

	// Build final GeoSiteList
	newList := &routercommon.GeoSiteList{Entry: make([]*routercommon.GeoSite, 0, len(ctx.Config.GeositeStrategies))}
	for _, strat := range ctx.Config.GeositeStrategies {
		domains := finalDomains[strat.Name]
		final := deduplicateDomains(domains)
		newList.Entry = append(newList.Entry, &routercommon.GeoSite{
			CountryCode: strings.ToUpper(strat.Name),
			Domain:      final,
		})
		fmt.Printf("    └─ Compiled strategy '%s' (%d domains)\n", strat.Name, len(final))
	}

	return newList, nil
}

func deduplicateDomains(domains []*routercommon.Domain) []*routercommon.Domain {
	unique := make([]*routercommon.Domain, 0, len(domains))

	 // Composite key: (Type, Value) — both must match to be a duplicate
	type domainKey struct {
		Type  routercommon.Domain_Type // enum: Plain=0, Regex=1, Full=2, RootDomain=3
		Value string				   // normalized: lowercase, trimmed
	}
	seen := make(map[domainKey]struct{})

	for _, d := range domains {
		// MUTATES the proto in place: "  GooGle.Com  " → "google.com"
		d.Value = strings.ToLower(strings.TrimSpace(d.Value))
		key := domainKey{Type: d.Type, Value: d.Value}
		if _, exists := seen[key]; !exists {
			seen[key] = struct{}{}
			unique = append(unique, d)	// Keeps the SAME pointer (now mutated)
		}
		// else: duplicate — pointer is dropped, original proto becomes unreachable
		// when export to singbox format, it will do the optimization by keeping root domain only.
	}
	return unique
}