package settings

import (
	"testing"

	"github.com/qdm12/gluetun/internal/constants/providers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_parsePureVPNServerType(t *testing.T) {
	t.Parallel()

	testCases := map[string]struct {
		raw, expected string
	}{
		"empty":                   {raw: "", expected: ""},
		"regular":                 {raw: "regular", expected: "regular"},
		"pf alias":                {raw: "pf", expected: "portforwarding"},
		"port forwarding alias":   {raw: "port_forwarding", expected: "portforwarding"},
		"qr alias":                {raw: "qr", expected: "quantumresistant"},
		"quantum resistant alias": {raw: "quantum-resistant", expected: "quantumresistant"},
		"obf alias":               {raw: "obf", expected: "obfuscation"},
		"obfuscated alias":        {raw: "obfuscated", expected: "obfuscation"},
		"unknown":                 {raw: "fast", expected: "fast"},
	}

	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, testCase.expected, parsePureVPNServerType(testCase.raw))
		})
	}
}

func Test_validateFeatureFilters_PureVPNServerType(t *testing.T) {
	t.Parallel()

	testCases := map[string]struct {
		provider   string
		serverType string
		err        error
	}{
		"valid with purevpn": {
			provider:   providers.Purevpn,
			serverType: "obfuscation",
		},
		"invalid provider": {
			provider:   providers.Mullvad,
			serverType: "regular",
			err:        ErrPureVPNServerTypeNotSupported,
		},
		"invalid value": {
			provider:   providers.Purevpn,
			serverType: "fast",
			err:        ErrPureVPNServerTypeNotValid,
		},
	}

	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			selection := ServerSelection{PureVPNServerType: testCase.serverType}.WithDefaults(testCase.provider)
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
