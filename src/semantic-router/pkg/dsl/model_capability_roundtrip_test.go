package dsl

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRequiredCapabilitiesCompileAndRoundTrip(t *testing.T) {
	source := `
MODEL capable {
  capabilities: ["chat", "reasoning"]
}

ROUTE reasoned {
  PRIORITY 10
  REQUIRES ["chat", "reasoning"]
  MODEL "capable"
}`

	cfg, errs := Compile(source)
	require.Empty(t, errs)
	require.Len(t, cfg.Decisions, 1)
	assert.Equal(t, []string{"chat", "reasoning"}, cfg.Decisions[0].RequiredCapabilities)

	decompiled, err := Decompile(cfg)
	require.NoError(t, err)
	assert.True(t, strings.Contains(decompiled, `REQUIRES ["chat", "reasoning"]`))

	roundTripped, errs := Compile(decompiled)
	require.Empty(t, errs)
	assert.Equal(t, cfg.Decisions[0].RequiredCapabilities, roundTripped.Decisions[0].RequiredCapabilities)
}
