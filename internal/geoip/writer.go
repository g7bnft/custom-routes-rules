package geoip

import (
	// "encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/maxmind/mmdbwriter"
	"github.com/maxmind/mmdbwriter/inserter"
	"github.com/maxmind/mmdbwriter/mmdbtype"
	"github.com/sagernet/sing-box/common/srs"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/option"
	"github.com/v2fly/v2ray-core/v5/app/router/routercommon"
	"google.golang.org/protobuf/proto"

	"custom-rules/internal/types"
)

func buildSingboxDB(result *types.GeoIPResult, languages []string, outputPath string) error {
	writer, err := mmdbwriter.New(mmdbwriter.Options{
		DatabaseType:            "sing-geoip",
		Languages:               languages,
		IPVersion:               int(result.Metadata.IPVersion),
		RecordSize:              int(result.Metadata.RecordSize),
		Inserter:                inserter.ReplaceWith,
		DisableIPv4Aliasing:     true,
		IncludeReservedNetworks: true,
	})
	if err != nil {
		return err
	}

	for code, nets := range result.CountryMap {
		for _, n := range nets {
			writer.Insert(n, mmdbtype.String(code))
		}
	}

	f, err := os.Create(outputPath)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = writer.WriteTo(f)
	return err
}

func buildXrayDAT(result *types.GeoIPResult, outputPath string) error {
	geoipList := &routercommon.GeoIPList{}

	for code, nets := range result.CountryMap {
		vIPs := make([]*routercommon.CIDR, 0, len(nets))
		for _, n := range nets {
			ones, _ := n.Mask.Size()
			vIPs = append(vIPs, &routercommon.CIDR{
				Ip:     n.IP,
				Prefix: uint32(ones),
			})
		}
		geoipList.Entry = append(geoipList.Entry, &routercommon.GeoIP{
			CountryCode: strings.ToUpper(code),
			Cidr:        vIPs,
		})
	}

	protoData, err := proto.Marshal(geoipList)
	if err != nil {
		return err
	}

	return os.WriteFile(outputPath, protoData, 0644)
}

func exportIPRuleSets(result *types.GeoIPResult, outputDir string) error {
	for code, nets := range result.CountryMap {
		cidrs := make([]string, 0, len(nets))
		for _, n := range nets {
			cidrs = append(cidrs, n.String())
		}
		sort.Strings(cidrs)

		ruleSet := option.PlainRuleSet{
			Rules: []option.HeadlessRule{{
				Type: C.RuleTypeDefault,
				DefaultOptions: option.DefaultHeadlessRule{
					IPCIDR: cidrs,
				},
			}},
		}

		sf, _ := os.Create(filepath.Join(outputDir, "geoip-"+code+".srs"))
		srs.Write(sf, ruleSet, 1)
		sf.Close()

		// jf, _ := os.Create(filepath.Join(outputDir, "geoip-"+code+".json"))
		// enc := json.NewEncoder(jf)
		// enc.SetIndent("", " ")
		// enc.Encode(ruleSet)
		// jf.Close()

		tf, _ := os.Create(filepath.Join(outputDir, "geoip-"+code+".txt"))
		for _, c := range cidrs {
			tf.WriteString(c + "\n")
		}
		tf.Close()
	}

	return nil
}