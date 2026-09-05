package common

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestValidateConfiguredSecret(t *testing.T) {
	tests := []struct {
		name   string
		secret string
		valid  bool
	}{
		{name: "strong", secret: "nM7!qZ2@vR9#xT4$kL8%pC6&dF3*sH1+", valid: true},
		{name: "too short", secret: "short-secret", valid: false},
		{name: "placeholder", secret: "replace-with-a-long-random-session-secret", valid: false},
		{name: "low diversity", secret: strings.Repeat("a", 64), valid: false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := validateConfiguredSecret("SESSION_SECRET", test.secret)
			if test.valid {
				assert.NoError(t, err)
			} else {
				assert.Error(t, err)
			}
		})
	}
}
