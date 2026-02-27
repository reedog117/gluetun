package settings

import (
	"testing"

	"github.com/qdm12/gluetun/internal/constants/providers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_parsePureVPNServerTypes(t *testing.T) {
	t.Parallel()

	raw := []string{
		"",
		"regular",
		"pf",
		"port_forwarding",
		"qr",
		"quantum-resistant",
		"obf",
		"obfuscated",
		"p2p",
		"fast",
		"regular",
	}

	parsed := parsePureVPNServerTypes(raw)
	assert.Equal(t,
		[]string{"regular", "portforwarding", "quantumresistant", "obfuscation", "p2p", "fast"},
		parsed)
}

func Test_validateFeatureFilters_PureVPNServerTypes(t *testing.T) {
	t.Parallel()

	testCases := map[string]struct {
		provider    string
		serverTypes []string
		err         error
	}{
		"valid with purevpn": {
			provider:    providers.Purevpn,
			serverTypes: []string{"obfuscation", "p2p"},
		},
		"invalid provider": {
			provider:    providers.Mullvad,
			serverTypes: []string{"regular"},
			err:         ErrPureVPNServerTypeNotSupported,
		},
		"invalid value": {
			provider:    providers.Purevpn,
			serverTypes: []string{"regular", "fast"},
			err:         ErrPureVPNServerTypeNotValid,
		},
	}

	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			selection := ServerSelection{PureVPNServerTypes: testCase.serverTypes}.WithDefaults(testCase.provider)
			err := validateFeatureFilters(selection, testCase.provider)

			if testCase.err == nil {
				require.NoError(t, err)
				return
			}

			require.Error(t, err)
			assert.ErrorIs(t, err, testCase.err)
		})
	}
}

func Test_normalizePureVPNCodes(t *testing.T) {
	t.Parallel()

	codes := normalizePureVPNCodes([]string{" US ", "us", "de", "", "DE"})
	assert.Equal(t, []string{"us", "de"}, codes)
}

func Test_validateFeatureFilters_PureVPNLocationCodeFilters(t *testing.T) {
	t.Parallel()

	testCases := map[string]struct {
		provider      string
		countryCodes  []string
		locationCodes []string
		err           error
	}{
		"valid country and location codes with purevpn": {
			provider:      providers.Purevpn,
			countryCodes:  []string{"us", "de"},
			locationCodes: []string{"usca", "ukm"},
		},
		"country codes not supported on other providers": {
			provider:     providers.Mullvad,
			countryCodes: []string{"us"},
			err:          ErrPureVPNCountryCodesNotSupported,
		},
		"location codes not supported on other providers": {
			provider:      providers.Mullvad,
			locationCodes: []string{"usca"},
			err:           ErrPureVPNLocationCodesNotSupported,
		},
		"invalid country code": {
			provider:     providers.Purevpn,
			countryCodes: []string{"usa"},
			err:          ErrPureVPNCountryCodeNotValid,
		},
		"invalid location code": {
			provider:      providers.Purevpn,
			locationCodes: []string{"us1"},
			err:           ErrPureVPNLocationCodeNotValid,
		},
	}

	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			selection := ServerSelection{
				PureVPNCountryCodes:  testCase.countryCodes,
				PureVPNLocationCodes: testCase.locationCodes,
			}.WithDefaults(testCase.provider)
			err := validateFeatureFilters(selection, testCase.provider)

			if testCase.err == nil {
				require.NoError(t, err)
				return
			}

			require.Error(t, err)
			assert.ErrorIs(t, err, testCase.err)
		})
	}
}

func Test_ServerSelection_WithDefaults_PureVPNTypesUseDefaultProtocol(t *testing.T) {
	t.Parallel()

	for _, serverTypes := range [][]string{{"regular"}, {"obfuscation"}, {"p2p", "quantumresistant"}} {
		selection := ServerSelection{PureVPNServerTypes: serverTypes}.WithDefaults(providers.Purevpn)
		assert.Equal(t, "udp", selection.OpenVPN.Protocol)
	}
}
