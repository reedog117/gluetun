package settings

import (
	"net/netip"
	"testing"

	"github.com/qdm12/gluetun/internal/constants"
	"github.com/qdm12/gluetun/internal/constants/providers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOpenVPNSelection_validate_PurevpnCustomPort(t *testing.T) {
	t.Parallel()

	testCases := map[string]struct {
		protocol string
		port     uint16
		err      error
	}{
		"tcp_80_is_valid": {
			protocol: constants.TCP,
			port:     80,
		},
		"tcp_1194_is_valid": {
			protocol: constants.TCP,
			port:     1194,
		},
		"udp_53_is_valid": {
			protocol: constants.UDP,
			port:     53,
		},
		"udp_1194_is_valid": {
			protocol: constants.UDP,
			port:     1194,
		},
		"tcp_443_is_invalid": {
			protocol: constants.TCP,
			port:     443,
			err:      ErrOpenVPNCustomPortNotAllowed,
		},
		"udp_443_is_invalid": {
			protocol: constants.UDP,
			port:     443,
			err:      ErrOpenVPNCustomPortNotAllowed,
		},
	}

	for testName, testCase := range testCases {
		testCase := testCase
		t.Run(testName, func(t *testing.T) {
			t.Parallel()

			selection := OpenVPNSelection{
				ConfFile:     strPtr(""),
				Protocol:     testCase.protocol,
				EndpointIP:   netip.IPv4Unspecified(),
				CustomPort:   uint16Ptr(testCase.port),
				PIAEncPreset: strPtr(""),
			}

			err := selection.validate(providers.Purevpn)

			if testCase.err == nil {
				require.NoError(t, err)
				return
			}
			require.Error(t, err)
			assert.ErrorIs(t, err, testCase.err)
		})
	}
}

func uint16Ptr(value uint16) *uint16 {
	return &value
}

func strPtr(value string) *string {
	return &value
}
