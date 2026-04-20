package deploy

import (
	"testing"
)

func TestDeploymentState(t *testing.T) {
	tests := []struct {
		state      DeploymentState
		str        string
		isTerminal bool
	}{
		{StateUnknown, "unknown", false},
		{StatePending, "pending", false},
		{StateInProgress, "in_progress", false},
		{StateSucceeded, "succeeded", true},
		{StateFailed, "failed", true},
		{StateDestroying, "destroying", false},
		{StateDestroyed, "destroyed", true},
	}

	for _, tt := range tests {
		t.Run(tt.str, func(t *testing.T) {
			if tt.state.String() != tt.str {
				t.Errorf("expected String() = %q, got %q", tt.str, tt.state.String())
			}
			if tt.state.IsTerminal() != tt.isTerminal {
				t.Errorf("expected IsTerminal() = %v, got %v", tt.isTerminal, tt.state.IsTerminal())
			}
		})
	}
}

func TestCalculateSummary(t *testing.T) {
	status := &DeploymentStatus{
		StackName: "test",
		Resources: []Resource{
			{Name: "r1", State: "created"},
			{Name: "r2", State: "created"},
			{Name: "r3", State: "updated"},
			{Name: "r4", State: "deleted"},
			{Name: "r5", State: "unchanged"},
			{Name: "r6", State: "same"},
		},
	}

	summary := status.CalculateSummary()

	if summary.Total != 6 {
		t.Errorf("expected Total = 6, got %d", summary.Total)
	}
	if summary.Created != 2 {
		t.Errorf("expected Created = 2, got %d", summary.Created)
	}
	if summary.Updated != 1 {
		t.Errorf("expected Updated = 1, got %d", summary.Updated)
	}
	if summary.Deleted != 1 {
		t.Errorf("expected Deleted = 1, got %d", summary.Deleted)
	}
	if summary.Unchanged != 2 {
		t.Errorf("expected Unchanged = 2, got %d", summary.Unchanged)
	}
}

func TestEmptyCalculateSummary(t *testing.T) {
	status := &DeploymentStatus{
		StackName: "empty",
		Resources: nil,
	}

	summary := status.CalculateSummary()

	if summary.Total != 0 {
		t.Errorf("expected Total = 0, got %d", summary.Total)
	}
	if summary.Created != 0 {
		t.Errorf("expected Created = 0, got %d", summary.Created)
	}
}
