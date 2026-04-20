// Package local provides an embedded local mode for running agents in-process.
package local

import omnillm "github.com/plexusone/omnillm-core"

// CanonicalToolMap maps multi-agent-spec canonical tool names to AgentKit local tool names.
// Canonical tools are defined in: https://github.com/plexusone/multi-agent-spec
var CanonicalToolMap = map[string]string{
	"Read":      "read",
	"Write":     "write",
	"Edit":      "write", // Edit uses write with diff semantics
	"Glob":      "glob",
	"Grep":      "grep",
	"Bash":      "shell",
	"WebSearch": "shell", // Implemented via curl/external commands
	"WebFetch":  "shell", // Implemented via curl/external commands
	"Task":      "",      // Task spawning handled by orchestration layer
}

// CanonicalModelMap maps multi-agent-spec canonical model names to provider-specific model IDs.
// Canonical models: haiku, sonnet, opus
var CanonicalModelMap = map[string]map[omnillm.ProviderName]string{
	"haiku": {
		omnillm.ProviderNameAnthropic: omnillm.ModelClaude3_5Haiku,
		omnillm.ProviderNameOpenAI:    omnillm.ModelGPT4oMini,
		omnillm.ProviderNameGemini:    omnillm.ModelGemini2_5Flash,
		omnillm.ProviderNameXAI:       omnillm.ModelGrok3Mini,
		omnillm.ProviderNameOllama:    omnillm.ModelOllamaLlama3_8B,
	},
	"sonnet": {
		omnillm.ProviderNameAnthropic: omnillm.ModelClaudeSonnet4,
		omnillm.ProviderNameOpenAI:    omnillm.ModelGPT4o,
		omnillm.ProviderNameGemini:    omnillm.ModelGemini2_5Pro,
		omnillm.ProviderNameXAI:       omnillm.ModelGrok4FastNonReasoning,
		omnillm.ProviderNameOllama:    omnillm.ModelOllamaLlama3_70B,
	},
	"opus": {
		omnillm.ProviderNameAnthropic: omnillm.ModelClaudeOpus4,
		omnillm.ProviderNameOpenAI:    omnillm.ModelGPT5,
		omnillm.ProviderNameGemini:    omnillm.ModelGemini2_5Pro,
		omnillm.ProviderNameXAI:       omnillm.ModelGrok4_1FastReasoning,
		omnillm.ProviderNameOllama:    omnillm.ModelOllamaQwen2_5,
	},
}

// MapCanonicalTools converts multi-agent-spec canonical tool names to AgentKit local tool names.
// Unknown tools are skipped. Empty mappings (like "Task") are also skipped.
func MapCanonicalTools(canonical []string) []string {
	seen := make(map[string]bool)
	var local []string

	for _, tool := range canonical {
		if mapped, ok := CanonicalToolMap[tool]; ok && mapped != "" {
			// Avoid duplicates (e.g., both "Write" and "Edit" map to "write")
			if !seen[mapped] {
				local = append(local, mapped)
				seen[mapped] = true
			}
		}
	}

	return local
}

// MapCanonicalModel converts a multi-agent-spec canonical model name to a provider-specific model ID.
// If the model is not a canonical name (haiku, sonnet, opus), it is returned as-is.
// If the provider is not found in the mapping, the canonical name is returned.
func MapCanonicalModel(canonical string, provider omnillm.ProviderName) string {
	if models, ok := CanonicalModelMap[canonical]; ok {
		if model, ok := models[provider]; ok {
			return model
		}
	}
	// Not a canonical model name, return as-is (allows direct model IDs)
	return canonical
}

// ProviderNameFromString converts a string provider name to omnillm.ProviderName.
// Returns the input as ProviderName if not recognized (allows custom providers).
func ProviderNameFromString(name string) omnillm.ProviderName {
	switch name {
	case "anthropic":
		return omnillm.ProviderNameAnthropic
	case "openai":
		return omnillm.ProviderNameOpenAI
	case "gemini":
		return omnillm.ProviderNameGemini
	case "xai":
		return omnillm.ProviderNameXAI
	case "ollama":
		return omnillm.ProviderNameOllama
	case "bedrock":
		return omnillm.ProviderNameBedrock
	default:
		return omnillm.ProviderName(name)
	}
}

// IsCanonicalModel returns true if the model name is a canonical model (haiku, sonnet, opus).
func IsCanonicalModel(model string) bool {
	_, ok := CanonicalModelMap[model]
	return ok
}

// GetCanonicalModelForProvider returns all canonical model mappings for a specific provider.
func GetCanonicalModelForProvider(provider omnillm.ProviderName) map[string]string {
	result := make(map[string]string)
	for canonical, providers := range CanonicalModelMap {
		if model, ok := providers[provider]; ok {
			result[canonical] = model
		}
	}
	return result
}
