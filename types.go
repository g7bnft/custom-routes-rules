package main

import (
	"net"
	"github.com/v2fly/v2ray-core/v5/app/router/routercommon"
	"github.com/oschwald/maxminddb-golang"
)

type domainKey struct {
	Type  routercommon.Domain_Type
	Value string
}

type configMods struct {
	adds map[string][]string
	rms  map[string]map[string]bool
}

// Add this to your existing types.go
type GeoIPResult struct {
	Metadata   maxminddb.Metadata
	CountryMap map[string][]*net.IPNet
}

