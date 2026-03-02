// Package local provides an embedded local mode for running agents in-process.
package local

import (
	"errors"
	"fmt"
)

// ErrHITLRequired is returned when an agent needs human input to proceed.
var ErrHITLRequired = errors.New("human-in-the-loop input required")

// HITLError wraps an HITL request as an error for agents to return.
type HITLError struct {
	Request *HITLRequest
}

// Error implements the error interface.
func (e *HITLError) Error() string {
	return fmt.Sprintf("%s: %s", ErrHITLRequired, e.Request.Question)
}

// Unwrap returns the underlying ErrHITLRequired error.
func (e *HITLError) Unwrap() error {
	return ErrHITLRequired
}

// NewHITLError creates a new HITL error with a request.
func NewHITLError(requestType, question string) *HITLError {
	return &HITLError{
		Request: &HITLRequest{
			RequestType: requestType,
			Question:    question,
		},
	}
}

// NewApprovalRequest creates an HITL error for approval requests.
func NewApprovalRequest(question, context string) *HITLError {
	return &HITLError{
		Request: &HITLRequest{
			RequestType: "approval",
			Question:    question,
			Context:     context,
		},
	}
}

// NewInputRequest creates an HITL error for free-form input requests.
func NewInputRequest(question, context, defaultValue string) *HITLError {
	return &HITLError{
		Request: &HITLRequest{
			RequestType:  "input",
			Question:     question,
			Context:      context,
			DefaultValue: defaultValue,
		},
	}
}

// NewChoiceRequest creates an HITL error for choice requests.
func NewChoiceRequest(question, context string, options []string) *HITLError {
	return &HITLError{
		Request: &HITLRequest{
			RequestType: "choice",
			Question:    question,
			Context:     context,
			Options:     options,
		},
	}
}

// NewReviewRequest creates an HITL error for review requests.
func NewReviewRequest(question, content string) *HITLError {
	return &HITLError{
		Request: &HITLRequest{
			RequestType: "review",
			Question:    question,
			Context:     content,
		},
	}
}

// IsHITLError checks if an error is an HITL request.
func IsHITLError(err error) bool {
	return errors.Is(err, ErrHITLRequired)
}

// GetHITLRequest extracts the HITL request from an error.
func GetHITLRequest(err error) *HITLRequest {
	var hitlErr *HITLError
	if errors.As(err, &hitlErr) {
		return hitlErr.Request
	}
	return nil
}

// HITLHandler is a callback for handling HITL requests.
// Implementations can prompt the user interactively, send notifications,
// or save the request for later response.
type HITLHandler func(runID, stepName string, request *HITLRequest) (*HITLResponse, error)

// DefaultHITLHandler returns nil, causing the workflow to pause.
// The workflow can be resumed later using the respond command.
func DefaultHITLHandler(runID, stepName string, request *HITLRequest) (*HITLResponse, error) {
	return nil, nil // Pause workflow
}
