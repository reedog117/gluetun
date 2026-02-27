package storage

import (
	"testing"

	"github.com/qdm12/gluetun/internal/configuration/settings"
	"github.com/qdm12/gluetun/internal/constants/providers"
	"github.com/qdm12/gluetun/internal/constants/vpn"
	"github.com/qdm12/gluetun/internal/models"
	"github.com/stretchr/testify/assert"
)

func Test_filterByPureVPNServerType_MultiTraitHost(t *testing.T) {
	t.Parallel()

	server := models.Server{
		PortForward:      true,
		QuantumResistant: true,
		Obfuscated:       true,
		Categories:       []string{"p2p"},
	}

	testCases := map[string]struct {
		serverType string
		filtered   bool
	}{
		"regular filters out multi-trait host": {
			serverType: "regular",
			filtered:   true,
		},
		"portforwarding keeps multi-trait host": {
			serverType: "portforwarding",
			filtered:   false,
		},
		"quantumresistant keeps multi-trait host": {
			serverType: "quantumresistant",
			filtered:   false,
		},
		"obfuscation keeps multi-trait host": {
			serverType: "obfuscation",
			filtered:   false,
		},
		"p2p keeps multi-trait host": {
			serverType: "p2p",
			filtered:   false,
		},
	}

	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			filtered := filterByPureVPNServerType(server, testCase.serverType)
			assert.Equal(t, testCase.filtered, filtered)
		})
	}
}

func Test_filterServer_PureVPNServerTypeAndHostname(t *testing.T) {
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
		serverType string
		filtered   bool
	}{
		"hostname match with obfuscation type": {
			serverType: "obfuscation",
			filtered:   false,
		},
		"hostname match with portforwarding type": {
			serverType: "portforwarding",
			filtered:   false,
		},
		"hostname match with quantumresistant type": {
			serverType: "quantumresistant",
			filtered:   false,
		},
		"hostname match with p2p type": {
			serverType: "p2p",
			filtered:   false,
		},
		"hostname match with regular type": {
			serverType: "regular",
			filtered:   true,
		},
	}

	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			selection := settings.ServerSelection{
				PureVPNServerType: testCase.serverType,
				Hostnames:         []string{server.Hostname},
			}.WithDefaults(providers.Purevpn)
			filtered := filterServer(server, selection)
			assert.Equal(t, testCase.filtered, filtered)
		})
	}
}
