package deploy

import (
	"errors"
	"fmt"
)

// Sentinel errors for common deployment failures.
var (
	// ErrProviderNotFound is returned when a requested provider is not registered.
	ErrProviderNotFound = errors.New("deploy: provider not found")

	// ErrProviderAlreadyRegistered is returned when attempting to register a duplicate provider.
	ErrProviderAlreadyRegistered = errors.New("deploy: provider already registered")

	// ErrInvalidConfig is returned when configuration is invalid.
	ErrInvalidConfig = errors.New("deploy: invalid configuration")

	// ErrDeploymentFailed is returned when a deployment fails.
	ErrDeploymentFailed = errors.New("deploy: deployment failed")

	// ErrDeploymentNotFound is returned when a deployment stack is not found.
	ErrDeploymentNotFound = errors.New("deploy: deployment not found")

	// ErrOperationCanceled is returned when an operation is canceled via context.
	ErrOperationCanceled = errors.New("deploy: operation canceled")

	// ErrResourceQuotaExceeded is returned when cloud resource quotas are exceeded.
	ErrResourceQuotaExceeded = errors.New("deploy: resource quota exceeded")

	// ErrAuthenticationFailed is returned when cloud authentication fails.
	ErrAuthenticationFailed = errors.New("deploy: authentication failed")

	// ErrPulumiNotConfigured is returned when Pulumi backend is not configured.
	ErrPulumiNotConfigured = errors.New("deploy: pulumi backend not configured")
)

// ProviderError wraps an error with provider context.
type ProviderError struct {
	Provider string
	Op       string
	Err      error
}

// Error implements the error interface.
func (e *ProviderError) Error() string {
	return fmt.Sprintf("deploy: provider %s: %s: %v", e.Provider, e.Op, e.Err)
}

// Unwrap returns the underlying error.
func (e *ProviderError) Unwrap() error {
	return e.Err
}

// NewProviderError creates a new ProviderError.
func NewProviderError(provider, op string, err error) *ProviderError {
	return &ProviderError{
		Provider: provider,
		Op:       op,
		Err:      err,
	}
}

// ConfigError wraps an error with configuration context.
type ConfigError struct {
	Field string
	Err   error
}

// Error implements the error interface.
func (e *ConfigError) Error() string {
	return fmt.Sprintf("deploy: config field %q: %v", e.Field, e.Err)
}

// Unwrap returns the underlying error.
func (e *ConfigError) Unwrap() error {
	return e.Err
}

// NewConfigError creates a new ConfigError.
func NewConfigError(field string, err error) *ConfigError {
	return &ConfigError{
		Field: field,
		Err:   err,
	}
}

// DeploymentError wraps an error with deployment context.
type DeploymentError struct {
	StackName string
	State     DeploymentState
	Err       error
}

// Error implements the error interface.
func (e *DeploymentError) Error() string {
	return fmt.Sprintf("deploy: stack %q (state: %s): %v", e.StackName, e.State, e.Err)
}

// Unwrap returns the underlying error.
func (e *DeploymentError) Unwrap() error {
	return e.Err
}

// NewDeploymentError creates a new DeploymentError.
func NewDeploymentError(stackName string, state DeploymentState, err error) *DeploymentError {
	return &DeploymentError{
		StackName: stackName,
		State:     state,
		Err:       err,
	}
}
