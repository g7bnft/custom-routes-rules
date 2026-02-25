package main

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/v2fly/v2ray-core/v5/app/router/routercommon"
	"google.golang.org/protobuf/testing/protocmp"
)

func TestApplyModifications_Removal(t *testing.T) {
	// 1. Setup our "Want to Remove" list (rmMap)
	mods := &configMods{
		rms: map[string]map[string]bool{
			"reject": {
				"paypal.com":  true,
				"gstatic.com": true,
			},
		},
	}

	// 2. Define the test table
	tests := []struct {
		name     string
		tag      string
		input    []string
		expected []string // What should remain after filtering
	}{
		{
			name: "Remove exact match",
			tag:  "reject",
			input: []string{"paypal.com"},
			expected: []string{},
		},
		{
			name: "Remove subdomain (The fix we need!)",
			tag:  "reject",
			input: []string{"i.paypal.com", "stats.paypal.com", "data.account.paypal.com"},
			expected: []string{},
		},
		{
			name: "Keep similar but different roots",
			tag:  "reject",
			input: []string{"notpaypal.com", "paypal.com.net", "my-gstatic.com"},
			expected: []string{"notpaypal.com", "paypal.com.net", "my-gstatic.com"},
		},
		{
			name: "Mixed removal in gstatic",
			tag:  "reject",
			input: []string{"fonts.gstatic.com", "google.com"},
			expected: []string{"google.com"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Convert input strings to routercommon.Domain objects
			entry := &routercommon.GeoSite{
				CountryCode: tt.tag,
				Domain:      []*routercommon.Domain{},
			}
			for _, d := range tt.input {
				entry.Domain = append(entry.Domain, &routercommon.Domain{
					Type:  routercommon.Domain_RootDomain,
					Value: d,
				})
			}

			// Run the logic
			applyModifications(entry, tt.tag, mods)

			// Prepare expected result
			expectedEntry := &routercommon.GeoSite{
				CountryCode: tt.tag,
				Domain:      []*routercommon.Domain{},
			}
			for _, d := range tt.expected {
				expectedEntry.Domain = append(expectedEntry.Domain, &routercommon.Domain{
					Type:  routercommon.Domain_RootDomain,
					Value: d,
				})
			}

			// Compare using protocmp.Transform
			if diff := cmp.Diff(expectedEntry, entry, protocmp.Transform()); diff != "" {
				t.Errorf("applyModifications() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}