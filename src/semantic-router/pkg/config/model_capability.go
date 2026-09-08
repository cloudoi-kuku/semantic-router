package config

import "strings"

// ModelCapabilityCatalogVersion identifies the provider-neutral vocabulary
// understood by hard model-eligibility checks. Model cards may retain
// additional descriptive capabilities, but decisions can require only these
// stable identifiers.
const ModelCapabilityCatalogVersion = "vllm-sr/model-capability-catalog/v1alpha1"

const (
	ModelCapabilityChat             = "chat"
	ModelCapabilityText             = "text"
	ModelCapabilityCode             = "code"
	ModelCapabilityReasoning        = "reasoning"
	ModelCapabilityToolCalling      = "tool_calling"
	ModelCapabilityParallelTools    = "parallel_tool_calling"
	ModelCapabilityStructuredOutput = "structured_output"
	ModelCapabilityJSONSchema       = "json_schema"
	ModelCapabilityVision           = "vision"
	ModelCapabilityAudio            = "audio"
	ModelCapabilityVideo            = "video"
	ModelCapabilityFile             = "file"
	ModelCapabilityEmbeddings       = "embeddings"
	ModelCapabilityImageGeneration  = "image_generation"
)

var supportedRequiredModelCapabilities = map[string]struct{}{
	ModelCapabilityChat:             {},
	ModelCapabilityText:             {},
	ModelCapabilityCode:             {},
	ModelCapabilityReasoning:        {},
	ModelCapabilityToolCalling:      {},
	ModelCapabilityParallelTools:    {},
	ModelCapabilityStructuredOutput: {},
	ModelCapabilityJSONSchema:       {},
	ModelCapabilityVision:           {},
	ModelCapabilityAudio:            {},
	ModelCapabilityVideo:            {},
	ModelCapabilityFile:             {},
	ModelCapabilityEmbeddings:       {},
	ModelCapabilityImageGeneration:  {},
}

func IsSupportedRequiredModelCapability(capability string) bool {
	_, ok := supportedRequiredModelCapabilities[strings.TrimSpace(capability)]
	return ok
}

// MissingModelCapabilities returns required capabilities absent from the
// model card. Matching is exact after trimming so catalogue identifiers remain
// portable across providers and configuration surfaces.
func MissingModelCapabilities(params ModelParams, required []string) []string {
	if len(required) == 0 {
		return nil
	}
	available := make(map[string]struct{}, len(params.Capabilities))
	for _, capability := range params.Capabilities {
		available[strings.TrimSpace(capability)] = struct{}{}
	}
	missing := make([]string, 0, len(required))
	for _, capability := range required {
		capability = strings.TrimSpace(capability)
		if _, ok := available[capability]; !ok {
			missing = append(missing, capability)
		}
	}
	return missing
}
