package main

import (
	"encoding/json"
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
)

// buildSingboxDB creates the .db file with the specific "sing-geoip" signature
func buildSingboxDB(result *GeoIPResult, outputPath string) error {
	writer, err := mmdbwriter.New(mmdbwriter.Options{
		DatabaseType:            "sing-geoip",
		Languages:               geoipTags,
		IPVersion:               int(result.Metadata.IPVersion),
		RecordSize:              int(result.Metadata.RecordSize),
		Inserter:                inserter.ReplaceWith,
		DisableIPv4Aliasing:     true, // Essential for routing consistency
		IncludeReservedNetworks: true, // Crucial for the "private" tag
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

// buildXrayDAT creates the legacy .dat file for Xray/V2Ray
func buildXrayDAT(result *GeoIPResult, outputPath string) error {
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

// exportIPRuleSets generates individual .srs, .json, and .txt files for each tag
func exportIPRuleSets(result *GeoIPResult, outputDir string) error {
	for code, nets := range result.CountryMap {
		cidrs := make([]string, 0, len(nets))
		for _, n := range nets {
			cidrs = append(cidrs, n.String())
		}
		sort.Strings(cidrs)

		// Create sing-box rule-set structure
		ruleSet := option.PlainRuleSet{
			Rules: []option.HeadlessRule{{
				Type: C.RuleTypeDefault,
				DefaultOptions: option.DefaultHeadlessRule{
					IPCIDR: cidrs,
				},
			}},
		}

		// 1. Binary Rule-Set (.srs)
		sf, _ := os.Create(filepath.Join(outputDir, "geoip-"+code+".srs"))
		srs.Write(sf, ruleSet, 1)
		sf.Close()

		// 2. JSON Rule-Set (.json)
		jf, _ := os.Create(filepath.Join(outputDir, "geoip-"+code+".json"))
		enc := json.NewEncoder(jf)
		enc.SetIndent("", "  ")
		enc.Encode(ruleSet)
		jf.Close()

		// 3. Plain Text (.txt)
		tf, _ := os.Create(filepath.Join(outputDir, "geoip-"+code+".txt"))
		for _, c := range cidrs {
			tf.WriteString(c + "\n")
		}
		tf.Close()
	}
	return nil
}