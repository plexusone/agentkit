package agentcore

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
)

// Server implements the AWS AgentCore HTTP contract.
// It handles /ping and /invocations endpoints as required by AgentCore Runtime.
type Server struct {
	registry   *Registry
	config     Config
	httpServer *http.Server
}

// NewServer creates a new AgentCore server with the given configuration.
func NewServer(cfg Config) *Server {
	if cfg.Port == 0 {
		cfg.Port = 8080
	}

	return &Server{
		registry: NewRegistry(),
		config:   cfg,
	}
}

// NewServerWithRegistry creates a new AgentCore server with a pre-configured registry.
func NewServerWithRegistry(cfg Config, registry *Registry) *Server {
	if cfg.Port == 0 {
		cfg.Port = 8080
	}

	server := &Server{
		registry: registry,
		config:   cfg,
	}

	if cfg.DefaultAgent != "" {
		_ = registry.SetDefault(cfg.DefaultAgent)
	}

	return server
}

// Register adds an agent to the server's registry.
func (s *Server) Register(ctx context.Context, agent Agent) error {
	return s.registry.Register(ctx, agent)
}

// MustRegister is like Register but panics on error.
func (s *Server) MustRegister(ctx context.Context, agent Agent) {
	s.registry.MustRegister(ctx, agent)
}

// RegisterAll registers multiple agents.
func (s *Server) RegisterAll(ctx context.Context, agents ...Agent) error {
	return s.registry.RegisterAll(ctx, agents...)
}

// SetDefaultAgent sets the default agent to use when none is specified.
func (s *Server) SetDefaultAgent(name string) error {
	return s.registry.SetDefault(name)
}

// handlePing implements the /ping endpoint required by AgentCore.
// Returns 200 OK if the server is healthy.
func (s *Server) handlePing(w http.ResponseWriter, r *http.Request) {
	// Check health of all registered agents
	healthResults := s.registry.HealthCheck(r.Context())

	for name, err := range healthResults {
		if err != nil {
			log.Printf("[AgentCore] Agent %s unhealthy: %v", name, err)
			http.Error(w, fmt.Sprintf("agent unhealthy: %s: %v", name, err), http.StatusServiceUnavailable)
			return
		}
	}

	w.WriteHeader(http.StatusOK)
}

// handleInvocations implements the /invocations endpoint required by AgentCore.
// Routes requests to the appropriate agent and returns the response.
func (s *Server) handleInvocations(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse request
	var req Request
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		if s.config.EnableRequestLogging {
			log.Printf("[AgentCore] Invalid request: %v", err)
		}
		http.Error(w, fmt.Sprintf("invalid request: %v", err), http.StatusBadRequest)
		return
	}

	// Use default agent if not specified
	if req.Agent == "" {
		req.Agent = s.config.DefaultAgent
	}

	if s.config.EnableRequestLogging {
		log.Printf("[AgentCore] Invocation: agent=%s session=%s prompt_len=%d",
			req.Agent, req.SessionID, len(req.Prompt))
	}

	// Create session context
	ctx := NewSessionContext(r.Context(), req.SessionID, &req)

	// Invoke agent
	resp, err := s.registry.Invoke(ctx, req)
	if err != nil {
		if s.config.EnableRequestLogging {
			log.Printf("[AgentCore] Invocation failed: %v", err)
		}
		http.Error(w, fmt.Sprintf("invocation failed: %v", err), http.StatusInternalServerError)
		return
	}

	// Send response
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		log.Printf("[AgentCore] Failed to encode response: %v", err)
	}

	if s.config.EnableRequestLogging && s.config.EnableSessionTracking {
		log.Printf("[AgentCore] Invocation complete: session=%s output_len=%d", //nolint:gosec // G706: Internal logging
			req.SessionID, len(resp.Output))
	}
}

// Start starts the AgentCore server. This method blocks until the server stops.
func (s *Server) Start() error {
	mux := http.NewServeMux()
	mux.HandleFunc("/ping", s.handlePing)
	mux.HandleFunc("/invocations", s.handleInvocations)

	addr := fmt.Sprintf(":%d", s.config.Port)
	s.httpServer = &http.Server{
		Addr:         addr,
		Handler:      mux,
		ReadTimeout:  s.config.ReadTimeout,
		WriteTimeout: s.config.WriteTimeout,
		IdleTimeout:  s.config.IdleTimeout,
	}

	log.Printf("[AgentCore] Server starting on %s", addr)
	log.Printf("[AgentCore] Registered agents: %v", s.registry.List())
	log.Printf("[AgentCore] Endpoints: /ping, /invocations")

	return s.httpServer.ListenAndServe()
}

// StartAsync starts the server in the background.
// Returns immediately. Use Stop() to shut down the server.
func (s *Server) StartAsync() {
	go func() {
		if err := s.Start(); err != nil && err != http.ErrServerClosed {
			log.Printf("[AgentCore] Server error: %v", err)
		}
	}()
}

// Stop gracefully shuts down the server and closes all agents.
func (s *Server) Stop(ctx context.Context) error {
	var errs []error

	// Shutdown HTTP server
	if s.httpServer != nil {
		if err := s.httpServer.Shutdown(ctx); err != nil {
			errs = append(errs, fmt.Errorf("http shutdown: %w", err))
		}
	}

	// Close all agents
	if err := s.registry.Close(); err != nil {
		errs = append(errs, err)
	}

	if len(errs) > 0 {
		return fmt.Errorf("shutdown errors: %v", errs)
	}
	return nil
}

// Registry returns the server's agent registry.
func (s *Server) Registry() *Registry {
	return s.registry
}

// Builder provides a fluent interface for building an AgentCore server.
type Builder struct {
	config   Config
	agents   []Agent
	registry *Registry
}

// NewBuilder creates a new server builder.
func NewBuilder() *Builder {
	return &Builder{
		config: DefaultConfig(),
		agents: make([]Agent, 0),
	}
}

// WithConfig sets the server configuration.
func (b *Builder) WithConfig(cfg Config) *Builder {
	b.config = cfg
	return b
}

// WithPort sets the server port.
func (b *Builder) WithPort(port int) *Builder {
	b.config.Port = port
	return b
}

// WithAgent adds an agent to the server.
func (b *Builder) WithAgent(agent Agent) *Builder {
	b.agents = append(b.agents, agent)
	return b
}

// WithAgents adds multiple agents to the server.
func (b *Builder) WithAgents(agents ...Agent) *Builder {
	b.agents = append(b.agents, agents...)
	return b
}

// WithDefaultAgent sets the default agent name.
func (b *Builder) WithDefaultAgent(name string) *Builder {
	b.config.DefaultAgent = name
	return b
}

// WithRegistry uses an existing registry instead of creating a new one.
func (b *Builder) WithRegistry(registry *Registry) *Builder {
	b.registry = registry
	return b
}

// Build creates the server and registers all agents.
func (b *Builder) Build(ctx context.Context) (*Server, error) {
	var server *Server

	if b.registry != nil {
		server = NewServerWithRegistry(b.config, b.registry)
	} else {
		server = NewServer(b.config)
	}

	if err := server.RegisterAll(ctx, b.agents...); err != nil {
		return nil, err
	}

	if b.config.DefaultAgent != "" {
		if err := server.SetDefaultAgent(b.config.DefaultAgent); err != nil {
			return nil, err
		}
	}

	return server, nil
}

// MustBuild is like Build but panics on error.
func (b *Builder) MustBuild(ctx context.Context) *Server {
	server, err := b.Build(ctx)
	if err != nil {
		panic(err)
	}
	return server
}
