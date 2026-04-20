package deploy

import (
	"fmt"
	"os"
	"strconv"

	"gopkg.in/yaml.v3"
)

// Environment variable names for configuration.
const (
	// EnvDeployProvider is the environment variable for selecting the provider.
	EnvDeployProvider = "AGENTKIT_DEPLOY_PROVIDER"

	// EnvPulumiBackend is the environment variable for the Pulumi backend URL.
	EnvPulumiBackend = "PULUMI_BACKEND_URL"

	// EnvPulumiAccessToken is the environment variable for Pulumi Cloud access.
	EnvPulumiAccessToken = "PULUMI_ACCESS_TOKEN"

	// EnvDryRun is the environment variable to enable dry-run mode.
	EnvDryRun = "AGENTKIT_DEPLOY_DRY_RUN"

	// EnvAutoApprove is the environment variable to auto-approve deployments.
	EnvAutoApprove = "AGENTKIT_DEPLOY_AUTO_APPROVE"
)

// DefaultProvider is the default deployment provider when none is specified.
const DefaultProvider = "lightsail"

// DeployConfig holds the configuration for a deployment.
type DeployConfig struct {
	// Stack contains the core stack configuration.
	Stack StackConfig `yaml:"stack" json:"stack"`

	// Provider is the deployment provider name ("lightsail", "ecs", "cloudrun", "docker").
	Provider string `yaml:"provider" json:"provider"`

	// ProviderConfig contains provider-specific configuration.
	// Use type assertion to access (e.g., cfg.ProviderConfig.(*LightsailConfig)).
	ProviderConfig any `yaml:"provider_config" json:"provider_config"`

	// PulumiBackend is the Pulumi state backend URL.
	// Examples: "file://~/.pulumi", "s3://my-bucket/pulumi", "https://api.pulumi.com"
	PulumiBackend string `yaml:"pulumi_backend" json:"pulumi_backend"`

	// DryRun performs a preview without making changes.
	DryRun bool `yaml:"dry_run" json:"dry_run"`

	// AutoApprove skips confirmation prompts.
	AutoApprove bool `yaml:"auto_approve" json:"auto_approve"`
}

// StackConfig defines the core deployment stack configuration.
type StackConfig struct {
	// Name is the stack name (e.g., "myapp-prod", "invest-advisor-staging").
	Name string `yaml:"name" json:"name"`

	// Project is the Pulumi project name.
	Project string `yaml:"project" json:"project"`

	// Description provides a human-readable description.
	Description string `yaml:"description" json:"description"`

	// Region is the cloud region for deployment.
	Region string `yaml:"region" json:"region"`

	// Tags are resource tags applied to all resources.
	Tags map[string]string `yaml:"tags" json:"tags"`

	// Image is the container image configuration.
	Image ImageConfig `yaml:"image" json:"image"`

	// Resources defines compute resources.
	Resources ResourceConfig `yaml:"resources" json:"resources"`

	// Environment contains environment variables.
	Environment map[string]string `yaml:"environment" json:"environment"`

	// Secrets contains secret references.
	Secrets map[string]SecretRef `yaml:"secrets" json:"secrets"`

	// HealthCheck defines the health check configuration.
	HealthCheck *HealthCheckConfig `yaml:"health_check" json:"health_check"`

	// Scaling defines the scaling configuration.
	Scaling *ScalingConfig `yaml:"scaling" json:"scaling"`
}

// ImageConfig defines container image settings.
type ImageConfig struct {
	// Repository is the container image repository.
	Repository string `yaml:"repository" json:"repository"`

	// Tag is the image tag (default: "latest").
	Tag string `yaml:"tag" json:"tag"`

	// Digest is the image digest for immutable deployments.
	Digest string `yaml:"digest" json:"digest"`

	// Build contains build configuration if building from source.
	Build *BuildConfig `yaml:"build" json:"build"`
}

// BuildConfig defines how to build the container image.
type BuildConfig struct {
	// Context is the build context path.
	Context string `yaml:"context" json:"context"`

	// Dockerfile is the path to the Dockerfile.
	Dockerfile string `yaml:"dockerfile" json:"dockerfile"`

	// Args are build arguments.
	Args map[string]string `yaml:"args" json:"args"`
}

// ResourceConfig defines compute resources.
type ResourceConfig struct {
	// CPU is the CPU allocation (e.g., "0.25", "1", "2").
	CPU string `yaml:"cpu" json:"cpu"`

	// Memory is the memory allocation (e.g., "512", "1024", "2048" in MB).
	Memory string `yaml:"memory" json:"memory"`

	// Port is the container port to expose.
	Port int `yaml:"port" json:"port"`
}

// SecretRef references a secret from a secrets manager.
type SecretRef struct {
	// Source is the secret source ("env", "ssm", "secrets-manager", "vault").
	Source string `yaml:"source" json:"source"`

	// Key is the secret key or path.
	Key string `yaml:"key" json:"key"`

	// Version is the secret version (optional).
	Version string `yaml:"version" json:"version"`
}

// HealthCheckConfig defines health check settings.
type HealthCheckConfig struct {
	// Path is the HTTP health check path.
	Path string `yaml:"path" json:"path"`

	// IntervalSeconds is the check interval.
	IntervalSeconds int `yaml:"interval_seconds" json:"interval_seconds"`

	// TimeoutSeconds is the check timeout.
	TimeoutSeconds int `yaml:"timeout_seconds" json:"timeout_seconds"`

	// HealthyThreshold is the number of successful checks required.
	HealthyThreshold int `yaml:"healthy_threshold" json:"healthy_threshold"`

	// UnhealthyThreshold is the number of failed checks before unhealthy.
	UnhealthyThreshold int `yaml:"unhealthy_threshold" json:"unhealthy_threshold"`
}

// ScalingConfig defines scaling settings.
type ScalingConfig struct {
	// MinInstances is the minimum number of instances.
	MinInstances int `yaml:"min_instances" json:"min_instances"`

	// MaxInstances is the maximum number of instances.
	MaxInstances int `yaml:"max_instances" json:"max_instances"`

	// TargetCPUPercent is the target CPU utilization for scaling.
	TargetCPUPercent int `yaml:"target_cpu_percent" json:"target_cpu_percent"`
}

// LoadDeployConfig loads a deployment configuration from a YAML file.
// Environment variables override file values (precedence: env > file > defaults).
func LoadDeployConfig(path string) (*DeployConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	cfg := &DeployConfig{}
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	// Apply environment variable overrides
	applyEnvOverrides(cfg)

	// Apply defaults
	applyDefaults(cfg)

	return cfg, nil
}

// NewDeployConfig creates a new DeployConfig with defaults applied.
func NewDeployConfig() *DeployConfig {
	cfg := &DeployConfig{}
	applyDefaults(cfg)
	return cfg
}

// applyEnvOverrides applies environment variable overrides to the config.
func applyEnvOverrides(cfg *DeployConfig) {
	// Provider override
	if provider := os.Getenv(EnvDeployProvider); provider != "" {
		cfg.Provider = provider
	}

	// Pulumi backend override
	if backend := os.Getenv(EnvPulumiBackend); backend != "" {
		cfg.PulumiBackend = backend
	}

	// Dry run override
	if dryRun := os.Getenv(EnvDryRun); dryRun != "" {
		cfg.DryRun = parseBool(dryRun)
	}

	// Auto approve override
	if autoApprove := os.Getenv(EnvAutoApprove); autoApprove != "" {
		cfg.AutoApprove = parseBool(autoApprove)
	}
}

// applyDefaults applies default values to the config.
func applyDefaults(cfg *DeployConfig) {
	if cfg.Provider == "" {
		cfg.Provider = DefaultProvider
	}

	if cfg.Stack.Image.Tag == "" {
		cfg.Stack.Image.Tag = "latest"
	}

	if cfg.Stack.Resources.Port == 0 {
		cfg.Stack.Resources.Port = 8080
	}

	if cfg.Stack.Resources.CPU == "" {
		cfg.Stack.Resources.CPU = "0.25"
	}

	if cfg.Stack.Resources.Memory == "" {
		cfg.Stack.Resources.Memory = "512"
	}
}

// Validate checks that the configuration is valid.
func (c *DeployConfig) Validate() error {
	if c.Stack.Name == "" {
		return NewConfigError("stack.name", ErrInvalidConfig)
	}

	if c.Stack.Project == "" {
		return NewConfigError("stack.project", ErrInvalidConfig)
	}

	if c.Stack.Image.Repository == "" && c.Stack.Image.Build == nil {
		return NewConfigError("stack.image", fmt.Errorf("%w: repository or build required", ErrInvalidConfig))
	}

	return nil
}

// GetProviderName returns the effective provider name, considering env overrides.
func (c *DeployConfig) GetProviderName() string {
	if provider := os.Getenv(EnvDeployProvider); provider != "" {
		return provider
	}
	if c.Provider != "" {
		return c.Provider
	}
	return DefaultProvider
}

// parseBool parses a boolean string (case-insensitive).
func parseBool(s string) bool {
	b, err := strconv.ParseBool(s)
	if err != nil {
		return false
	}
	return b
}
