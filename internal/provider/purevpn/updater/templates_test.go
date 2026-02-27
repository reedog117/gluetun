package updater

import (
	"encoding/base64"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_parseAtomAPIBaseURL(t *testing.T) {
	t.Parallel()

	content := []byte(`(0, _defineProperty2["default"])(AtomApi, "BASE_URL", "https://atomapi.com/");`)
	baseURL, err := parseAtomAPIBaseURL(content)
	require.NoError(t, err)
	assert.Equal(t, "https://atomapi.com/", baseURL)
}

func Test_parseCryptoKeyBase64(t *testing.T) {
	t.Parallel()

	content := []byte(`var p = "I0tSbk5sdk1KdmwhMjAyNQ=="`)
	keyBase64, err := parseCryptoKeyBase64(content)
	require.NoError(t, err)
	assert.Equal(t, "I0tSbk5sdk1KdmwhMjAyNQ==", keyBase64)
}

func Test_encryptForAtom(t *testing.T) {
	t.Parallel()

	encrypted, err := encryptForAtom("password", "I0tSbk5sdk1KdmwhMjAyNQ==")
	require.NoError(t, err)
	_, err = base64.StdEncoding.DecodeString(encrypted)
	assert.NoError(t, err)
}

func Test_parsePureVPNCountrySlug(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "us", parsePureVPNCountrySlug("usca2-auto-udp.ptoserver.com"))
	assert.Equal(t, "uk", parsePureVPNCountrySlug("uk2-auto-udp.ptoserver.com"))
	assert.Equal(t, "", parsePureVPNCountrySlug("broken-hostname"))
}

func Test_parseAtomSecretFromContent(t *testing.T) {
	t.Parallel()

	content := []byte(`window.appConfigs={ATOM_SECRET:"abcDEF1234567890"};`)
	secret := parseAtomSecretFromContent(content)
	assert.Equal(t, "abcDEF1234567890", secret)
}

func Test_resolveAtomSecret(t *testing.T) {
	t.Parallel()

	original := os.Getenv(defaultAtomSecretEnv)
	t.Cleanup(func() { _ = os.Setenv(defaultAtomSecretEnv, original) })

	_ = os.Unsetenv(defaultAtomSecretEnv)
	extracted := resolveAtomSecret([]byte(`ATOM_SECRET:"fromasar123456"`))
	assert.Equal(t, "fromasar123456", extracted)

	_ = os.Setenv(defaultAtomSecretEnv, "fromenv123")
	fromEnv := resolveAtomSecret(nil)
	assert.Equal(t, "fromenv123", fromEnv)

	_ = os.Unsetenv(defaultAtomSecretEnv)
	fallback := resolveAtomSecret(nil)
	assert.Equal(t, defaultAtomSecret, fallback)
}
