// Package local provides an embedded local mode for running agents in-process.
package local

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/plexusone/omnillm"
	"github.com/plexusone/omnillm/provider"
)

// OmniLLMClient implements LLMClient using omnillm.ChatClient.
type OmniLLMClient struct {
	client   *omnillm.ChatClient
	model    string
	provider omnillm.ProviderName
}

// OmniLLMConfig holds configuration for creating an OmniLLMClient.
type OmniLLMConfig struct {
	// Provider is the LLM provider name (anthropic, openai, gemini, xai, ollama).
	Provider string
	// APIKey is the API key for the provider (not needed for ollama).
	APIKey string //nolint:gosec // G117: Config needs API key field
	// Model is the model ID or canonical name (haiku, sonnet, opus).
	Model string
	// BaseURL is an optional custom base URL for the provider.
	BaseURL string
}

// NewOmniLLMClient creates a new LLMClient backed by omnillm.
func NewOmniLLMClient(cfg OmniLLMConfig) (*OmniLLMClient, error) {
	providerName := ProviderNameFromString(cfg.Provider)

	// Resolve canonical model name to provider-specific model ID
	model := MapCanonicalModel(cfg.Model, providerName)

	providerCfg := omnillm.ProviderConfig{
		Provider: providerName,
		APIKey:   cfg.APIKey,
		BaseURL:  cfg.BaseURL,
	}

	client, err := omnillm.NewClient(omnillm.ClientConfig{
		Providers: []omnillm.ProviderConfig{providerCfg},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create omnillm client: %w", err)
	}

	return &OmniLLMClient{
		client:   client,
		model:    model,
		provider: providerName,
	}, nil
}

// Complete generates a completion for the given messages.
func (c *OmniLLMClient) Complete(ctx context.Context, messages []Message, tools []ToolDefinition) (*CompletionResponse, error) {
	// Convert local messages to omnillm messages
	omniMessages := make([]provider.Message, len(messages))
	for i, msg := range messages {
		omniMessages[i] = convertToOmniMessage(msg)
	}

	// Convert tool definitions to omnillm tools
	omniTools := make([]provider.Tool, len(tools))
	for i, tool := range tools {
		omniTools[i] = convertToOmniTool(tool)
	}

	req := &provider.ChatCompletionRequest{
		Model:    c.model,
		Messages: omniMessages,
		Tools:    omniTools,
	}

	resp, err := c.client.CreateChatCompletion(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("completion failed: %w", err)
	}

	return convertFromOmniResponse(resp), nil
}

// Close closes the underlying client.
func (c *OmniLLMClient) Close() error {
	return c.client.Close()
}

// Model returns the current model ID.
func (c *OmniLLMClient) Model() string {
	return c.model
}

// Provider returns the provider name.
func (c *OmniLLMClient) Provider() omnillm.ProviderName {
	return c.provider
}

// convertToOmniMessage converts a local Message to provider.Message.
func convertToOmniMessage(msg Message) provider.Message {
	omniMsg := provider.Message{
		Role:    convertRole(msg.Role),
		Content: msg.Content,
	}
	if msg.Name != "" {
		omniMsg.Name = &msg.Name
	}
	if msg.ToolID != "" {
		omniMsg.ToolCallID = &msg.ToolID
	}
	return omniMsg
}

// convertRole converts a role string to provider.Role.
func convertRole(role string) provider.Role {
	switch role {
	case "system":
		return provider.RoleSystem
	case "user":
		return provider.RoleUser
	case "assistant":
		return provider.RoleAssistant
	case "tool":
		return provider.RoleTool
	default:
		return provider.Role(role)
	}
}

// convertToOmniTool converts a local ToolDefinition to provider.Tool.
func convertToOmniTool(tool ToolDefinition) provider.Tool {
	return provider.Tool{
		Type: "function",
		Function: provider.ToolSpec{
			Name:        tool.Name,
			Description: tool.Description,
			Parameters:  tool.Parameters,
		},
	}
}

// convertFromOmniResponse converts provider.ChatCompletionResponse to local CompletionResponse.
func convertFromOmniResponse(resp *provider.ChatCompletionResponse) *CompletionResponse {
	result := &CompletionResponse{
		Done: true,
	}

	if len(resp.Choices) == 0 {
		return result
	}

	choice := resp.Choices[0]
	result.Content = choice.Message.Content

	// Check finish reason to determine if we're done
	if choice.FinishReason != nil {
		switch *choice.FinishReason {
		case "tool_calls", "tool_use":
			result.Done = false
		}
	}

	// Convert tool calls
	if len(choice.Message.ToolCalls) > 0 {
		result.ToolCalls = make([]ToolCall, len(choice.Message.ToolCalls))
		for i, tc := range choice.Message.ToolCalls {
			args := parseToolArguments(tc.Function.Arguments)
			result.ToolCalls[i] = ToolCall{
				ID:        tc.ID,
				Name:      tc.Function.Name,
				Arguments: args,
			}
		}
		result.Done = false
	}

	return result
}

// parseToolArguments parses JSON arguments string to map.
func parseToolArguments(argsStr string) map[string]any {
	var args map[string]any
	if err := json.Unmarshal([]byte(argsStr), &args); err != nil {
		// Return empty map if parsing fails
		return make(map[string]any)
	}
	return args
}

// NewOmniLLMClientFromConfig creates an OmniLLMClient from the existing LLMConfig type.
func NewOmniLLMClientFromConfig(cfg LLMConfig) (*OmniLLMClient, error) {
	return NewOmniLLMClient(OmniLLMConfig{
		Provider: cfg.Provider,
		APIKey:   cfg.APIKey,
		Model:    cfg.Model,
		BaseURL:  cfg.BaseURL,
	})
}
