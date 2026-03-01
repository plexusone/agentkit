// Package adapters provides LLM adapters for different providers.
package adapters

import (
	"context"
	"fmt"
	"iter"

	"github.com/plexusone/omnillm"
	"github.com/plexusone/omnillm/provider"
	"google.golang.org/adk/model"
	"google.golang.org/genai"
)

// OmniLLMAdapterConfig holds configuration for creating a OmniLLM adapter.
type OmniLLMAdapterConfig struct {
	ProviderName      string
	APIKey            string //nolint:gosec // G117: Config needs API key field
	ModelName         string
	ObservabilityHook omnillm.ObservabilityHook
}

// OmniLLMAdapter adapts OmniLLM ChatClient to ADK's LLM interface.
type OmniLLMAdapter struct {
	client *omnillm.ChatClient
	model  string
}

// NewOmniLLMAdapter creates a new OmniLLM adapter.
func NewOmniLLMAdapter(providerName, apiKey, modelName string) (*OmniLLMAdapter, error) {
	return NewOmniLLMAdapterWithConfig(OmniLLMAdapterConfig{
		ProviderName: providerName,
		APIKey:       apiKey,
		ModelName:    modelName,
	})
}

// NewOmniLLMAdapterWithConfig creates a new OmniLLM adapter with full configuration.
func NewOmniLLMAdapterWithConfig(cfg OmniLLMAdapterConfig) (*OmniLLMAdapter, error) {
	// For ollama, API key is optional
	if cfg.ProviderName != "ollama" && cfg.APIKey == "" {
		return nil, fmt.Errorf("%s API key is required", cfg.ProviderName)
	}

	// Create OmniLLM config with new Providers slice API
	config := omnillm.ClientConfig{
		Providers: []omnillm.ProviderConfig{
			{
				Provider: omnillm.ProviderName(cfg.ProviderName),
				APIKey:   cfg.APIKey,
			},
		},
		ObservabilityHook: cfg.ObservabilityHook,
	}

	client, err := omnillm.NewClient(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create OmniLLM client: %w", err)
	}

	return &OmniLLMAdapter{
		client: client,
		model:  cfg.ModelName,
	}, nil
}

// Name returns the model name.
func (m *OmniLLMAdapter) Name() string {
	return m.model
}

// GenerateContent implements the LLM interface.
func (m *OmniLLMAdapter) GenerateContent(ctx context.Context, req *model.LLMRequest, stream bool) iter.Seq2[*model.LLMResponse, error] {
	return func(yield func(*model.LLMResponse, error) bool) {
		// Convert ADK request to OmniLLM request
		messages := make([]provider.Message, 0)

		for _, content := range req.Contents {
			var text string
			for _, part := range content.Parts {
				text += part.Text
			}

			var role provider.Role
			switch content.Role {
			case "model", "assistant":
				role = provider.RoleAssistant
			case "system":
				role = provider.RoleSystem
			default:
				role = provider.RoleUser
			}

			messages = append(messages, provider.Message{
				Role:    role,
				Content: text,
			})
		}

		// Create OmniLLM request
		omniReq := &provider.ChatCompletionRequest{
			Model:    m.model,
			Messages: messages,
		}

		// Call OmniLLM API
		resp, err := m.client.CreateChatCompletion(ctx, omniReq)
		if err != nil {
			yield(nil, fmt.Errorf("OmniLLM API error: %w", err))
			return
		}

		// Convert OmniLLM response to ADK response
		if len(resp.Choices) > 0 {
			adkResp := &model.LLMResponse{
				Content: &genai.Content{
					Parts: []*genai.Part{
						{Text: resp.Choices[0].Message.Content},
					},
				},
			}
			yield(adkResp, nil)
		}
	}
}
