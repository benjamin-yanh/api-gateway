package common

import (
	"fmt"

	hostcommon "github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/relaykit/types"
)

// EnforceBalanceProtectionLimit is the final outbound guard. It runs after
// adaptor conversion and channel parameter overrides so neither pass-through
// configuration nor an override can raise the immutable request budget.
func EnforceBalanceProtectionLimit(jsonData []byte, info *RelayInfo) ([]byte, error) {
	if info == nil || !info.BalanceProtectionActive || info.EffectiveMaxOutputTokens == 0 {
		return jsonData, nil
	}
	var body map[string]interface{}
	if err := hostcommon.Unmarshal(jsonData, &body); err != nil {
		return nil, fmt.Errorf("apply balance protection output limit: %w", err)
	}
	limit := float64(info.EffectiveMaxOutputTokens)
	format := info.GetFinalRequestRelayFormat()

	hadOutputLimit := false
	for _, key := range []string{"max_tokens", "max_completion_tokens", "max_output_tokens", "max_new_tokens", "maxTokens", "maxOutputTokens"} {
		if _, exists := body[key]; exists {
			hadOutputLimit = true
		}
		if err := capJSONLimit(body, key, limit); err != nil {
			return nil, err
		}
	}
	if inferenceConfig, ok := body["inferenceConfig"].(map[string]interface{}); ok {
		if _, exists := inferenceConfig["maxTokens"]; exists {
			hadOutputLimit = true
		}
		if err := capJSONLimit(inferenceConfig, "maxTokens", limit); err != nil {
			return nil, err
		}
	}

	switch format {
	case types.RelayFormatClaude:
		claudeLimit := limit
		if existing, ok := body["max_tokens"].(float64); ok {
			claudeLimit = existing
		} else {
			body["max_tokens"] = limit
		}
		if thinking, ok := body["thinking"].(map[string]interface{}); ok {
			if budget, ok := thinking["budget_tokens"].(float64); ok && budget >= claudeLimit {
				return nil, fmt.Errorf(
					"balance protection max_tokens %d must exceed Claude thinking budget_tokens %.0f",
					uint(claudeLimit),
					budget,
				)
			}
		}
		delete(body, "max_tokens_to_sample")
	case types.RelayFormatGemini:
		requests, isBatch := body["requests"].([]interface{})
		if !isBatch || body["generationConfig"] != nil || body["contents"] != nil {
			if err := capGeminiGenerationConfig(body, limit); err != nil {
				return nil, err
			}
		}
		if isBatch {
			for _, item := range requests {
				requestBody, ok := item.(map[string]interface{})
				if !ok {
					return nil, fmt.Errorf("invalid Gemini batch request body")
				}
				if err := capGeminiGenerationConfig(requestBody, limit); err != nil {
					return nil, err
				}
			}
		}
	case types.RelayFormatOpenAIResponses, types.RelayFormatOpenAIResponsesCompaction:
		if _, exists := body["max_output_tokens"]; !exists {
			body["max_output_tokens"] = limit
		}
	default:
		if !hadOutputLimit {
			body["max_tokens"] = limit
		}
	}
	return hostcommon.Marshal(body)
}

func capJSONLimit(body map[string]interface{}, key string, limit float64) error {
	value, exists := body[key]
	if !exists {
		return nil
	}
	number, ok := value.(float64)
	if !ok || number < 0 {
		return fmt.Errorf("invalid numeric output limit at %s", key)
	}
	if number > limit {
		body[key] = limit
	}
	return nil
}

func capGeminiGenerationConfig(body map[string]interface{}, limit float64) error {
	config, _ := body["generationConfig"].(map[string]interface{})
	if config == nil {
		config = make(map[string]interface{})
	}
	if err := capJSONLimit(config, "maxOutputTokens", limit); err != nil {
		return err
	}
	if _, exists := config["maxOutputTokens"]; !exists {
		if snake, snakeExists := config["max_output_tokens"]; snakeExists {
			number, ok := snake.(float64)
			if !ok || number < 0 {
				return fmt.Errorf("invalid numeric output limit at generationConfig.max_output_tokens")
			}
			if number > limit {
				number = limit
			}
			config["maxOutputTokens"] = number
			delete(config, "max_output_tokens")
		} else {
			config["maxOutputTokens"] = limit
		}
	}
	if _, exists := config["maxOutputTokens"]; !exists {
		config["maxOutputTokens"] = limit
	}
	body["generationConfig"] = config
	return nil
}
