package common

import (
	"testing"

	hostcommon "github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEnforceBalanceProtectionLimit(t *testing.T) {
	tests := []struct {
		name       string
		format     types.RelayFormat
		input      string
		assertBody func(t *testing.T, body map[string]interface{})
	}{
		{
			name:   "openai override cannot raise limit",
			format: types.RelayFormatOpenAI,
			input:  `{"model":"gpt","max_tokens":9999}`,
			assertBody: func(t *testing.T, body map[string]interface{}) {
				assert.Equal(t, float64(128), body["max_tokens"])
			},
		},
		{
			name:   "responses injects canonical field",
			format: types.RelayFormatOpenAIResponses,
			input:  `{"model":"gpt"}`,
			assertBody: func(t *testing.T, body map[string]interface{}) {
				assert.Equal(t, float64(128), body["max_output_tokens"])
			},
		},
		{
			name:   "existing lower limit is never increased",
			format: types.RelayFormatOpenAI,
			input:  `{"model":"gpt","max_tokens":32}`,
			assertBody: func(t *testing.T, body map[string]interface{}) {
				assert.Equal(t, float64(32), body["max_tokens"])
			},
		},
		{
			name:   "claude removes legacy alias",
			format: types.RelayFormatClaude,
			input:  `{"model":"claude","max_tokens_to_sample":9999}`,
			assertBody: func(t *testing.T, body map[string]interface{}) {
				assert.Equal(t, float64(128), body["max_tokens"])
				assert.NotContains(t, body, "max_tokens_to_sample")
			},
		},
		{
			name:   "gemini uses nested camel case",
			format: types.RelayFormatGemini,
			input:  `{"generationConfig":{"maxOutputTokens":9999}}`,
			assertBody: func(t *testing.T, body map[string]interface{}) {
				config, ok := body["generationConfig"].(map[string]interface{})
				require.True(t, ok)
				assert.Equal(t, float64(128), config["maxOutputTokens"])
			},
		},
		{
			name:   "gemini batch clamps every nested request",
			format: types.RelayFormatGemini,
			input:  `{"requests":[{"generationConfig":{"maxOutputTokens":9999}},{"generationConfig":{"maxOutputTokens":32}}]}`,
			assertBody: func(t *testing.T, body map[string]interface{}) {
				requests, ok := body["requests"].([]interface{})
				require.True(t, ok)
				assert.NotContains(t, body, "generationConfig")
				first := requests[0].(map[string]interface{})["generationConfig"].(map[string]interface{})
				second := requests[1].(map[string]interface{})["generationConfig"].(map[string]interface{})
				assert.Equal(t, float64(128), first["maxOutputTokens"])
				assert.Equal(t, float64(32), second["maxOutputTokens"])
			},
		},
		{
			name:   "provider nested limit is clamped without injecting openai field",
			format: types.RelayFormatOpenAI,
			input:  `{"inferenceConfig":{"maxTokens":9999}}`,
			assertBody: func(t *testing.T, body map[string]interface{}) {
				config := body["inferenceConfig"].(map[string]interface{})
				assert.Equal(t, float64(128), config["maxTokens"])
				assert.NotContains(t, body, "max_tokens")
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			info := &RelayInfo{
				BalanceProtectionActive:  true,
				EffectiveMaxOutputTokens: 128,
				FinalRequestRelayFormat:  test.format,
			}
			output, err := EnforceBalanceProtectionLimit([]byte(test.input), info)
			require.NoError(t, err)
			var body map[string]interface{}
			require.NoError(t, hostcommon.Unmarshal(output, &body))
			test.assertBody(t, body)
		})
	}
}

func TestEnforceBalanceProtectionLimitNoopOutsideProtectionZone(t *testing.T) {
	input := []byte(`{"max_tokens":9999}`)
	output, err := EnforceBalanceProtectionLimit(input, &RelayInfo{})
	require.NoError(t, err)
	assert.Equal(t, input, output)
}

func TestEnforceBalanceProtectionLimitRejectsClaudeThinkingBudgetConflict(t *testing.T) {
	info := &RelayInfo{
		BalanceProtectionActive:  true,
		EffectiveMaxOutputTokens: 1024,
		FinalRequestRelayFormat:  types.RelayFormatClaude,
	}
	_, err := EnforceBalanceProtectionLimit(
		[]byte(`{"max_tokens":4096,"thinking":{"type":"enabled","budget_tokens":1280}}`),
		info,
	)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "thinking budget_tokens")
}
