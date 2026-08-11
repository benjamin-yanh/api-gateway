package controller

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateNativeAppRedirectURI(t *testing.T) {
	tests := []struct {
		name  string
		raw   string
		valid bool
	}{
		{name: "IPv4 loopback", raw: "http://127.0.0.1:49152/callback", valid: true},
		{name: "localhost", raw: "http://localhost:49152/callback", valid: true},
		{name: "IPv6 loopback", raw: "http://[::1]:49152/callback", valid: true},
		{name: "missing port", raw: "http://127.0.0.1/callback", valid: false},
		{name: "privileged port", raw: "http://127.0.0.1:80/callback", valid: false},
		{name: "remote host", raw: "https://example.com/callback", valid: false},
		{name: "lookalike host", raw: "http://localhost.example.com:49152/callback", valid: false},
		{name: "embedded credentials", raw: "http://user@127.0.0.1:49152/callback", valid: false},
		{name: "query parameters", raw: "http://127.0.0.1:49152/callback?next=bad", valid: false},
		{name: "fragment", raw: "http://127.0.0.1:49152/callback#token", valid: false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			redirectURI, valid := validateNativeAppRedirectURI(test.raw)
			assert.Equal(t, test.valid, valid)
			if test.valid {
				assert.NotEmpty(t, redirectURI)
			} else {
				assert.Empty(t, redirectURI)
			}
		})
	}
}

func TestNativeAppPKCEChallengeMatchesS256(t *testing.T) {
	verifier := "dBjftJeZ4CVP-mB92K27uhbUJU1p1r_wW1gFWFOEjXk"
	require.True(t, nativeAppCodeVerifierPattern.MatchString(verifier))
	assert.Equal(t, "E9Melhoa2OwvFrEMTJguCHaoeK1t8URWbuGJSstw-cM", nativeAppPKCEChallenge(verifier))
}
