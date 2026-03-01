package agentcore

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/plexusone/agentkit/orchestration"
)

// ExecutorAdapter wraps an AgentKit Executor to implement the Agent interface.
// This allows Eino-based workflows to run on AgentCore without modification.
type ExecutorAdapter[I, O any] struct {
	name         string
	executor     *orchestration.Executor[I, O]
	parseInput   func(prompt string) (I, error)
	formatOutput func(output O) string
}

// ExecutorAdapterConfig configures an ExecutorAdapter.
type ExecutorAdapterConfig[I, O any] struct {
	// Name is the agent name for routing.
	Name string

	// Executor is the AgentKit Executor to wrap.
	Executor *orchestration.Executor[I, O]

	// ParseInput converts the prompt string to the executor's input type.
	// If nil, JSON unmarshaling is used.
	ParseInput func(prompt string) (I, error)

	// FormatOutput converts the executor's output to a response string.
	// If nil, JSON marshaling is used.
	FormatOutput func(output O) string
}

// NewExecutorAdapter creates an Agent that wraps an AgentKit Executor.
func NewExecutorAdapter[I, O any](cfg ExecutorAdapterConfig[I, O]) *ExecutorAdapter[I, O] {
	adapter := &ExecutorAdapter[I, O]{
		name:     cfg.Name,
		executor: cfg.Executor,
	}

	// Default input parser: JSON unmarshal
	if cfg.ParseInput != nil {
		adapter.parseInput = cfg.ParseInput
	} else {
		adapter.parseInput = func(prompt string) (I, error) {
			var input I
			if err := json.Unmarshal([]byte(prompt), &input); err != nil {
				return input, fmt.Errorf("failed to parse input as JSON: %w", err)
			}
			return input, nil
		}
	}

	// Default output formatter: JSON marshal
	if cfg.FormatOutput != nil {
		adapter.formatOutput = cfg.FormatOutput
	} else {
		adapter.formatOutput = func(output O) string {
			data, err := json.Marshal(output)
			if err != nil {
				return fmt.Sprintf("error marshaling output: %v", err)
			}
			return string(data)
		}
	}

	return adapter
}

// Name returns the agent name.
func (a *ExecutorAdapter[I, O]) Name() string {
	return a.name
}

// Invoke executes the wrapped Executor.
func (a *ExecutorAdapter[I, O]) Invoke(ctx context.Context, req Request) (Response, error) {
	// Parse input
	input, err := a.parseInput(req.Prompt)
	if err != nil {
		return Response{Error: err.Error()}, err
	}

	// Execute workflow
	output, err := a.executor.Execute(ctx, input)
	if err != nil {
		return Response{Error: err.Error()}, err
	}

	// Format output
	return Response{
		Output: a.formatOutput(output),
	}, nil
}

// WrapExecutor is a convenience function to create an ExecutorAdapter with defaults.
// Uses JSON for input parsing and output formatting.
func WrapExecutor[I, O any](name string, executor *orchestration.Executor[I, O]) *ExecutorAdapter[I, O] {
	return NewExecutorAdapter(ExecutorAdapterConfig[I, O]{
		Name:     name,
		Executor: executor,
	})
}

// WrapExecutorWithPrompt creates an adapter where the prompt is passed directly
// to a field in the input struct. Useful for simple prompt-in/response-out agents.
func WrapExecutorWithPrompt[I, O any](
	name string,
	executor *orchestration.Executor[I, O],
	setPrompt func(prompt string) I,
	getOutput func(output O) string,
) *ExecutorAdapter[I, O] {
	return NewExecutorAdapter(ExecutorAdapterConfig[I, O]{
		Name:     name,
		Executor: executor,
		ParseInput: func(prompt string) (I, error) {
			return setPrompt(prompt), nil
		},
		FormatOutput: getOutput,
	})
}

// HandlerAdapter wraps an http.HandlerFunc-style function as an Agent.
// Useful for migrating existing HTTP handlers to AgentCore.
type HandlerAdapter struct {
	name    string
	handler func(ctx context.Context, req Request) (Response, error)
}

// NewHandlerAdapter creates an Agent from a handler function.
func NewHandlerAdapter(name string, handler func(ctx context.Context, req Request) (Response, error)) *HandlerAdapter {
	return &HandlerAdapter{
		name:    name,
		handler: handler,
	}
}

// Name returns the agent name.
func (a *HandlerAdapter) Name() string {
	return a.name
}

// Invoke calls the handler function.
func (a *HandlerAdapter) Invoke(ctx context.Context, req Request) (Response, error) {
	return a.handler(ctx, req)
}

// ADKAgentAdapter wraps a Google ADK agent for AgentCore.
// Note: This is a placeholder - actual implementation depends on ADK's Go API.
type ADKAgentAdapter struct {
	name        string
	description string
	invoke      func(ctx context.Context, prompt string) (string, error)
}

// ADKAgentConfig configures an ADKAgentAdapter.
type ADKAgentConfig struct {
	Name        string
	Description string
	// Invoke is the function that calls the ADK agent.
	// This abstraction allows different ADK agent implementations.
	Invoke func(ctx context.Context, prompt string) (string, error)
}

// NewADKAgentAdapter creates an Agent that wraps a Google ADK agent.
func NewADKAgentAdapter(cfg ADKAgentConfig) *ADKAgentAdapter {
	return &ADKAgentAdapter{
		name:        cfg.Name,
		description: cfg.Description,
		invoke:      cfg.Invoke,
	}
}

// Name returns the agent name.
func (a *ADKAgentAdapter) Name() string {
	return a.name
}

// Invoke calls the underlying ADK agent.
func (a *ADKAgentAdapter) Invoke(ctx context.Context, req Request) (Response, error) {
	output, err := a.invoke(ctx, req.Prompt)
	if err != nil {
		return Response{Error: err.Error()}, err
	}
	return Response{Output: output}, nil
}

// MultiAgentRouter routes requests to multiple agents based on the agent field.
// This is useful when you want a single AgentCore endpoint to handle multiple agents.
type MultiAgentRouter struct {
	name     string
	registry *Registry
}

// NewMultiAgentRouter creates a router that delegates to other agents.
func NewMultiAgentRouter(name string, agents ...Agent) (*MultiAgentRouter, error) {
	registry := NewRegistry()
	ctx := context.Background()

	for _, agent := range agents {
		if err := registry.Register(ctx, agent); err != nil {
			return nil, err
		}
	}

	return &MultiAgentRouter{
		name:     name,
		registry: registry,
	}, nil
}

// Name returns the router name.
func (r *MultiAgentRouter) Name() string {
	return r.name
}

// Invoke routes the request to the appropriate agent.
func (r *MultiAgentRouter) Invoke(ctx context.Context, req Request) (Response, error) {
	// The agent field in the request determines which sub-agent to use
	return r.registry.Invoke(ctx, req)
}

// RegisterAgent adds an agent to the router.
func (r *MultiAgentRouter) RegisterAgent(ctx context.Context, agent Agent) error {
	return r.registry.Register(ctx, agent)
}
