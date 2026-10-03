package geosite

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	singeosite "github.com/sagernet/sing-box/common/geosite"
	"github.com/sagernet/sing-box/common/srs"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/option"
)

func writeGeositeDB(f *os.File, m map[string][]singeosite.Item) error {
	bw := bufio.NewWriter(f)
	if err := singeosite.Write(bw, m); err != nil {
		return err
	}
	return bw.Flush()
}

func writeRuleSets(dir string, domainMap map[string][]singeosite.Item) error {
	for code, items := range domainMap {
		optimizedItems := combItems(items)
		compiled := singeosite.Compile(optimizedItems)

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

		sf, _ := os.Create(filepath.Join(dir, "geosite-"+code+".srs"))
		sbw := bufio.NewWriter(sf)
		srs.Write(sbw, plainRuleSet, 1)
		sbw.Flush()
		sf.Close()

		jf, _ := os.Create(filepath.Join(dir, "geosite-"+code+".json"))
		enc := json.NewEncoder(jf)
		enc.SetIndent("", " ")
		enc.Encode(plainRuleSet)
		jf.Close()

		tf, _ := os.Create(filepath.Join(dir, "geosite-"+code+".txt"))
		tbw := bufio.NewWriter(tf)
		for _, item := range optimizedItems {
			if item.Type == singeosite.RuleTypeDomainSuffix {
				val := strings.TrimPrefix(item.Value, ".")
				tbw.WriteString("." + val + "\n")
			}
		}
		tbw.Flush()
		tf.Close()
	}

	return nil
}
