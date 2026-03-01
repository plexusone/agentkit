// Package a2a provides a factory for creating A2A protocol servers.
// This eliminates ~350 lines of boilerplate per agent.
package a2a

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/url"
	"time"

	"github.com/a2aproject/a2a-go/a2a"
	"github.com/a2aproject/a2a-go/a2asrv"
	"google.golang.org/adk/agent"
	"google.golang.org/adk/runner"
	"google.golang.org/adk/server/adka2a"
	"google.golang.org/adk/session"
)

// Config holds the configuration for an A2A server.
type Config struct {
	// Agent is the ADK agent to expose via A2A protocol.
	Agent agent.Agent

	// Port is the port to listen on. If empty, a random port is used.
	Port string

	// Description overrides the agent's description in the agent card.
	// If empty, uses the agent's built-in description.
	Description string

	// InvokePath is the path for the invoke endpoint. Default is "/invoke".
	InvokePath string

	// ReadHeaderTimeout is the timeout for reading request headers.
	// Default is 10 seconds.
	ReadHeaderTimeout time.Duration

	// SessionService is the session service for the executor.
	// If nil, uses in-memory session service.
	SessionService session.Service
}

// Server wraps an A2A protocol server with convenient lifecycle methods.
type Server struct {
	agent      agent.Agent
	listener   net.Listener
	baseURL    *url.URL
	httpServer *http.Server
	config     Config
}

// NewServer creates a new A2A server for the given agent.
// This is a factory that eliminates ~70 lines of boilerplate per agent.
func NewServer(cfg Config) (*Server, error) {
	if cfg.Agent == nil {
		return nil, fmt.Errorf("agent is required")
	}

	// Set defaults
	if cfg.Port == "" {
		cfg.Port = "0" // Random port
	}
	if cfg.InvokePath == "" {
		cfg.InvokePath = "/invoke"
	}
	if cfg.ReadHeaderTimeout == 0 {
		cfg.ReadHeaderTimeout = 10 * time.Second
	}
	if cfg.SessionService == nil {
		cfg.SessionService = session.InMemoryService()
	}

	// Create listener
	addr := "0.0.0.0:" + cfg.Port
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("failed to create listener: %w", err)
	}

	baseURL := &url.URL{Scheme: "http", Host: listener.Addr().String()}

	return &Server{
		agent:    cfg.Agent,
		listener: listener,
		baseURL:  baseURL,
		config:   cfg,
	}, nil
}

// Start starts the A2A server. This method blocks until the server is stopped.
func (s *Server) Start(ctx context.Context) error {
	description := s.config.Description
	if description == "" {
		description = s.agent.Name()
	}

	// Build agent card
	agentCard := &a2a.AgentCard{
		Name:               s.agent.Name(),
		Description:        description,
		Skills:             adka2a.BuildAgentSkills(s.agent),
		PreferredTransport: a2a.TransportProtocolJSONRPC,
		URL:                s.baseURL.JoinPath(s.config.InvokePath).String(),
		Capabilities:       a2a.AgentCapabilities{Streaming: true},
	}

	mux := http.NewServeMux()

	// Register agent card endpoint
	mux.Handle(a2asrv.WellKnownAgentCardPath, a2asrv.NewStaticAgentCardHandler(agentCard))

	// Create executor
	executor := adka2a.NewExecutor(adka2a.ExecutorConfig{
		RunnerConfig: runner.Config{
			AppName:        s.agent.Name(),
			Agent:          s.agent,
			SessionService: s.config.SessionService,
		},
	})

	// Create handlers
	requestHandler := a2asrv.NewHandler(executor)
	mux.Handle(s.config.InvokePath, a2asrv.NewJSONRPCHandler(requestHandler))

	// Health check
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
	})

	log.Printf("[A2A] %s server starting on %s", s.agent.Name(), s.baseURL.String())          //nolint:gosec // G706: Server startup log
	log.Printf("[A2A]   Agent Card: %s%s", s.baseURL.String(), a2asrv.WellKnownAgentCardPath) //nolint:gosec // G706: Server startup log
	log.Printf("[A2A]   Invoke: %s%s", s.baseURL.String(), s.config.InvokePath)               //nolint:gosec // G706: Server startup log

	s.httpServer = &http.Server{
		Handler:           mux,
		ReadHeaderTimeout: s.config.ReadHeaderTimeout,
	}

	return s.httpServer.Serve(s.listener)
}

// StartAsync starts the A2A server in the background.
// Returns immediately. Use Stop() to shut down the server.
func (s *Server) StartAsync(ctx context.Context) {
	go func() {
		if err := s.Start(ctx); err != nil && err != http.ErrServerClosed {
			log.Printf("[A2A] %s server error: %v", s.agent.Name(), err)
		}
	}()
}

// Stop gracefully shuts down the server.
func (s *Server) Stop(ctx context.Context) error {
	if s.httpServer != nil {
		return s.httpServer.Shutdown(ctx)
	}
	return s.listener.Close()
}

// URL returns the base URL of the server.
func (s *Server) URL() string {
	return s.baseURL.String()
}

// AgentCardURL returns the URL of the agent card endpoint.
func (s *Server) AgentCardURL() string {
	return s.baseURL.String() + a2asrv.WellKnownAgentCardPath
}

// InvokeURL returns the URL of the invoke endpoint.
func (s *Server) InvokeURL() string {
	return s.baseURL.JoinPath(s.config.InvokePath).String()
}

// Addr returns the address the server is listening on.
func (s *Server) Addr() net.Addr {
	return s.listener.Addr()
}
