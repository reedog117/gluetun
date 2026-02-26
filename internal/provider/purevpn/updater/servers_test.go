package updater

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_shouldUseGeolocation(t *testing.T) {
	t.Parallel()

	testCases := map[string]struct {
		parsedCountry      string
		geolocationCountry string
		use                bool
	}{
		"empty parsed country": {
			parsedCountry:      "",
			geolocationCountry: "Germany",
			use:                true,
		},
		"matching countries": {
			parsedCountry:      "India",
			geolocationCountry: "India",
			use:                true,
		},
		"mismatching countries": {
			parsedCountry:      "Russia",
			geolocationCountry: "Germany",
			use:                false,
		},
	}

	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			use := shouldUseGeolocation(testCase.parsedCountry, testCase.geolocationCountry)
			assert.Equal(t, testCase.use, use)
		})
	}
}
