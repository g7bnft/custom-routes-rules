package strategy

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"custom-rules/internal/types"
)

// StrategyStage executes the conflict resolution logic.
type StrategyStage struct{}

func NewStrategyStage() *StrategyStage {
	return &StrategyStage{}
}

func (s *StrategyStage) Name() string { return "strategy" }

func (s *StrategyStage) Execute(ctx *types.BuildContext) error {
	cfg := ctx.Config

	mods := &types.ConfigMods{
		Adds: make(map[string][]string),
		Rms:  make(map[string]map[string]bool),
	}

	categories := make([]string, 0, len(cfg.GeositeStrategies))

	for _, k := range cfg.GeositeStrategies {
		categories = append(categories, k.Name)

		addPath := filepath.Join(cfg.UserStrategyDir, k.Name+".txt")
		if cleanedDomains, err := loadList(addPath); err == nil {
			mods.Adds[k.Name] = cleanedDomains
		} else {
			fmt.Printf("  [Strategy] couldn't load %s: %v\n", addPath, err)
		}

		rmPath := filepath.Join(cfg.UserStrategyDir, k.Name+"-need-to-remove.txt")
		if cleanedDomains, err := loadList(rmPath); err == nil {
			rmMap := make(map[string]bool)
			for _, domain := range cleanedDomains {
				rmMap[domain] = true
			}
			mods.Rms[k.Name] = rmMap
		} else {
			fmt.Printf("  [Strategy] couldn't load %s: %v\n", rmPath, err)
		}

		fmt.Printf("  [Strategy] %s: %d adds, %d removals loaded\n", k.Name, len(mods.Adds[k.Name]), len(mods.Rms[k.Name]))
	}

	// Stage 1: Deduplicate adds within each category (user may have duplicates)
	deduplicateAdds(mods, categories)

	// Stage 2: Check user data contradictions (intra-category: add vs remove)
	checkUserContradictions(mods, categories)

	// Stage 3: Apply Priority Cascade (cross-category: reject > proxy > direct)
	applyPriorityCascade(mods, categories)

	// Stage 4: Write back cleaned adds to user files
	writeBackUserFiles(mods, categories, cfg.UserStrategyDir)

	summary := make([]string, 0, len(categories))
	for _, cat := range categories {
		summary = append(summary, fmt.Sprintf("[%s: %d(+)/%d(-)]", cat, len(mods.Adds[cat]), len(mods.Rms[cat])))
	}
	fmt.Println("  [Strategy] Final Strategy: " + strings.Join(summary, " "))

	ctx.Mods = mods
	return nil
}

// checkUserContradictions handles intra-category conflicts:
// if a domain is in both Adds[cat] and Rms[cat] for the same category,
// it's a user configuration error. Remove wins, domain is dropped from adds
// with a warning.
func checkUserContradictions(mods *types.ConfigMods, categories []string) {
	for _, cat := range categories {
		rm := mods.Rms[cat]
		if rm == nil {
			continue
		}

		var cleanAdds []string
		for _, domain := range mods.Adds[cat] {
			if rm[domain] {
				fmt.Printf("  [Strategy] WARNING: %s '%s' is in both add and remove — remove wins, dropped from add\n", cat, domain)
				continue
			}
			cleanAdds = append(cleanAdds, domain)
		}
		mods.Adds[cat] = cleanAdds
	}
}

// applyPriorityCascade implements the cross-category priority waterfall:
// Categories are processed in order (reject > proxy > direct).
// - Category A (Forced Route): Domain in Adds[cat] claims it for this category,
//   stripping from all lower categories' adds.
// - Category B (Excavation/Fallback): Domain in Rms[cat] is evicted from this
//   category, allowing fallback to lower categories if they have it in their adds.
// - Category C (Direct Entry): Domain not claimed by higher categories, but in
//   this category's adds, securely populates this category.
// - Category D (Void): Domain caught by remove list with no fallback/add in
//   subsequent routes is completely pruned.
func applyPriorityCascade(mods *types.ConfigMods, categories []string) {
	// finalOwners tracks which category ultimately owns each domain
	finalOwners := make(map[string]string) // domain -> owning category name

	for _, cat := range categories {
		adds := mods.Adds[cat]
		rms := mods.Rms[cat]

		// Process adds for this category
		var cleanAdds []string
		for _, domain := range adds {
			// Check if already owned by higher priority category
			if owner, exists := finalOwners[domain]; exists {
				// Category A: higher priority owns it
				fmt.Printf("  [Strategy] %s '%s' intercepted by %s (priority), dropped\n", cat, domain, owner)
				continue
			}
			// Check if this category's own remove list has it
			// (intra-category contradiction already handled by checkUserContradictions,
			// but defense in depth)
			if rms != nil && rms[domain] {
				// This category's remove wins - don't add, allow fallback
				continue
			}
			// This category claims it
			finalOwners[domain] = cat
			cleanAdds = append(cleanAdds, domain)
		}
		mods.Adds[cat] = cleanAdds

		// Process removes: these domains are EXPLICITLY REMOVED from this category
		// If this category owned them, release ownership (allow fallback to lower)
		// If higher category owned them, higher category still owns them (already processed)
		for domain := range rms {
			if finalOwners[domain] == cat {
				delete(finalOwners, domain) // Release for fallback
			}
			// If owned by higher priority, keep it (higher priority wins)
		}
	}

	// After cascade, all categories' Adds are cleaned.
	// Removes stay as-is for Geosite Patch Stage to apply against upstream.
}

// writeBackUserFiles writes the post-cascade adds back to user strategy files.
func writeBackUserFiles(mods *types.ConfigMods, categories []string, userStrategyDir string) {
	for _, cat := range categories {
		addPath := filepath.Join(userStrategyDir, cat+".txt")
		adds := mods.Adds[cat]

		// Write cleaned adds back to file
		var lines []string
		for _, domain := range adds {
			lines = append(lines, domain)
		}
		content := strings.Join(lines, "\n")
		if content != "" {
			content += "\n" // trailing newline
		}

		if err := os.WriteFile(addPath, []byte(content), 0644); err != nil {
			fmt.Printf("  [Strategy] WARNING: failed to write back %s: %v\n", addPath, err)
			continue
		}
		fmt.Printf("  [Strategy] Wrote back %d domains to %s\n", len(adds), addPath)
	}
}

// deduplicateAdds removes duplicate domains within each category's add list.
func deduplicateAdds(mods *types.ConfigMods, categories []string) {
	for _, cat := range categories {
		seen := make(map[string]bool)
		var unique []string
		for _, domain := range mods.Adds[cat] {
			if !seen[domain] {
				seen[domain] = true
				unique = append(unique, domain)
			} else {
				fmt.Printf("  [Strategy] WARNING: %s duplicate add '%s' removed\n", cat, domain)
			}
		}
		mods.Adds[cat] = unique
	}
}

func loadList(path string) ([]string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err // Return the file open error
	}
	defer file.Close()

	var clean []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.ToLower(strings.TrimSpace(scanner.Text()))
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "//") {
			continue
		}

		clean = append(clean, line)
	}
	// Check for scanning errors after the loop ends
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("failed scanning %s: %w", path, err)
	}

	return clean, nil
}