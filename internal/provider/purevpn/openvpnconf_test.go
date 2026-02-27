package purevpn

import (
	"net/netip"
	"strings"
	"testing"

	"github.com/qdm12/gluetun/internal/configuration/settings"
	"github.com/qdm12/gluetun/internal/constants"
	"github.com/qdm12/gluetun/internal/constants/providers"
	"github.com/qdm12/gluetun/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProviderOpenVPNConfig_UsesBuiltInCryptoMaterial(t *testing.T) {
	t.Parallel()

	p := Provider{}
	connection := models.Connection{
		IP:       netip.MustParseAddr("1.2.3.4"),
		Port:     15021,
		Protocol: constants.UDP,
		Hostname: "us2-udp.ptoserver.com",
	}
	openvpnSettings := settings.OpenVPN{}.WithDefaults(providers.Purevpn)

	lines := p.OpenVPNConfig(connection, openvpnSettings, false)

	assert.True(t, hasLineContaining(lines, "remote-cert-tls server"))
	assert.True(t, hasLineContaining(lines, "key-direction 1"))
	assert.True(t, hasLineContaining(lines, "compress"))
	assert.True(t, hasLineContaining(lines, "route-method exe"))
	assert.True(t, hasLineContaining(lines, "route-delay 0"))
	assert.True(t, hasLineContaining(lines, "script-security 2"))
	assert.True(t, hasLineContaining(lines, "explicit-exit-notify 2"))
	assert.True(t, hasLineContaining(lines, "<ca>"))
	assert.True(t, hasLineContaining(lines, "</ca>"))
	assert.True(t, hasLineContaining(lines, "<cert>"))
	assert.True(t, hasLineContaining(lines, "</cert>"))
	assert.True(t, hasLineContaining(lines, "<key>"))
	assert.True(t, hasLineContaining(lines, "</key>"))
	assert.True(t, hasLineContaining(lines, "<tls-auth>"))
	assert.True(t, hasLineContaining(lines, "</tls-auth>"))
}

func TestOpenVPNConfig_UDP1194AddsUDPFallback(t *testing.T) {
	t.Parallel()

	p := Provider{}
	connection := models.Connection{
		IP:       netip.MustParseAddr("1.2.3.4"),
		Port:     1194,
		Protocol: constants.UDP,
	}

	lines := p.OpenVPNConfig(connection, testOpenVPNSettings(), true)

	primaryIndex := indexOfLine(lines, "remote 1.2.3.4 1194")
	fallbackIndex := indexOfLine(lines, "remote 1.2.3.4 53")
	require.NotEqual(t, -1, primaryIndex)
	require.NotEqual(t, -1, fallbackIndex)
	assert.Less(t, primaryIndex, fallbackIndex)
}

func TestOpenVPNConfig_TCP1194AddsTCPFallback(t *testing.T) {
	t.Parallel()

	p := Provider{}
	connection := models.Connection{
		IP:       netip.MustParseAddr("1.2.3.4"),
		Port:     1194,
		Protocol: constants.TCP,
	}

	lines := p.OpenVPNConfig(connection, testOpenVPNSettings(), true)

	primaryIndex := indexOfLine(lines, "remote 1.2.3.4 1194")
	fallbackIndex := indexOfLine(lines, "remote 1.2.3.4 80")
	require.NotEqual(t, -1, primaryIndex)
	require.NotEqual(t, -1, fallbackIndex)
	assert.Less(t, primaryIndex, fallbackIndex)
}

func TestOpenVPNConfig_Non1194HasNoFallback(t *testing.T) {
	t.Parallel()

	p := Provider{}
	connection := models.Connection{
		IP:       netip.MustParseAddr("1.2.3.4"),
		Port:     53,
		Protocol: constants.UDP,
	}

	lines := p.OpenVPNConfig(connection, testOpenVPNSettings(), true)

	assert.NotEqual(t, -1, indexOfLine(lines, "remote 1.2.3.4 53"))
	assert.Equal(t, -1, indexOfLine(lines, "remote 1.2.3.4 1194"))
	assert.Equal(t, -1, indexOfLine(lines, "remote 1.2.3.4 80"))
}

func testOpenVPNSettings() settings.OpenVPN {
	return settings.OpenVPN{
		User:          strPtr(""),
		Auth:          strPtr(""),
		MSSFix:        uint16Ptr(0),
		Interface:     "tun0",
		ProcessUser:   "root",
		Verbosity:     intPtr(1),
		EncryptedKey:  strPtr(""),
		KeyPassphrase: strPtr(""),
		Cert:          strPtr(""),
		Key:           strPtr(""),
	}
}

func indexOfLine(lines []string, target string) int {
	for i, line := range lines {
		if line == target {
			return i
		}
	}
	return -1
}

func strPtr(value string) *string { return &value }
func uint16Ptr(value uint16) *uint16 { return &value }
func intPtr(value int) *int { return &value }

func hasLineContaining(lines []string, needle string) bool {
	for _, line := range lines {
		if strings.Contains(line, needle) {
			return true
		}
	}
	return false
}
