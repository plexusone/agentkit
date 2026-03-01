// Package orchestration provides patterns for building Eino-based agent workflows.
package orchestration

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/cloudwego/eino/compose"

	agenthttp "github.com/plexusone/agentkit/http"
)

// GraphBuilder helps construct Eino workflow graphs.
type GraphBuilder[I, O any] struct {
	graph  *compose.Graph[I, O]
	name   string
	nodes  []string
	client *http.Client
}

// NewGraphBuilder creates a new graph builder.
func NewGraphBuilder[I, O any](name string) *GraphBuilder[I, O] {
	return &GraphBuilder[I, O]{
		graph:  compose.NewGraph[I, O](),
		name:   name,
		nodes:  make([]string, 0),
		client: &http.Client{Timeout: 60 * time.Second},
	}
}

// SetClient sets a custom HTTP client for agent calls.
func (gb *GraphBuilder[I, O]) SetClient(client *http.Client) *GraphBuilder[I, O] {
	gb.client = client
	return gb
}

// Graph returns the underlying Eino graph for direct manipulation.
// Use this when you need access to Eino-specific features.
func (gb *GraphBuilder[I, O]) Graph() *compose.Graph[I, O] {
	return gb.graph
}

// AddLambdaNodeFunc adds a lambda node using a function.
// Note: Due to Go generics limitations, you may need to use Graph() directly
// for complex type conversions.
func (gb *GraphBuilder[I, O]) AddLambdaNodeFunc(name string, lambda *compose.Lambda) error {
	if err := gb.graph.AddLambdaNode(name, lambda); err != nil {
		return fmt.Errorf("failed to add node %s: %w", name, err)
	}
	gb.nodes = append(gb.nodes, name)
	return nil
}

// AddEdge adds an edge between two nodes.
func (gb *GraphBuilder[I, O]) AddEdge(from, to string) error {
	return gb.graph.AddEdge(from, to)
}

// AddStartEdge adds an edge from START to a node.
func (gb *GraphBuilder[I, O]) AddStartEdge(to string) error {
	return gb.graph.AddEdge(compose.START, to)
}

// AddEndEdge adds an edge from a node to END.
func (gb *GraphBuilder[I, O]) AddEndEdge(from string) error {
	return gb.graph.AddEdge(from, compose.END)
}

// Build returns the completed graph.
func (gb *GraphBuilder[I, O]) Build() *compose.Graph[I, O] {
	log.Printf("[%s] Graph built with nodes: %v", gb.name, gb.nodes)
	return gb.graph
}

// Executor executes a compiled Eino graph.
type Executor[I, O any] struct {
	graph  *compose.Graph[I, O]
	name   string
	client *http.Client
}

// NewExecutor creates a new graph executor.
func NewExecutor[I, O any](graph *compose.Graph[I, O], name string) *Executor[I, O] {
	return &Executor[I, O]{
		graph:  graph,
		name:   name,
		client: &http.Client{Timeout: 60 * time.Second},
	}
}

// SetClient sets a custom HTTP client.
func (e *Executor[I, O]) SetClient(client *http.Client) *Executor[I, O] {
	e.client = client
	return e
}

// Execute compiles and runs the graph.
func (e *Executor[I, O]) Execute(ctx context.Context, input I) (O, error) {
	log.Printf("[%s] Starting workflow execution", e.name)

	compiled, err := e.graph.Compile(ctx)
	if err != nil {
		var zero O
		return zero, fmt.Errorf("failed to compile graph: %w", err)
	}

	result, err := compiled.Invoke(ctx, input)
	if err != nil {
		var zero O
		return zero, fmt.Errorf("workflow execution failed: %w", err)
	}

	log.Printf("[%s] Workflow completed successfully", e.name)
	return result, nil
}

// AgentCaller provides methods for calling other agents via HTTP.
type AgentCaller struct {
	client  *http.Client
	baseURL string
	name    string
}

// NewAgentCaller creates a new agent caller.
func NewAgentCaller(baseURL, name string) *AgentCaller {
	return &AgentCaller{
		client:  &http.Client{Timeout: 60 * time.Second},
		baseURL: baseURL,
		name:    name,
	}
}

// SetClient sets a custom HTTP client.
func (ac *AgentCaller) SetClient(client *http.Client) *AgentCaller {
	ac.client = client
	return ac
}

// Call calls an agent endpoint with JSON request/response.
func (ac *AgentCaller) Call(ctx context.Context, endpoint string, request, response interface{}) error {
	url := fmt.Sprintf("%s%s", ac.baseURL, endpoint)
	return agenthttp.PostJSON(ctx, ac.client, url, request, response)
}

// HealthCheck checks if the agent is healthy.
func (ac *AgentCaller) HealthCheck(ctx context.Context) error {
	return agenthttp.HealthCheck(ctx, ac.client, ac.baseURL)
}

// HTTPHandler wraps an executor as an HTTP handler.
type HTTPHandler[I, O any] struct {
	executor *Executor[I, O]
}

// NewHTTPHandler creates a new HTTP handler for a graph executor.
func NewHTTPHandler[I, O any](executor *Executor[I, O]) *HTTPHandler[I, O] {
	return &HTTPHandler[I, O]{executor: executor}
}

// ServeHTTP implements http.Handler.
func (h *HTTPHandler[I, O]) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req I
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf("Invalid request: %v", err), http.StatusBadRequest)
		return
	}

	resp, err := h.executor.Execute(r.Context(), req)
	if err != nil {
		http.Error(w, fmt.Sprintf("Execution failed: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		log.Printf("Failed to encode response: %v", err)
	}
}
