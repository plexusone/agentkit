// Package local provides an embedded local mode for running agents in-process.
package local

import (
	"context"
	"encoding/json"
	"time"
)

// StateBackend defines the interface for persisting agent execution state.
// This enables resumable workflows and cross-invocation state management.
type StateBackend interface {
	// SaveState persists the execution state for a given run ID.
	SaveState(ctx context.Context, runID string, state *ExecutionState) error

	// LoadState retrieves the execution state for a given run ID.
	// Returns nil, nil if no state exists for the run ID.
	LoadState(ctx context.Context, runID string) (*ExecutionState, error)

	// DeleteState removes the execution state for a given run ID.
	DeleteState(ctx context.Context, runID string) error

	// ListRuns returns all run IDs, optionally filtered by workflow ID.
	ListRuns(ctx context.Context, workflowID string) ([]string, error)
}

// ExecutionState represents the complete state of a workflow execution.
type ExecutionState struct {
	// RunID is the unique identifier for this execution run.
	RunID string `json:"run_id"`

	// WorkflowID identifies the workflow being executed.
	WorkflowID string `json:"workflow_id"`

	// Status is the overall execution status.
	Status ExecutionStatus `json:"status"`

	// Steps contains the state of each step in the workflow.
	Steps map[string]*StepState `json:"steps"`

	// Context holds shared data across steps (e.g., accumulated results).
	Context map[string]any `json:"context,omitempty"`

	// StartedAt is when execution began.
	StartedAt time.Time `json:"started_at"`

	// UpdatedAt is when the state was last updated.
	UpdatedAt time.Time `json:"updated_at"`

	// CompletedAt is when execution finished (success or failure).
	CompletedAt *time.Time `json:"completed_at,omitempty"`

	// Error contains error details if the execution failed.
	Error string `json:"error,omitempty"`
}

// ExecutionStatus represents the status of a workflow execution.
type ExecutionStatus string

const (
	// StatusPending indicates execution has not yet started.
	StatusPending ExecutionStatus = "pending"

	// StatusRunning indicates execution is in progress.
	StatusRunning ExecutionStatus = "running"

	// StatusCompleted indicates execution finished successfully.
	StatusCompleted ExecutionStatus = "completed"

	// StatusFailed indicates execution failed with an error.
	StatusFailed ExecutionStatus = "failed"

	// StatusCancelled indicates execution was cancelled.
	StatusCancelled ExecutionStatus = "cancelled"

	// StatusWaitingHITL indicates execution is paused waiting for human input.
	StatusWaitingHITL ExecutionStatus = "waiting_hitl"
)

// StepState represents the state of a single step in the workflow.
type StepState struct {
	// StepID is the unique identifier for this step.
	StepID string `json:"step_id"`

	// AgentName is the name of the agent executing this step.
	AgentName string `json:"agent_name"`

	// Status is the step's execution status.
	Status StepStatus `json:"status"`

	// Input is the input provided to the agent.
	Input string `json:"input,omitempty"`

	// Output is the result returned by the agent.
	Output string `json:"output,omitempty"`

	// Error contains error details if the step failed.
	Error string `json:"error,omitempty"`

	// StartedAt is when the step began execution.
	StartedAt *time.Time `json:"started_at,omitempty"`

	// CompletedAt is when the step finished execution.
	CompletedAt *time.Time `json:"completed_at,omitempty"`

	// Retries is the number of retry attempts made.
	Retries int `json:"retries,omitempty"`

	// Dependencies lists step IDs this step depends on.
	Dependencies []string `json:"dependencies,omitempty"`

	// HITLRequest contains the human-in-the-loop request if waiting for input.
	HITLRequest *HITLRequest `json:"hitl_request,omitempty"`

	// HITLResponse contains the human response when provided.
	HITLResponse *HITLResponse `json:"hitl_response,omitempty"`
}

// HITLRequest represents a human-in-the-loop request from an agent.
type HITLRequest struct {
	// RequestType is the kind of input needed: "approval", "input", "choice", "review".
	RequestType string `json:"request_type"`

	// Question is what the agent is asking.
	Question string `json:"question"`

	// Context provides background information for the human.
	Context string `json:"context,omitempty"`

	// Options are available choices (for "choice" type).
	Options []string `json:"options,omitempty"`

	// DefaultValue is the suggested default response.
	DefaultValue string `json:"default_value,omitempty"`

	// CreatedAt is when the request was created.
	CreatedAt time.Time `json:"created_at"`
}

// HITLResponse represents the human's response to a HITL request.
type HITLResponse struct {
	// Response is the human's input.
	Response string `json:"response"`

	// Approved is true for approval-type requests.
	Approved *bool `json:"approved,omitempty"`

	// SelectedOption is the chosen option index (for choice-type).
	SelectedOption *int `json:"selected_option,omitempty"`

	// RespondedAt is when the response was provided.
	RespondedAt time.Time `json:"responded_at"`
}

// StepStatus represents the status of a workflow step.
type StepStatus string

const (
	// StepPending indicates the step has not yet started.
	StepPending StepStatus = "pending"

	// StepBlocked indicates the step is waiting on dependencies.
	StepBlocked StepStatus = "blocked"

	// StepRunning indicates the step is currently executing.
	StepRunning StepStatus = "running"

	// StepCompleted indicates the step finished successfully.
	StepCompleted StepStatus = "completed"

	// StepFailed indicates the step failed with an error.
	StepFailed StepStatus = "failed"

	// StepSkipped indicates the step was skipped (e.g., due to conditions).
	StepSkipped StepStatus = "skipped"

	// StepWaitingHITL indicates the step is waiting for human input.
	StepWaitingHITL StepStatus = "waiting_hitl"
)

// NewExecutionState creates a new ExecutionState for a workflow run.
func NewExecutionState(runID, workflowID string) *ExecutionState {
	now := time.Now()
	return &ExecutionState{
		RunID:      runID,
		WorkflowID: workflowID,
		Status:     StatusPending,
		Steps:      make(map[string]*StepState),
		Context:    make(map[string]any),
		StartedAt:  now,
		UpdatedAt:  now,
	}
}

// AddStep adds a new step to the execution state.
func (s *ExecutionState) AddStep(stepID, agentName string, dependencies []string) {
	s.Steps[stepID] = &StepState{
		StepID:       stepID,
		AgentName:    agentName,
		Status:       StepPending,
		Dependencies: dependencies,
	}
	s.UpdatedAt = time.Now()
}

// MarkStepRunning marks a step as running.
func (s *ExecutionState) MarkStepRunning(stepID, input string) {
	if step, ok := s.Steps[stepID]; ok {
		now := time.Now()
		step.Status = StepRunning
		step.Input = input
		step.StartedAt = &now
		s.UpdatedAt = now
	}
}

// MarkStepCompleted marks a step as completed with output.
func (s *ExecutionState) MarkStepCompleted(stepID, output string) {
	if step, ok := s.Steps[stepID]; ok {
		now := time.Now()
		step.Status = StepCompleted
		step.Output = output
		step.CompletedAt = &now
		s.UpdatedAt = now
	}
}

// MarkStepFailed marks a step as failed with an error.
func (s *ExecutionState) MarkStepFailed(stepID, errMsg string) {
	if step, ok := s.Steps[stepID]; ok {
		now := time.Now()
		step.Status = StepFailed
		step.Error = errMsg
		step.CompletedAt = &now
		s.UpdatedAt = now
	}
}

// MarkStepSkipped marks a step as skipped.
func (s *ExecutionState) MarkStepSkipped(stepID string) {
	if step, ok := s.Steps[stepID]; ok {
		now := time.Now()
		step.Status = StepSkipped
		step.CompletedAt = &now
		s.UpdatedAt = now
	}
}

// Start marks the execution as running.
func (s *ExecutionState) Start() {
	s.Status = StatusRunning
	s.UpdatedAt = time.Now()
}

// Complete marks the execution as completed.
func (s *ExecutionState) Complete() {
	now := time.Now()
	s.Status = StatusCompleted
	s.CompletedAt = &now
	s.UpdatedAt = now
}

// Fail marks the execution as failed.
func (s *ExecutionState) Fail(errMsg string) {
	now := time.Now()
	s.Status = StatusFailed
	s.Error = errMsg
	s.CompletedAt = &now
	s.UpdatedAt = now
}

// Cancel marks the execution as cancelled.
func (s *ExecutionState) Cancel() {
	now := time.Now()
	s.Status = StatusCancelled
	s.CompletedAt = &now
	s.UpdatedAt = now
}

// PauseForHITL marks the execution as waiting for human input.
func (s *ExecutionState) PauseForHITL() {
	s.Status = StatusWaitingHITL
	s.UpdatedAt = time.Now()
}

// IsComplete returns true if the execution has finished (success, failure, or cancelled).
func (s *ExecutionState) IsComplete() bool {
	return s.Status == StatusCompleted || s.Status == StatusFailed || s.Status == StatusCancelled
}

// IsWaitingHITL returns true if any step is waiting for human input.
func (s *ExecutionState) IsWaitingHITL() bool {
	for _, step := range s.Steps {
		if step.Status == StepWaitingHITL {
			return true
		}
	}
	return false
}

// GetWaitingHITLStep returns the step waiting for HITL, if any.
func (s *ExecutionState) GetWaitingHITLStep() *StepState {
	for _, step := range s.Steps {
		if step.Status == StepWaitingHITL {
			return step
		}
	}
	return nil
}

// MarkStepWaitingHITL marks a step as waiting for human input.
func (s *ExecutionState) MarkStepWaitingHITL(stepID string, request *HITLRequest) {
	if step, ok := s.Steps[stepID]; ok {
		now := time.Now()
		step.Status = StepWaitingHITL
		step.HITLRequest = request
		request.CreatedAt = now
		s.UpdatedAt = now
	}
}

// ProvideHITLResponse provides a human response to a waiting step.
func (s *ExecutionState) ProvideHITLResponse(stepID string, response *HITLResponse) {
	if step, ok := s.Steps[stepID]; ok {
		if step.Status == StepWaitingHITL {
			step.HITLResponse = response
			response.RespondedAt = time.Now()
			// Step goes back to pending so it can be re-executed with the response
			step.Status = StepPending
			s.UpdatedAt = time.Now()
		}
	}
}

// GetReadySteps returns steps that are ready to execute (pending with all dependencies completed).
func (s *ExecutionState) GetReadySteps() []string {
	var ready []string
	for stepID, step := range s.Steps {
		if step.Status != StepPending {
			continue
		}

		// Check if all dependencies are completed
		allDepsCompleted := true
		for _, depID := range step.Dependencies {
			if depStep, ok := s.Steps[depID]; ok {
				if depStep.Status != StepCompleted {
					allDepsCompleted = false
					break
				}
			}
		}

		if allDepsCompleted {
			ready = append(ready, stepID)
		}
	}
	return ready
}

// SetContext sets a value in the execution context.
func (s *ExecutionState) SetContext(key string, value any) {
	s.Context[key] = value
	s.UpdatedAt = time.Now()
}

// GetContext retrieves a value from the execution context.
func (s *ExecutionState) GetContext(key string) (any, bool) {
	val, ok := s.Context[key]
	return val, ok
}

// ToJSON serializes the execution state to JSON.
func (s *ExecutionState) ToJSON() ([]byte, error) {
	return json.MarshalIndent(s, "", "  ")
}

// FromJSON deserializes execution state from JSON.
func FromJSON(data []byte) (*ExecutionState, error) {
	var state ExecutionState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, err
	}
	return &state, nil
}
