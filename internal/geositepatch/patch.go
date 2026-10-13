package geositepatch

import (
	"fmt"
	"os"
	"strings"

	"custom-rules/internal/types"

	"github.com/v2fly/v2ray-core/v5/app/router/routercommon"
	"google.golang.org/protobuf/proto"
)

// domainKey enforces structural uniqueness: both Type and Value must match.
// Used for claimed tracking and deduplication across the package.
type domainKey struct {
	Type  routercommon.Domain_Type
	Value string
}

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
	claimed := make(map[domainKey]string) // domainKey -> owning category

	// Helper to check if a domain value is claimed by ANY category (returns owning category)
	// Cascade priority is at VALUE level: if higher priority owns the domain value, lower cannot claim it
	checkClaimedValue := func(val string) (string, bool) {
		for _, t := range []routercommon.Domain_Type{
			routercommon.Domain_Plain,
			routercommon.Domain_Regex,
			routercommon.Domain_Full,
			routercommon.Domain_RootDomain,
		} {
			if owner, exists := claimed[domainKey{Type: t, Value: val}]; exists {
				return owner, true
			}
		}
		return "", false
	}

	// Helper to check if a specific (type, value) is claimed
	checkClaimedExact := func(val string, typ routercommon.Domain_Type) (string, bool) {
		if owner, exists := claimed[domainKey{Type: typ, Value: val}]; exists {
			return owner, true
		}
		return "", false
	}

	// Helper to release all type variants of a domain for a category
	releaseClaim := func(val string, cat string) {
		for _, t := range []routercommon.Domain_Type{
			routercommon.Domain_Plain,
			routercommon.Domain_Regex,
			routercommon.Domain_Full,
			routercommon.Domain_RootDomain,
		} {
			key := domainKey{Type: t, Value: val}
			if claimed[key] == cat {
				delete(claimed, key)
			}
		}
	}

	// Helper to claim a domain with a specific type
	claim := func(val string, cat string, typ routercommon.Domain_Type) {
		claimed[domainKey{Type: typ, Value: val}] = cat
	}

	for _, strat := range ctx.Config.GeositeStrategies {
		cat := strat.Name
		uSet := upstreamDomains[cat]  // Upstream domains for this category
		aSet := ctx.Mods.Adds[cat]    // User adds (pre-cascaded from Strategy Stage)
		rSet := ctx.Mods.Rms[cat]     // User removes

		// First: Handle upstream domains (U) for this category
		for _, dom := range uSet {
			cleanVal := strings.ToLower(strings.TrimSpace(dom.Value))

			// Check if this domain VALUE is already claimed by higher priority (cascade: value-level)
			if owner, exists := checkClaimedValue(cleanVal); exists {
				fmt.Printf("    └─ [%s] upstream '%s' skipped — already claimed by [%s] (priority)\n", cat, cleanVal, owner)
				continue
			}
			// Check if this exact (type, value) is already in THIS category (structural duplicate)
			if owner, exists := checkClaimedExact(cleanVal, dom.Type); exists && owner == cat {
				fmt.Printf("    └─ [%s] upstream '%s' (type=%d) skipped — structural duplicate in same category\n", cat, cleanVal, dom.Type)
				continue
			}
			// Check if this category's remove list has it (Category B: evict)
			if rSet != nil && rSet[cleanVal] {
				// Evicted from this category — allow fallback to lower
				continue
			}
			// Category A: Claim it with its original type
			claim(cleanVal, cat, dom.Type)
			if finalDomains[cat] == nil {
				finalDomains[cat] = []*routercommon.Domain{}
			}
			finalDomains[cat] = append(finalDomains[cat], dom)
		}

		// Second: Handle user adds (A) for this category
		for _, addDomain := range aSet {
			cleanVal := strings.ToLower(strings.TrimSpace(addDomain))

			// Check if this domain VALUE is already claimed by higher priority (cascade: value-level)
			if owner, exists := checkClaimedValue(cleanVal); exists {
				fmt.Printf("    └─ [%s] user add '%s' intercepted by [%s] (priority), dropped\n", cat, cleanVal, owner)
				continue
			}
			// Check if this exact (type, value) is already in THIS category (structural duplicate)
			addType := routercommon.Domain_RootDomain
			if owner, exists := checkClaimedExact(cleanVal, addType); exists && owner == cat {
				fmt.Printf("    └─ [%s] user add '%s' already present from upstream, skipped\n", cat, cleanVal)
				continue
			}
			// Check if this category's remove has it
			if rSet != nil && rSet[cleanVal] {
				continue
			}
			// Claim it as RootDomain
			claim(cleanVal, cat, addType)
			if finalDomains[cat] == nil {
				finalDomains[cat] = []*routercommon.Domain{}
			}
			finalDomains[cat] = append(finalDomains[cat], &routercommon.Domain{
				Type:  addType,
				Value: cleanVal,
			})
		}

		// Process removes: release ownership for fallback to lower categories
		for domain := range rSet {
			// Remove applies to all type variants of this domain
			releaseClaim(domain, cat)
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

	seen := make(map[domainKey]struct{})

	for _, d := range domains {
		// Normalize without mutating original proto
		normalized := strings.ToLower(strings.TrimSpace(d.Value))
		key := domainKey{Type: d.Type, Value: normalized}
		if _, exists := seen[key]; !exists {
			seen[key] = struct{}{}
			// Create a copy with normalized value to avoid mutating upstream proto
			unique = append(unique, &routercommon.Domain{
				Type:      d.Type,
				Value:     normalized,
				Attribute: d.Attribute, // Preserve attributes if any
			})
		}
		// else: duplicate — dropped
	}
	return unique
}