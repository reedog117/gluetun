package updater

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_inferPureVPNTraits(t *testing.T) {
	t.Parallel()

	testCases := map[string]struct {
		hostname                    string
		portForward, qr, obfuscated bool
	}{
		"regular": {
			hostname: "us2-udp.ptoserver.com",
		},
		"port forwarding": {
			hostname:    "us2-udp-pf.ptoserver.com",
			portForward: true,
		},
		"quantum resistant": {
			hostname: "us2-auto-udp-qr.ptoserver.com",
			qr:       true,
		},
		"obfuscated": {
			hostname:   "us2-obf-udp.ptoserver.com",
			obfuscated: true,
		},
		"multiple traits": {
			hostname:    "us2-udp-qr-pf.ptoserver.com",
			portForward: true,
			qr:          true,
		},
	}

	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			portForward, qr, obfuscated := inferPureVPNTraits(testCase.hostname)

			assert.Equal(t, testCase.portForward, portForward)
			assert.Equal(t, testCase.qr, qr)
			assert.Equal(t, testCase.obfuscated, obfuscated)
		})
	}
}
