package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateDecisionCapabilityContracts(t *testing.T) {
	cfg := &RouterConfig{BackendModels: BackendModels{ModelConfig: map[string]ModelParams{
		"general":   {Capabilities: []string{"chat", "code", "tool_calling"}},
		"reasoning": {Capabilities: []string{"chat", "reasoning"}},
	}}}

	require.NoError(t, validateDecisionCapabilityContracts(cfg, Decision{
		Name:                 "code-route",
		RequiredCapabilities: []string{"chat", "code"},
		ModelRefs:            []ModelRef{{Model: "general"}},
	}))

	require.NoError(t, validateDecisionCapabilityContracts(cfg, Decision{
		Name:                 "reasoning-route",
		RequiredCapabilities: []string{"chat", "reasoning"},
		ModelRefs:            []ModelRef{{Model: "general"}, {Model: "reasoning"}},
	}))

	err := validateDecisionCapabilityContracts(cfg, Decision{
		Name:                 "tool-route",
		RequiredCapabilities: []string{"chat", "tool_calling"},
		ModelRefs:            []ModelRef{{Model: "reasoning"}},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no modelRef satisfies")
}

func TestValidateDecisionCapabilityContractsRejectsUnknownAndDuplicateIDs(t *testing.T) {
	cfg := &RouterConfig{}
	for _, capabilities := range [][]string{{"tool-use"}, {"chat", "chat"}, {" chat"}} {
		err := validateDecisionCapabilityContracts(cfg, Decision{
			Name:                 "invalid",
			RequiredCapabilities: capabilities,
		})
		require.Error(t, err)
	}
}

func TestMissingModelCapabilitiesPreservesRequirementOrder(t *testing.T) {
	missing := MissingModelCapabilities(
		ModelParams{Capabilities: []string{"chat"}},
		[]string{"chat", "reasoning", "tool_calling"},
	)
	assert.Equal(t, []string{"reasoning", "tool_calling"}, missing)
}
