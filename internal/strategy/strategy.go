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

	reconcileConflicts(mods, categories)

	summary := make([]string, 0, len(categories))
	for _, cat := range categories {
		summary = append(summary, fmt.Sprintf("[%s: %d(+)/%d(-)]", cat, len(mods.Adds[cat]), len(mods.Rms[cat])))
	}
	fmt.Println("  [Strategy] Final Strategy: " + strings.Join(summary, " "))

	ctx.Mods = mods
	return nil
}

// reconcileConflicts resolves the one real conflict that can occur:
// the same domain listed in both a category's add and remove file.
// Remove wins — if you've explicitly marked a domain for exclusion,
// an add entry for the same domain is treated as stale/contradictory
// and dropped, with a warning so you can clean up the source file.
func reconcileConflicts(mods *types.ConfigMods, categories []string) {
	for _, cat := range categories {
		rm := mods.Rms[cat]
		if rm == nil {
			continue
		}

		var cleanAdds []string
		for _, domain := range mods.Adds[cat] {
			if rm[domain] {
				fmt.Printf("  [Strategy] %s '%s' is in both add and remove — remove wins, dropped from add\n", cat, domain)
				continue
			}
			cleanAdds = append(cleanAdds, domain)
		}
		mods.Adds[cat] = cleanAdds
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