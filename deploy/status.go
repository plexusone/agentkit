package deploy

import (
	"time"
)

// DeploymentState represents the current state of a deployment.
type DeploymentState string

// Deployment states.
const (
	// StateUnknown indicates the deployment state is unknown.
	StateUnknown DeploymentState = "unknown"

	// StatePending indicates the deployment is pending.
	StatePending DeploymentState = "pending"

	// StateInProgress indicates the deployment is in progress.
	StateInProgress DeploymentState = "in_progress"

	// StateSucceeded indicates the deployment succeeded.
	StateSucceeded DeploymentState = "succeeded"

	// StateFailed indicates the deployment failed.
	StateFailed DeploymentState = "failed"

	// StateDestroying indicates the deployment is being destroyed.
	StateDestroying DeploymentState = "destroying"

	// StateDestroyed indicates the deployment was destroyed.
	StateDestroyed DeploymentState = "destroyed"
)

// String implements the Stringer interface.
func (s DeploymentState) String() string {
	return string(s)
}

// IsTerminal returns true if this state is a terminal state.
func (s DeploymentState) IsTerminal() bool {
	switch s {
	case StateSucceeded, StateFailed, StateDestroyed:
		return true
	default:
		return false
	}
}

// DeploymentStatus represents the full status of a deployment.
type DeploymentStatus struct {
	// StackName is the unique identifier for this deployment.
	StackName string `json:"stack_name"`

	// State is the current deployment state.
	State DeploymentState `json:"state"`

	// Provider is the name of the provider used.
	Provider string `json:"provider"`

	// Version is the deployment version (e.g., git sha, timestamp).
	Version string `json:"version,omitempty"`

	// StartTime is when the deployment started.
	StartTime time.Time `json:"start_time"`

	// EndTime is when the deployment ended (zero if still running).
	EndTime time.Time `json:"end_time,omitempty"`

	// Duration is the elapsed time.
	Duration time.Duration `json:"duration,omitempty"`

	// Resources lists the deployed resources.
	Resources []Resource `json:"resources,omitempty"`

	// Outputs contains deployment outputs (e.g., URLs, IPs).
	Outputs map[string]string `json:"outputs,omitempty"`

	// Error contains error details if the deployment failed.
	Error string `json:"error,omitempty"`

	// Message provides additional status information.
	Message string `json:"message,omitempty"`

	// Metadata contains provider-specific metadata.
	Metadata map[string]any `json:"metadata,omitempty"`
}

// Resource represents a deployed cloud resource.
type Resource struct {
	// URN is the unique resource name in Pulumi format.
	URN string `json:"urn"`

	// Type is the resource type (e.g., "aws:lightsail:ContainerService").
	Type string `json:"type"`

	// Name is the resource name.
	Name string `json:"name"`

	// ID is the provider-specific resource ID.
	ID string `json:"id,omitempty"`

	// State is the resource state (e.g., "created", "updated", "deleted").
	State string `json:"state"`

	// Outputs contains resource-specific outputs.
	Outputs map[string]any `json:"outputs,omitempty"`
}

// ResourceSummary provides a summary of deployed resources.
type ResourceSummary struct {
	// Total is the total number of resources.
	Total int `json:"total"`

	// Created is the number of resources created.
	Created int `json:"created"`

	// Updated is the number of resources updated.
	Updated int `json:"updated"`

	// Deleted is the number of resources deleted.
	Deleted int `json:"deleted"`

	// Unchanged is the number of unchanged resources.
	Unchanged int `json:"unchanged"`
}

// CalculateSummary computes a summary from the resources.
func (s *DeploymentStatus) CalculateSummary() ResourceSummary {
	summary := ResourceSummary{
		Total: len(s.Resources),
	}

	for _, r := range s.Resources {
		switch r.State {
		case "created":
			summary.Created++
		case "updated":
			summary.Updated++
		case "deleted":
			summary.Deleted++
		default:
			summary.Unchanged++
		}
	}

	return summary
}
