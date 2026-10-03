package strategy

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"custom-rules/internal/types"
)

var categories = []string{"reject", "proxy", "direct"}

// StrategyStage loads the strategy/*.txt add and remove lists and
// reconciles any conflicts between categories.
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

	for _, k := range categories {
		mods.Adds[k] = loadList(filepath.Join(cfg.StrategyDir, k+".txt"))

		rmMap := make(map[string]bool)
		for _, item := range loadList(filepath.Join(cfg.StrategyDir, k+"-need-to-remove.txt")) {
			cleanItem := strings.ToLower(strings.TrimSpace(item))
			if cleanItem != "" {
				rmMap[cleanItem] = true
			}
		}
		mods.Rms[k] = rmMap

		fmt.Printf(" └─ Module [%s]: %d adds, %d removals loaded\n", k, len(mods.Adds[k]), len(mods.Rms[k]))
	}

	reconcileConflicts(mods)

	summary := []string{}
	for _, k := range categories {
		summary = append(summary, fmt.Sprintf("[%s: %d(+)/%d(-)]", k, len(mods.Adds[k]), len(mods.Rms[k])))
	}
	fmt.Println(" └─ Final Strategy: " + strings.Join(summary, " "))

	ctx.Mods = mods
	return nil
}

func loadList(path string) []string {
	file, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer file.Close()

	var clean []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		clean = append(clean, strings.ToLower(line))
	}
	return clean
}

func reconcileConflicts(mods *types.ConfigMods) {
	claimed := make(map[string]string)

	for _, currentCat := range categories {
		for _, domain := range mods.Adds[currentCat] {
			if owner, exists := claimed[domain]; exists {
				fmt.Printf(" ⚠️ [Conflict] '%s' in %s is IGNORED (already claimed by %s)\n", domain, currentCat, owner)
				continue
			}
			claimed[domain] = currentCat

			if mods.Rms[currentCat][domain] {
				fmt.Printf(" 🔧 [Self-Clean] Removed '%s' from %s-need-to-remove\n", domain, currentCat)
				delete(mods.Rms[currentCat], domain)
			}

			for _, otherCat := range categories {
				if currentCat == otherCat {
					continue
				}
				if !mods.Rms[otherCat][domain] {
					fmt.Printf(" 🔗 [Priority] Forced '%s' into %s-need-to-remove (overridden by %s)\n", domain, otherCat, currentCat)
					mods.Rms[otherCat][domain] = true
				}
			}
		}
	}
}
