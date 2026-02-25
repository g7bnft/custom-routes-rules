package main

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"github.com/sagernet/sing-box/common/geosite"
	"github.com/sagernet/sing-box/common/srs"
	"github.com/sagernet/sing-box/option"
	C "github.com/sagernet/sing-box/constant"
)

func writeGeositeDB(f *os.File, m map[string][]geosite.Item) error {
	bw := bufio.NewWriter(f)
	if err := geosite.Write(bw, m); err != nil {
		return err
	}
	return bw.Flush()
}

func writeRuleSets(dir string, domainMap map[string][]geosite.Item) error {

	for code, items := range domainMap {
		optimizedItems := combItems(items)
		compiled := geosite.Compile(optimizedItems)
		plainRuleSet := option.PlainRuleSet{
			Rules: []option.HeadlessRule{{
				Type: C.RuleTypeDefault,
				DefaultOptions: option.DefaultHeadlessRule{
					Domain:        compiled.Domain,
					DomainSuffix:  compiled.DomainSuffix,
					DomainKeyword: compiled.DomainKeyword,
					DomainRegex:   compiled.DomainRegex,
				},
			}},
		}

		// SRS with version 1 and buffering
		sf, _ := os.Create(filepath.Join(dir, "geosite-"+code+".srs"))
		sbw := bufio.NewWriter(sf)
		srs.Write(sbw, plainRuleSet, 1)
		sbw.Flush()
		sf.Close()

		// JSON
		jf, _ := os.Create(filepath.Join(dir, "geosite-"+code+".json"))
		enc := json.NewEncoder(jf)
		enc.SetIndent("", "  ")
		enc.Encode(plainRuleSet)
		jf.Close()

		// 3. Slim Plain TXT (.domain format)
		tf, _ := os.Create(filepath.Join(dir, "geosite-"+code+".txt"))
		tbw := bufio.NewWriter(tf)
		for _, item := range optimizedItems {
            // Only output Suffix types to avoid doubling the file size
            if item.Type == geosite.RuleTypeDomainSuffix {
                // Ensure it starts with exactly one dot
                val := strings.TrimPrefix(item.Value, ".")
                tbw.WriteString("." + val + "\n")
            }
		}
		tbw.Flush()
		tf.Close()
	}
	return nil
}

func loadList(path string) []string {
	file, err := os.Open(path)
	if err != nil { return nil }
	defer file.Close()
	var clean []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") { continue }
		clean = append(clean, strings.ToLower(line))
	}
	return clean
}