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

func hasLineContaining(lines []string, needle string) bool {
	for _, line := range lines {
		if strings.Contains(line, needle) {
			return true
		}
	}
	return false
}
