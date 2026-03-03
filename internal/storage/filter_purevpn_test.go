package storage

import (
	"testing"

	"github.com/qdm12/gluetun/internal/configuration/settings"
	"github.com/qdm12/gluetun/internal/constants/providers"
	"github.com/qdm12/gluetun/internal/constants/vpn"
	"github.com/qdm12/gluetun/internal/models"
	"github.com/stretchr/testify/assert"
)

func Test_filterByPureVPNServerTypes_MultiTraitHost(t *testing.T) {
	t.Parallel()

	server := models.Server{
		PortForward:      true,
		QuantumResistant: true,
		Obfuscated:       true,
		Categories:       []string{"p2p"},
	}

	testCases := map[string]struct {
		serverTypes []string
		filtered    bool
	}{
		"empty server types filters obfuscated host": {
			serverTypes: nil,
			filtered:    true,
		},
		"obfuscation keeps multi-trait host": {
			serverTypes: []string{"obfuscation"},
			filtered:    false,
		},
		"portforwarding filters obfuscated host without explicit obfuscation": {
			serverTypes: []string{"portforwarding"},
			filtered:    true,
		},
		"portforwarding and obfuscation keeps host": {
			serverTypes: []string{"portforwarding", "obfuscation"},
			filtered:    false,
		},
		"all matching types keep host": {
			serverTypes: []string{"portforwarding", "quantumresistant", "obfuscation", "p2p"},
			filtered:    false,
		},
	}

	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			filtered := filterByPureVPNServerTypes(server, testCase.serverTypes)
			assert.Equal(t, testCase.filtered, filtered)
		})
	}
}

func Test_filterByPureVPNServerTypes_NonObfuscatedMultiTraitHost(t *testing.T) {
	t.Parallel()

	server := models.Server{
		PortForward:      true,
		QuantumResistant: true,
		Obfuscated:       false,
		Categories:       []string{"p2p"},
	}

	testCases := map[string]struct {
		serverTypes []string
		filtered    bool
	}{
		"empty keeps non-obfuscated host": {
			serverTypes: nil,
			filtered:    false,
		},
		"portforwarding keeps non-obfuscated host": {
			serverTypes: []string{"portforwarding"},
			filtered:    false,
		},
		"quantumresistant and p2p keeps host": {
			serverTypes: []string{"quantumresistant", "p2p"},
			filtered:    false,
		},
		"regular filters non-obfuscated multi-trait host": {
			serverTypes: []string{"regular"},
			filtered:    true,
		},
	}

	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			filtered := filterByPureVPNServerTypes(server, testCase.serverTypes)
			assert.Equal(t, testCase.filtered, filtered)
		})
	}
}

func Test_filterServer_PureVPNServerTypesAndHostname(t *testing.T) {
	t.Parallel()

	server := models.Server{
		VPN:              vpn.OpenVPN,
		Hostname:         "usny2-auto-udp-obf.ptoserver.com",
		PortForward:      true,
		QuantumResistant: true,
		Obfuscated:       true,
		Categories:       []string{"p2p"},
		UDP:              true,
	}

	testCases := map[string]struct {
		serverTypes []string
		filtered    bool
	}{
		"hostname match with obfuscation type": {
			serverTypes: []string{"obfuscation"},
			filtered:    false,
		},
		"hostname match with portforwarding and obfuscation types": {
			serverTypes: []string{"portforwarding", "obfuscation"},
			filtered:    false,
		},
		"hostname match with p2p type only": {
			serverTypes: []string{"p2p"},
			filtered:    true,
		},
	}

	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			selection := settings.ServerSelection{
				PureVPNServerTypes: testCase.serverTypes,
				Hostnames:          []string{server.Hostname},
			}.WithDefaults(providers.Purevpn)
			filtered := filterServer(server, selection)
			assert.Equal(t, testCase.filtered, filtered)
		})
	}
}

func Test_filterServer_CountryAndCity(t *testing.T) {
	t.Parallel()

	server := models.Server{
		VPN:     vpn.OpenVPN,
		Country: "United States",
		City:    "New York",
		UDP:     true,
	}

	testCases := map[string]struct {
		countries []string
		cities    []string
		filtered  bool
	}{
		"country match and city match": {
			countries: []string{"United States", "Canada"},
			cities:    []string{"New York"},
			filtered:  false,
		},
		"country mismatch despite city list": {
			countries: []string{"Canada"},
			cities:    []string{"New York", "Toronto"},
			filtered:  true,
		},
		"country match but no city match": {
			countries: []string{"United States"},
			cities:    []string{"Toronto"},
			filtered:  true,
		},
	}

	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			selection := settings.ServerSelection{
				Countries: testCase.countries,
				Cities:    testCase.cities,
			}.WithDefaults(providers.Purevpn)
			filtered := filterServer(server, selection)
			assert.Equal(t, testCase.filtered, filtered)
		})
	}
}

func Test_filterServer_CityWithoutCountry(t *testing.T) {
	t.Parallel()

	server := models.Server{
		VPN:     vpn.OpenVPN,
		Country: "United States",
		City:    "New York",
		UDP:     true,
	}

	testCases := map[string]struct {
		cities   []string
		filtered bool
	}{
		"city match without country": {
			cities:   []string{"New York"},
			filtered: false,
		},
		"city mismatch without country": {
			cities:   []string{"Toronto"},
			filtered: true,
		},
	}

	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			selection := settings.ServerSelection{
				Cities: testCase.cities,
			}.WithDefaults(providers.Purevpn)
			filtered := filterServer(server, selection)
			assert.Equal(t, testCase.filtered, filtered)
		})
	}
}

func Test_filterAllByPossibilities(t *testing.T) {
	t.Parallel()

	testCases := map[string]struct {
		values        []string
		possibilities []string
		filtered      bool
	}{
		"no possibilities means no filter": {
			values:        []string{"p2p"},
			possibilities: nil,
			filtered:      false,
		},
		"single category match": {
			values:        []string{"p2p", "streaming"},
			possibilities: []string{"p2p"},
			filtered:      false,
		},
		"all categories must match": {
			values:        []string{"p2p", "streaming", "gaming"},
			possibilities: []string{"p2p", "streaming"},
			filtered:      false,
		},
		"missing one category filters server": {
			values:        []string{"p2p"},
			possibilities: []string{"p2p", "streaming"},
			filtered:      true,
		},
	}

	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			filtered := filterAllByPossibilities(testCase.values, testCase.possibilities)
			assert.Equal(t, testCase.filtered, filtered)
		})
	}
}
