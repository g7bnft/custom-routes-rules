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

	// Phase 1: cross-category claim resolution on RAW upstream data only
	// — no user mods involved yet. Priority = GeositeStrategies order
	// (reject, proxy, direct). First strategy to see a domain keeps it.
	claimed := make(map[string]string)
	type claimedSet struct {
		strategy string
		domains []*routercommon.Domain
	}
	claimedSets := make([]claimedSet, 0, len(ctx.Config.GeositeStrategies))

	// collect domains by strategy via flatten grouped tags.
	for _, strat := range ctx.Config.GeositeStrategies {
		var raw []*routercommon.Domain
		for _, tag := range strat.Tags {
			if upSite, exists := upstreamMap[strings.ToLower(tag)]; exists {
				raw = append(raw, upSite.Domain...)
			}
		}

		var kept []*routercommon.Domain
		for _, dom := range raw {
			cleanVal := strings.ToLower(strings.TrimSpace(dom.Value))
			if owner, exists := claimed[cleanVal]; exists {
				fmt.Printf("    └─ [%s] '%s' skipped — already claimed by [%s] upstream\n", strat.Name, cleanVal, owner)
				continue
			}
			claimed[cleanVal] = strat.Name
			kept = append(kept, dom)
		}

		claimedSets = append(claimedSets, claimedSet{strategy: strat.Name, domains: kept})
	}

	// Phase 2: user Rms/Adds apply per-category, AFTER claiming, with
	// no further cross-category check — user's explicit config has the
	// final word, even if it creates duplicates across categories.
	// That's on the user to resolve, not this function's job to prevent.
	newList := &routercommon.GeoSiteList{
		Entry: make([]*routercommon.GeoSite, 0, len(claimedSets)),
	}

	for _, cs := range claimedSets {
		rms := ctx.Mods.Rms[cs.strategy]
		adds := ctx.Mods.Adds[cs.strategy]

		var filtered []*routercommon.Domain
		removedCount := 0
		for _, dom := range cs.domains {
			// rms not configured or rms configured but domain not found in rms
			// allow to add to keep(filtered).
			if rms != nil && rms[dom.Value] {
				removedCount++
				continue
			}
			filtered = append(filtered, dom)
		}

		for _, addDomain := range ctx.Mods.Adds[cs.strategy] {
			filtered = append(filtered, &routercommon.Domain{
				Type:  routercommon.Domain_RootDomain,
				Value: strings.ToLower(strings.TrimSpace(addDomain)),
			})
		}

		final := deduplicateDomains(filtered)

		if removedCount > 0 {
			fmt.Printf("    └─ [%s] removed %d domain(s) via user remove-list\n", cs.strategy, removedCount)
		}
		if len(adds) > 0 {
			fmt.Printf("    └─ [%s] added %d domain(s) via user add-list\n", cs.strategy, len(adds))
		}

		newList.Entry = append(newList.Entry, &routercommon.GeoSite{
			CountryCode: strings.ToUpper(cs.strategy),
			Domain:      final,
		})

		fmt.Printf("    └─ Compiled strategy '%s' (%d domains)\n", cs.strategy, len(final))
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