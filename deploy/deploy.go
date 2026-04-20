// Package deploy provides a deployment provider system for agentkit that allows
// any omniagent implementation to deploy to multiple cloud targets via environment
// variable configuration.
package deploy

import (
	"context"
)

// Provider defines the interface for deployment providers.
// Implementations must be thread-safe.
type Provider interface {
	// Deploy deploys the configuration to the target platform.
	// Returns a DeploymentStatus on success or error on failure.
	Deploy(ctx context.Context, cfg *DeployConfig) (*DeploymentStatus, error)

	// Status returns the current status of a deployment.
	Status(ctx context.Context, stackName string) (*DeploymentStatus, error)

	// Destroy removes all resources associated with a deployment.
	Destroy(ctx context.Context, stackName string) error

	// Name returns the unique identifier for this provider.
	Name() string

	// Capabilities returns the capabilities of this provider.
	Capabilities() Capabilities

	// Close releases any resources held by the provider.
	Close() error
}

// Capabilities describes what features a provider supports.
type Capabilities struct {
	// AutoScaling indicates the provider supports auto-scaling.
	AutoScaling bool `json:"auto_scaling"`

	// CustomDomain indicates the provider supports custom domain mapping.
	CustomDomain bool `json:"custom_domain"`

	// HTTPS indicates the provider supports HTTPS termination.
	HTTPS bool `json:"https"`

	// VPC indicates the provider supports VPC networking.
	VPC bool `json:"vpc"`

	// SecretsIntegration indicates the provider integrates with secrets managers.
	SecretsIntegration bool `json:"secrets_integration"`

	// Preview indicates the provider supports deployment previews.
	Preview bool `json:"preview"`

	// Rollback indicates the provider supports deployment rollback.
	Rollback bool `json:"rollback"`

	// MaxMemoryMB is the maximum memory allocation in MB.
	MaxMemoryMB int `json:"max_memory_mb"`
}

// PreviewProvider is an optional interface for providers that support preview deployments.
type PreviewProvider interface {
	Provider

	// Preview shows what would be deployed without making changes.
	Preview(ctx context.Context, cfg *DeployConfig) (*PreviewResult, error)
}

// RollbackProvider is an optional interface for providers that support rollback.
type RollbackProvider interface {
	Provider

	// Rollback reverts to a previous deployment version.
	Rollback(ctx context.Context, stackName string, targetVersion string) (*DeploymentStatus, error)

	// ListVersions returns available versions for rollback.
	ListVersions(ctx context.Context, stackName string) ([]string, error)
}

// LogProvider is an optional interface for providers that support log streaming.
type LogProvider interface {
	Provider

	// StreamLogs streams deployment logs to the provided channel.
	StreamLogs(ctx context.Context, stackName string, logs chan<- LogEntry) error
}

// PreviewResult contains the result of a deployment preview.
type PreviewResult struct {
	// StackName is the name of the stack.
	StackName string `json:"stack_name"`

	// Creates lists resources that would be created.
	Creates []ResourcePreview `json:"creates,omitempty"`

	// Updates lists resources that would be updated.
	Updates []ResourcePreview `json:"updates,omitempty"`

	// Deletes lists resources that would be deleted.
	Deletes []ResourcePreview `json:"deletes,omitempty"`

	// Summary provides a human-readable summary.
	Summary string `json:"summary"`
}

// ResourcePreview describes a resource change in a preview.
type ResourcePreview struct {
	// URN is the unique resource name.
	URN string `json:"urn"`

	// Type is the resource type.
	Type string `json:"type"`

	// Name is the resource name.
	Name string `json:"name"`

	// Details provides additional information about the change.
	Details map[string]any `json:"details,omitempty"`
}

// LogEntry represents a single log entry from a deployment.
type LogEntry struct {
	// Timestamp is the time the log was generated.
	Timestamp string `json:"timestamp"`

	// Level is the log level (info, warn, error).
	Level string `json:"level"`

	// Message is the log message.
	Message string `json:"message"`

	// Source identifies where the log came from.
	Source string `json:"source,omitempty"`
}
