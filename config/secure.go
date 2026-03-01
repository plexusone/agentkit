package config

import (
	"context"
	"fmt"

	"github.com/plexusone/vaultguard"
)

// SecureConfig wraps Config with VaultGuard for secure credential access
// and optionally integrates with OmniVault for unified secret management.
type SecureConfig struct {
	*Config
	vault   *vaultguard.SecureVault
	secrets *SecretsClient
}

// LoadSecureConfig loads configuration with VaultGuard security checks.
// It enforces security policies based on the environment (local or cloud).
// Optionally integrates with OmniVault for unified secret management.
func LoadSecureConfig(ctx context.Context, opts ...SecureConfigOption) (*SecureConfig, error) {
	options := &secureConfigOptions{
		policy: nil, // Use default policy
	}
	for _, opt := range opts {
		opt(options)
	}

	// Create VaultGuard configuration
	vgConfig := &vaultguard.Config{
		Policy: options.policy,
	}

	// Create secure vault
	sv, err := vaultguard.New(vgConfig)
	if err != nil {
		return nil, fmt.Errorf("security check failed: %w", err)
	}

	// Create OmniVault secrets client if configured
	var secrets *SecretsClient
	if options.secretsConfig != nil {
		secrets, err = NewSecretsClient(*options.secretsConfig)
		if err != nil {
			return nil, fmt.Errorf("creating secrets client: %w", err)
		}
	}

	// Load base config
	cfg := LoadConfig()

	sc := &SecureConfig{
		Config:  cfg,
		vault:   sv,
		secrets: secrets,
	}

	// Load sensitive credentials (OmniVault first, then VaultGuard fallback)
	sc.loadSecureCredentials(ctx)

	return sc, nil
}

// loadSecureCredentials loads API keys from OmniVault (if configured) or VaultGuard.
// Missing credentials are silently skipped as they are optional.
// Resolution order: OmniVault → VaultGuard → Environment variables
func (sc *SecureConfig) loadSecureCredentials(ctx context.Context) {
	// Load LLM API key if not set
	if sc.LLMAPIKey == "" {
		sc.LLMAPIKey = sc.getSecureValue(ctx, "LLM_API_KEY")
	}

	// Load provider-specific keys
	if sc.GeminiAPIKey == "" {
		sc.GeminiAPIKey = sc.getSecureValue(ctx, "GEMINI_API_KEY")
		if sc.GeminiAPIKey == "" {
			sc.GeminiAPIKey = sc.getSecureValue(ctx, "GOOGLE_API_KEY")
		}
	}

	if sc.ClaudeAPIKey == "" {
		sc.ClaudeAPIKey = sc.getSecureValue(ctx, "CLAUDE_API_KEY")
		if sc.ClaudeAPIKey == "" {
			sc.ClaudeAPIKey = sc.getSecureValue(ctx, "ANTHROPIC_API_KEY")
		}
	}

	if sc.OpenAIAPIKey == "" {
		sc.OpenAIAPIKey = sc.getSecureValue(ctx, "OPENAI_API_KEY")
	}

	if sc.XAIAPIKey == "" {
		sc.XAIAPIKey = sc.getSecureValue(ctx, "XAI_API_KEY")
	}

	// Load search API keys
	if sc.SerperAPIKey == "" {
		sc.SerperAPIKey = sc.getSecureValue(ctx, "SERPER_API_KEY")
	}

	if sc.SerpAPIKey == "" {
		sc.SerpAPIKey = sc.getSecureValue(ctx, "SERPAPI_API_KEY")
	}

	// Load observability API key
	if sc.ObservabilityAPIKey == "" {
		sc.ObservabilityAPIKey = sc.getSecureValue(ctx, "OBSERVABILITY_API_KEY")
		if sc.ObservabilityAPIKey == "" {
			sc.ObservabilityAPIKey = sc.getSecureValue(ctx, "OPIK_API_KEY")
		}
	}

	// Update LLMAPIKey based on provider if still not set
	if sc.LLMAPIKey == "" {
		switch sc.LLMProvider {
		case "gemini":
			sc.LLMAPIKey = sc.GeminiAPIKey
		case "claude":
			sc.LLMAPIKey = sc.ClaudeAPIKey
		case "openai":
			sc.LLMAPIKey = sc.OpenAIAPIKey
		case "xai":
			sc.LLMAPIKey = sc.XAIAPIKey
		}
	}
}

// getSecureValue retrieves a value from OmniVault first, then VaultGuard.
func (sc *SecureConfig) getSecureValue(ctx context.Context, name string) string {
	// Try OmniVault first if configured
	if sc.secrets != nil {
		if value, err := sc.secrets.Get(ctx, name); err == nil && value != "" {
			return value
		}
	}

	// Fall back to VaultGuard
	if value, err := sc.GetCredential(ctx, name); err == nil && value != "" {
		return value
	}

	return ""
}

// GetCredential retrieves a credential from the secure vault.
func (sc *SecureConfig) GetCredential(ctx context.Context, name string) (string, error) {
	return sc.vault.GetValue(ctx, name)
}

// GetRequiredCredentials retrieves multiple credentials, failing if any are missing.
func (sc *SecureConfig) GetRequiredCredentials(ctx context.Context, names ...string) (map[string]string, error) {
	result := make(map[string]string)
	for _, name := range names {
		value, err := sc.vault.GetValue(ctx, name)
		if err != nil {
			return nil, fmt.Errorf("required credential %s not found: %w", name, err)
		}
		if value == "" {
			return nil, fmt.Errorf("required credential %s is empty", name)
		}
		result[name] = value
	}
	return result, nil
}

// Environment returns the detected deployment environment.
func (sc *SecureConfig) Environment() vaultguard.Environment {
	return sc.vault.Environment()
}

// SecurityResult returns the security assessment result.
func (sc *SecureConfig) SecurityResult() *vaultguard.SecurityResult {
	return sc.vault.SecurityResult()
}

// Close cleans up resources.
func (sc *SecureConfig) Close() error {
	var errs []error
	if sc.secrets != nil {
		if err := sc.secrets.Close(); err != nil {
			errs = append(errs, err)
		}
	}
	if sc.vault != nil {
		if err := sc.vault.Close(); err != nil {
			errs = append(errs, err)
		}
	}
	if len(errs) > 0 {
		return errs[0]
	}
	return nil
}

// SecureConfigOption configures secure config loading.
type SecureConfigOption func(*secureConfigOptions)

type secureConfigOptions struct {
	policy        *vaultguard.Policy
	secretsConfig *SecretsConfig
}

// WithPolicy sets a custom security policy.
func WithPolicy(policy *vaultguard.Policy) SecureConfigOption {
	return func(o *secureConfigOptions) {
		o.policy = policy
	}
}

// WithDevPolicy uses a permissive development policy.
func WithDevPolicy() SecureConfigOption {
	return func(o *secureConfigOptions) {
		o.policy = vaultguard.DevelopmentPolicy()
	}
}

// WithStrictPolicy uses a strict security policy.
func WithStrictPolicy() SecureConfigOption {
	return func(o *secureConfigOptions) {
		o.policy = vaultguard.StrictPolicy()
	}
}

// WithSecretsProvider configures OmniVault as the secrets provider.
// When set, secrets are loaded from OmniVault first, with fallback to VaultGuard.
func WithSecretsProvider(cfg SecretsConfig) SecureConfigOption {
	return func(o *secureConfigOptions) {
		o.secretsConfig = &cfg
	}
}

// WithAWSSecretsManager configures AWS Secrets Manager as the secrets provider.
// This is a convenience function for AWS deployments.
func WithAWSSecretsManager(prefix, region string) SecureConfigOption {
	return func(o *secureConfigOptions) {
		o.secretsConfig = &SecretsConfig{
			Provider:      SecretsProviderAWSSM,
			Prefix:        prefix,
			Region:        region,
			FallbackToEnv: true,
		}
	}
}

// WithAutoSecretsProvider uses DefaultSecretsConfig to auto-detect the provider.
// In AWS environments, this will use AWS Secrets Manager; otherwise, env vars.
func WithAutoSecretsProvider() SecureConfigOption {
	return func(o *secureConfigOptions) {
		cfg := DefaultSecretsConfig()
		o.secretsConfig = &cfg
	}
}
