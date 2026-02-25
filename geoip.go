package main

import (
	"net"
	"strings"

	"github.com/oschwald/geoip2-golang"
	"github.com/oschwald/maxminddb-golang"
)

func scanGeoIP(path string, targets []string) (*GeoIPResult, error) {
	db, err := maxminddb.Open(path)
	if err != nil {
		return nil, err
	}
	defer db.Close()

	targetSet := make(map[string]bool)
	for _, t := range targets {
		targetSet[strings.ToLower(t)] = true
	}

	countryMap := make(map[string][]*net.IPNet)
	networks := db.Networks(maxminddb.SkipAliasedNetworks)

	var record geoip2.Enterprise
	for networks.Next() {
		subnet, err := networks.Network(&record)
		if err != nil {
			return nil, err
		}

		code := getBestCode(record)
		if code != "" && targetSet[code] {
			countryMap[code] = append(countryMap[code], subnet)
		}
	}

	return &GeoIPResult{
		Metadata:   db.Metadata,
		CountryMap: countryMap,
	}, networks.Err()
}

// getBestCode implements "the guy's" robust fallback logic
func getBestCode(record geoip2.Enterprise) string {
	if record.Country.IsoCode != "" {
		return strings.ToLower(record.Country.IsoCode)
	}
	if record.RegisteredCountry.IsoCode != "" {
		return strings.ToLower(record.RegisteredCountry.IsoCode)
	}
	if record.RepresentedCountry.IsoCode != "" {
		return strings.ToLower(record.RepresentedCountry.IsoCode)
	}
	if record.Continent.Code != "" {
		return strings.ToLower(record.Continent.Code)
	}
	return ""
}


func countTotalIPs(result *GeoIPResult) int {
	count := 0
	for _, nets := range result.CountryMap {
		count += len(nets)
	}
	return count
}