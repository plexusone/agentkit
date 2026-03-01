// Package agent provides base agent functionality for building AI agents.
package agent

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"google.golang.org/adk/agent"
	"google.golang.org/adk/model"

	"github.com/plexusone/agentkit/config"
	"github.com/plexusone/agentkit/llm"
)

// BaseAgent provides common functionality for all agents.
type BaseAgent struct {
	Cfg          *config.Config
	Client       *http.Client
	Model        model.LLM
	ModelFactory *llm.ModelFactory
	Name         string
}

// NewBaseAgent creates a new base agent with LLM initialization.
func NewBaseAgent(cfg *config.Config, name string, timeoutSec int) (*BaseAgent, error) {
	ctx := context.Background()

	// Create model using factory
	modelFactory := llm.NewModelFactory(cfg)
	llmModel, err := modelFactory.CreateModel(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create model: %w", err)
	}

	return &BaseAgent{
		Cfg:          cfg,
		Client:       &http.Client{Timeout: time.Duration(timeoutSec) * time.Second},
		Model:        llmModel,
		ModelFactory: modelFactory,
		Name:         name,
	}, nil
}

// NewBaseAgentSecure creates a base agent with VaultGuard security checks.
func NewBaseAgentSecure(ctx context.Context, name string, timeoutSec int, opts ...config.SecureConfigOption) (*BaseAgent, *config.SecureConfig, error) {
	// Load secure config
	secCfg, err := config.LoadSecureConfig(ctx, opts...)
	if err != nil {
		return nil, nil, fmt.Errorf("security check failed: %w", err)
	}

	// Create model using factory
	modelFactory := llm.NewModelFactory(secCfg.Config)
	llmModel, err := modelFactory.CreateModel(ctx)
	if err != nil {
		_ = secCfg.Close()
		return nil, nil, fmt.Errorf("failed to create model: %w", err)
	}

	ba := &BaseAgent{
		Cfg:          secCfg.Config,
		Client:       &http.Client{Timeout: time.Duration(timeoutSec) * time.Second},
		Model:        llmModel,
		ModelFactory: modelFactory,
		Name:         name,
	}

	return ba, secCfg, nil
}

// Close cleans up resources.
func (ba *BaseAgent) Close() error {
	if ba.ModelFactory != nil {
		return ba.ModelFactory.Close()
	}
	return nil
}

// GetProviderInfo returns information about the LLM provider.
func (ba *BaseAgent) GetProviderInfo() string {
	return ba.ModelFactory.GetProviderInfo()
}

// FetchURL fetches content from a URL with proper error handling.
func (ba *BaseAgent) FetchURL(ctx context.Context, url string, maxSizeMB int) (string, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("User-Agent", fmt.Sprintf("AgentKit/%s", ba.Name))

	resp, err := ba.Client.Do(req) //nolint:gosec // G704: URL provided by SDK user
	if err != nil {
		return "", fmt.Errorf("failed to fetch URL: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("HTTP %d: %s", resp.StatusCode, resp.Status)
	}

	// Limit response size
	maxBytes := int64(maxSizeMB * 1024 * 1024)
	limitedReader := io.LimitReader(resp.Body, maxBytes)
	body, err := io.ReadAll(limitedReader)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	return string(body), nil
}

// LogInfo logs an informational message with agent context.
func (ba *BaseAgent) LogInfo(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	log.Printf("[%s] %s", ba.Name, msg)
}

// LogError logs an error message with agent context.
func (ba *BaseAgent) LogError(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	log.Printf("[%s] ERROR: %s", ba.Name, msg)
}

// Wrapper wraps common agent initialization patterns.
type Wrapper struct {
	*BaseAgent
	ADKAgent agent.Agent
}

// NewWrapper creates a wrapper with both base functionality and ADK agent.
func NewWrapper(base *BaseAgent, adkAgent agent.Agent) *Wrapper {
	return &Wrapper{
		BaseAgent: base,
		ADKAgent:  adkAgent,
	}
}
