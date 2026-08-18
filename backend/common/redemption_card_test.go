package common

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenerateRedemptionCardKey(t *testing.T) {
	seen := make(map[string]struct{}, 1000)
	for range 1000 {
		key, err := GenerateRedemptionCardKey()
		require.NoError(t, err)
		assert.Len(t, key, RedemptionCardKeyLength)
		assert.Regexp(t, `^[23456789ABCDEFGHJKLMNPQRSTUVWXYZ]{24}$`, key)
		_, duplicate := seen[key]
		assert.False(t, duplicate)
		seen[key] = struct{}{}
	}
}
