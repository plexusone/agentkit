// Package config provides OmniVault integration for unified secret management.
package config

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/plexusone/omnivault"
	"github.com/plexusone/omnivault/vault"
)

// SecretsProvider specifies the secrets backend to use.
type SecretsProvider string

// Known secrets providers.
const (
	// SecretsProviderEnv uses environment variables (default, local dev).
	SecretsProviderEnv SecretsProvider = "env"

	// SecretsProviderAWSSM uses AWS Secrets Manager.
	SecretsProviderAWSSM SecretsProvider = "aws-sm"

	// SecretsProviderAWSSSM uses AWS Systems Manager Parameter Store.
	SecretsProviderAWSSSM SecretsProvider = "aws-ssm"

	// SecretsProviderMemory uses in-memory storage (testing).
	SecretsProviderMemory SecretsProvider = "memory"
)

// SecretsConfig holds configuration for OmniVault secrets management.
type SecretsConfig struct {
	// Provider specifies which secrets backend to use.
	// Default: "env" (environment variables)
	Provider SecretsProvider

	// Prefix is prepended to secret paths (e.g., "stats-agent/" for AWS).
	// For AWS Secrets Manager, secrets are stored as "{prefix}{name}".
	Prefix string

	// Region is the AWS region (for aws-sm, aws-ssm providers).
	Region string

	// CustomVault allows injecting a custom vault implementation.
	// When set, this takes precedence over Provider.
	CustomVault vault.Vault

	// Logger is an optional structured logger.
	Logger *slog.Logger

	// FallbackToEnv enables falling back to environment variables
	// when a secret is not found in the configured provider.
	// Default: true
	FallbackToEnv bool
}

// SecretsClient wraps OmniVault with agentkit-specific functionality.
type SecretsClient struct {
	client        *omnivault.Client
	config        SecretsConfig
	fallbackToEnv bool
}

// NewSecretsClient creates a new secrets client with the given configuration.
func NewSecretsClient(cfg SecretsConfig) (*SecretsClient, error) {
	// Default to env provider
	if cfg.Provider == "" {
		cfg.Provider = SecretsProviderEnv
	}

	// Default to fallback enabled
	if !cfg.FallbackToEnv {
		cfg.FallbackToEnv = true
	}

	// Map SecretsProvider to omnivault.ProviderName
	var provider omnivault.ProviderName
	switch cfg.Provider {
	case SecretsProviderEnv:
		provider = omnivault.ProviderEnv
	case SecretsProviderAWSSM:
		provider = omnivault.ProviderAWSSecretsManager
	case SecretsProviderAWSSSM:
		provider = omnivault.ProviderAWSParameterStore
	case SecretsProviderMemory:
		provider = omnivault.ProviderMemory
	default:
		// Allow passing through other omnivault providers directly
		provider = omnivault.ProviderName(cfg.Provider)
	}

	// Build omnivault config
	ovConfig := omnivault.Config{
		Provider:    provider,
		CustomVault: cfg.CustomVault,
		Logger:      cfg.Logger,
	}

	// Add provider-specific config for AWS
	if cfg.Region != "" && (cfg.Provider == SecretsProviderAWSSM || cfg.Provider == SecretsProviderAWSSSM) {
		ovConfig.Extra = map[string]any{
			"region": cfg.Region,
		}
	}

	client, err := omnivault.NewClient(ovConfig)
	if err != nil {
		return nil, fmt.Errorf("creating secrets client: %w", err)
	}

	return &SecretsClient{
		client:        client,
		config:        cfg,
		fallbackToEnv: cfg.FallbackToEnv,
	}, nil
}

// Get retrieves a secret by name.
// If a prefix is configured, it's prepended to the name.
// Falls back to environment variables if configured and secret not found.
func (sc *SecretsClient) Get(ctx context.Context, name string) (string, error) {
	// Build the full path with prefix
	path := name
	if sc.config.Prefix != "" {
		path = sc.config.Prefix + name
	}

	// Try the primary provider
	value, err := sc.client.GetValue(ctx, path)
	if err == nil && value != "" {
		return value, nil
	}

	// Try without prefix if prefixed lookup failed
	if sc.config.Prefix != "" && err != nil {
		value, err = sc.client.GetValue(ctx, name)
		if err == nil && value != "" {
			return value, nil
		}
	}

	// Fallback to environment variables
	if sc.fallbackToEnv && sc.config.Provider != SecretsProviderEnv {
		if envValue := os.Getenv(name); envValue != "" {
			return envValue, nil
		}
	}

	if err != nil {
		return "", fmt.Errorf("secret %s not found: %w", name, err)
	}
	return "", fmt.Errorf("secret %s not found", name)
}

// GetField retrieves a specific field from a JSON secret.
// Useful for AWS Secrets Manager secrets with multiple key-value pairs.
func (sc *SecretsClient) GetField(ctx context.Context, name, field string) (string, error) {
	path := name
	if sc.config.Prefix != "" {
		path = sc.config.Prefix + name
	}

	value, err := sc.client.GetField(ctx, path, field)
	if err == nil && value != "" {
		return value, nil
	}

	// Fallback to environment variable using field name
	if sc.fallbackToEnv {
		if envValue := os.Getenv(field); envValue != "" {
			return envValue, nil
		}
	}

	if err != nil {
		return "", fmt.Errorf("secret field %s.%s not found: %w", name, field, err)
	}
	return "", fmt.Errorf("secret field %s.%s not found", name, field)
}

// Exists checks if a secret exists.
func (sc *SecretsClient) Exists(ctx context.Context, name string) bool {
	path := name
	if sc.config.Prefix != "" {
		path = sc.config.Prefix + name
	}

	exists, err := sc.client.Exists(ctx, path)
	if err != nil {
		return false
	}
	return exists
}

// Provider returns the configured provider name.
func (sc *SecretsClient) Provider() SecretsProvider {
	return sc.config.Provider
}

// Close releases resources.
func (sc *SecretsClient) Close() error {
	if sc.client != nil {
		return sc.client.Close()
	}
	return nil
}

// DefaultSecretsConfig returns a SecretsConfig based on environment detection.
// It auto-detects the appropriate provider based on the runtime environment.
func DefaultSecretsConfig() SecretsConfig {
	cfg := SecretsConfig{
		Provider:      SecretsProviderEnv,
		FallbackToEnv: true,
	}

	// Check for explicit provider setting
	if provider := os.Getenv("SECRETS_PROVIDER"); provider != "" {
		cfg.Provider = SecretsProvider(provider)
	}

	// Check for AWS environment indicators
	if cfg.Provider == SecretsProviderEnv {
		if isAWSEnvironment() {
			cfg.Provider = SecretsProviderAWSSM
		}
	}

	// Get prefix from environment
	if prefix := os.Getenv("SECRETS_PREFIX"); prefix != "" {
		cfg.Prefix = prefix
		// Ensure prefix ends with /
		if !strings.HasSuffix(cfg.Prefix, "/") {
			cfg.Prefix += "/"
		}
	}

	// Get region from environment
	if region := os.Getenv("AWS_REGION"); region != "" {
		cfg.Region = region
	} else if region := os.Getenv("AWS_DEFAULT_REGION"); region != "" {
		cfg.Region = region
	}

	return cfg
}

// isAWSEnvironment checks if we're running in an AWS environment.
func isAWSEnvironment() bool {
	// Check for ECS task metadata
	if os.Getenv("ECS_CONTAINER_METADATA_URI_V4") != "" {
		return true
	}
	if os.Getenv("ECS_CONTAINER_METADATA_URI") != "" {
		return true
	}

	// Check for Lambda
	if os.Getenv("AWS_LAMBDA_FUNCTION_NAME") != "" {
		return true
	}

	// Check for EC2 instance metadata availability
	// (This is a lightweight check, not a full metadata query)
	if os.Getenv("AWS_EXECUTION_ENV") != "" {
		return true
	}

	return false
}
