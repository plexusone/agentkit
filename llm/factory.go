// Package llm provides LLM model creation and management.
package llm

import (
	"context"
	"fmt"

	"github.com/plexusone/omnillm"
	omnillmhook "github.com/plexusone/omniobserve/integrations/omnillm"
	"github.com/plexusone/omniobserve/llmops"
	"google.golang.org/adk/model"
	"google.golang.org/adk/model/gemini"
	"google.golang.org/genai"

	"github.com/plexusone/agentkit/config"
	"github.com/plexusone/agentkit/llm/adapters"

	// Import observability providers (driver registration via init())
	_ "github.com/plexusone/omniobserve/llmops/langfuse"
	_ "github.com/plexusone/opik-go/llmops"
	_ "github.com/plexusone/phoenix-go/llmops"
)

// ModelFactory creates LLM models based on configuration.
type ModelFactory struct {
	cfg      *config.Config
	obsHook  omnillm.ObservabilityHook
	obsClose func() error
}

// NewModelFactory creates a new model factory.
func NewModelFactory(cfg *config.Config) *ModelFactory {
	mf := &ModelFactory{cfg: cfg}

	// Initialize observability if enabled
	if cfg.ObservabilityEnabled && cfg.ObservabilityProvider != "" {
		hook, closeFn := mf.initObservability()
		mf.obsHook = hook
		mf.obsClose = closeFn
	}

	return mf
}

// initObservability initializes the observability provider and returns a hook.
func (mf *ModelFactory) initObservability() (omnillm.ObservabilityHook, func() error) {
	opts := []llmops.ClientOption{
		llmops.WithProjectName(mf.cfg.ObservabilityProject),
	}

	if mf.cfg.ObservabilityAPIKey != "" {
		opts = append(opts, llmops.WithAPIKey(mf.cfg.ObservabilityAPIKey))
	}

	if mf.cfg.ObservabilityEndpoint != "" {
		opts = append(opts, llmops.WithEndpoint(mf.cfg.ObservabilityEndpoint))
	}

	provider, err := llmops.Open(mf.cfg.ObservabilityProvider, opts...)
	if err != nil {
		// Log error but don't fail - observability is optional
		fmt.Printf("Warning: failed to initialize observability provider %s: %v\n", mf.cfg.ObservabilityProvider, err)
		return nil, nil
	}

	return omnillmhook.NewHook(provider), provider.Close
}

// Close cleans up resources (call when factory is no longer needed).
func (mf *ModelFactory) Close() error {
	if mf.obsClose != nil {
		return mf.obsClose()
	}
	return nil
}

// CreateModel creates an LLM model based on the configured provider.
func (mf *ModelFactory) CreateModel(ctx context.Context) (model.LLM, error) {
	switch mf.cfg.LLMProvider {
	case "gemini", "":
		return mf.createGeminiModel(ctx)
	case "claude":
		return mf.createClaudeModel()
	case "openai":
		return mf.createOpenAIModel()
	case "xai":
		return mf.createXAIModel()
	case "ollama":
		return mf.createOllamaModel()
	default:
		return nil, fmt.Errorf("unsupported LLM provider: %s (supported: gemini, claude, openai, xai, ollama)", mf.cfg.LLMProvider)
	}
}

// createGeminiModel creates a Gemini model.
func (mf *ModelFactory) createGeminiModel(ctx context.Context) (model.LLM, error) {
	apiKey := mf.cfg.GeminiAPIKey
	if apiKey == "" {
		apiKey = mf.cfg.LLMAPIKey
	}

	if apiKey == "" {
		return nil, fmt.Errorf("gemini API key not set - please set GOOGLE_API_KEY or GEMINI_API_KEY")
	}

	modelName := mf.cfg.LLMModel
	if modelName == "" {
		modelName = "gemini-2.0-flash-exp"
	}

	return gemini.NewModel(ctx, modelName, &genai.ClientConfig{
		APIKey: apiKey,
	})
}

// createClaudeModel creates a Claude model using OmniLLM.
func (mf *ModelFactory) createClaudeModel() (model.LLM, error) {
	apiKey := mf.cfg.ClaudeAPIKey
	if apiKey == "" {
		apiKey = mf.cfg.LLMAPIKey
	}

	if apiKey == "" {
		return nil, fmt.Errorf("claude API key not set - please set CLAUDE_API_KEY or ANTHROPIC_API_KEY")
	}

	modelName := mf.cfg.LLMModel
	if modelName == "" {
		modelName = "claude-sonnet-4-20250514"
	}

	return adapters.NewOmniLLMAdapterWithConfig(adapters.OmniLLMAdapterConfig{
		ProviderName:      "anthropic",
		APIKey:            apiKey,
		ModelName:         modelName,
		ObservabilityHook: mf.obsHook,
	})
}

// createOpenAIModel creates an OpenAI model using OmniLLM.
func (mf *ModelFactory) createOpenAIModel() (model.LLM, error) {
	apiKey := mf.cfg.OpenAIAPIKey
	if apiKey == "" {
		apiKey = mf.cfg.LLMAPIKey
	}

	if apiKey == "" {
		return nil, fmt.Errorf("openai API key not set - please set OPENAI_API_KEY")
	}

	modelName := mf.cfg.LLMModel
	if modelName == "" {
		modelName = "gpt-4o-mini"
	}

	return adapters.NewOmniLLMAdapterWithConfig(adapters.OmniLLMAdapterConfig{
		ProviderName:      "openai",
		APIKey:            apiKey,
		ModelName:         modelName,
		ObservabilityHook: mf.obsHook,
	})
}

// createXAIModel creates an xAI Grok model using OmniLLM.
func (mf *ModelFactory) createXAIModel() (model.LLM, error) {
	apiKey := mf.cfg.XAIAPIKey
	if apiKey == "" {
		apiKey = mf.cfg.LLMAPIKey
	}

	if apiKey == "" {
		return nil, fmt.Errorf("xAI API key not set - please set XAI_API_KEY")
	}

	modelName := mf.cfg.LLMModel
	if modelName == "" {
		modelName = "grok-3"
	}

	return adapters.NewOmniLLMAdapterWithConfig(adapters.OmniLLMAdapterConfig{
		ProviderName:      "xai",
		APIKey:            apiKey,
		ModelName:         modelName,
		ObservabilityHook: mf.obsHook,
	})
}

// createOllamaModel creates an Ollama model using OmniLLM.
func (mf *ModelFactory) createOllamaModel() (model.LLM, error) {
	modelName := mf.cfg.LLMModel
	if modelName == "" {
		modelName = "llama3.2"
	}

	// Ollama doesn't need an API key for local instances
	return adapters.NewOmniLLMAdapterWithConfig(adapters.OmniLLMAdapterConfig{
		ProviderName:      "ollama",
		APIKey:            "",
		ModelName:         modelName,
		ObservabilityHook: mf.obsHook,
	})
}

// GetProviderInfo returns information about the current provider.
func (mf *ModelFactory) GetProviderInfo() string {
	return fmt.Sprintf("Provider: %s, Model: %s", mf.cfg.LLMProvider, mf.cfg.LLMModel)
}
