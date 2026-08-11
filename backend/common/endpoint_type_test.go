package common

import (
	"testing"

	"github.com/QuantumNous/new-api/constant"
	"github.com/stretchr/testify/assert"
)

func TestGetEndpointTypesByChannelTypeRestrictsImageOnlyModels(t *testing.T) {
	tests := []struct {
		name      string
		modelName string
	}{
		{name: "grok image", modelName: "grok-imagine-image"},
		{name: "grok image pro", modelName: "grok-imagine-image-pro"},
		{name: "openai image", modelName: "gpt-image-1"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t,
				[]constant.EndpointType{constant.EndpointTypeImageGeneration},
				GetEndpointTypesByChannelType(constant.ChannelTypeXai, test.modelName),
			)
		})
	}
}

func TestGetEndpointTypesByChannelTypeKeepsXAITextEndpoints(t *testing.T) {
	assert.Equal(t,
		[]constant.EndpointType{
			constant.EndpointTypeOpenAI,
			constant.EndpointTypeOpenAIResponse,
		},
		GetEndpointTypesByChannelType(constant.ChannelTypeXai, "grok-4-1-fast-reasoning"),
	)
}
