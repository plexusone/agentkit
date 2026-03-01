package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"strings"

	"github.com/plexusone/agentkit/platforms/local"
)

const (
	// ProtocolVersion is the supported MCP protocol version.
	ProtocolVersion = "2024-11-05"
)

// Server is an MCP server that exposes agent teams to CLI assistants.
type Server struct {
	runner      *local.Runner
	serverInfo  ServerInfo
	initialized bool
}

// NewServer creates a new MCP server.
func NewServer(runner *local.Runner, name, version string) *Server {
	return &Server{
		runner: runner,
		serverInfo: ServerInfo{
			Name:    name,
			Version: version,
		},
	}
}

// ServeStdio runs the MCP server over stdio (stdin/stdout).
func (s *Server) ServeStdio(ctx context.Context) error {
	log.Println("[MCP] Starting stdio server")
	return s.serve(ctx, os.Stdin, os.Stdout)
}

// serve handles the MCP protocol over the given reader/writer.
func (s *Server) serve(ctx context.Context, r io.Reader, w io.Writer) error {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 1024*1024), 10*1024*1024) // 10MB max message

	for scanner.Scan() {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		line := scanner.Text()
		if line == "" {
			continue
		}

		var req Request
		if err := json.Unmarshal([]byte(line), &req); err != nil {
			s.writeError(w, nil, ErrParseError, "Parse error", err)
			continue
		}

		resp := s.handleRequest(ctx, &req)
		if resp != nil {
			if err := s.writeResponse(w, resp); err != nil {
				log.Printf("[MCP] Write error: %v", err)
			}
		}
	}

	return scanner.Err()
}

// handleRequest processes a single MCP request.
func (s *Server) handleRequest(ctx context.Context, req *Request) *Response {
	log.Printf("[MCP] Request: %s", req.Method)

	switch req.Method {
	case "initialize":
		return s.handleInitialize(req)
	case "initialized":
		// Notification, no response
		return nil
	case "tools/list":
		return s.handleToolsList(req)
	case "tools/call":
		return s.handleToolsCall(ctx, req)
	case "resources/list":
		return s.handleResourcesList(req)
	case "resources/read":
		return s.handleResourcesRead(ctx, req)
	case "prompts/list":
		return s.handlePromptsList(req)
	case "prompts/get":
		return s.handlePromptsGet(req)
	default:
		return s.errorResponse(req.ID, ErrMethodNotFound, "Method not found", nil)
	}
}

// handleInitialize handles the initialize request.
func (s *Server) handleInitialize(req *Request) *Response {
	s.initialized = true

	result := InitializeResult{
		ProtocolVersion: ProtocolVersion,
		Capabilities: Capabilities{
			Tools: &ToolsCapability{},
		},
		ServerInfo: s.serverInfo,
	}

	return &Response{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result:  result,
	}
}

// handleToolsList returns the list of available tools.
func (s *Server) handleToolsList(req *Request) *Response {
	tools := []ToolInfo{
		{
			Name:        "invoke_agent",
			Description: "Invoke a specific agent with an input prompt",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"agent": {
						Type:        "string",
						Description: "Name of the agent to invoke",
						Enum:        s.runner.ListAgents(),
					},
					"input": {
						Type:        "string",
						Description: "Input prompt for the agent",
					},
				},
				Required: []string{"agent", "input"},
			},
		},
		{
			Name:        "invoke_parallel",
			Description: "Invoke multiple agents in parallel with the same input",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"agents": {
						Type:        "string",
						Description: "Comma-separated list of agent names",
					},
					"input": {
						Type:        "string",
						Description: "Input prompt for all agents",
					},
				},
				Required: []string{"agents", "input"},
			},
		},
		{
			Name:        "list_agents",
			Description: "List all available agents and their descriptions",
			InputSchema: InputSchema{
				Type: "object",
			},
		},
	}

	// Add direct tools from the runner's toolset
	directTools := []ToolInfo{
		{
			Name:        "read_file",
			Description: "Read the contents of a file in the workspace",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"path": {
						Type:        "string",
						Description: "Path to the file (relative to workspace)",
					},
				},
				Required: []string{"path"},
			},
		},
		{
			Name:        "glob_files",
			Description: "Find files matching a glob pattern",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"pattern": {
						Type:        "string",
						Description: "Glob pattern (e.g., '**/*.go')",
					},
				},
				Required: []string{"pattern"},
			},
		},
		{
			Name:        "grep_files",
			Description: "Search for a pattern in files",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"pattern": {
						Type:        "string",
						Description: "Regex pattern to search for",
					},
					"file_pattern": {
						Type:        "string",
						Description: "Optional file name pattern filter",
					},
				},
				Required: []string{"pattern"},
			},
		},
		{
			Name:        "run_command",
			Description: "Execute a shell command in the workspace",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"command": {
						Type:        "string",
						Description: "Shell command to execute",
					},
				},
				Required: []string{"command"},
			},
		},
	}

	tools = append(tools, directTools...)

	return &Response{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result:  ListToolsResult{Tools: tools},
	}
}

// handleToolsCall handles a tool invocation.
func (s *Server) handleToolsCall(ctx context.Context, req *Request) *Response {
	var params CallToolParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return s.errorResponse(req.ID, ErrInvalidParams, "Invalid params", err)
	}

	log.Printf("[MCP] Tool call: %s", params.Name)

	var result CallToolResult

	switch params.Name {
	case "invoke_agent":
		result = s.callInvokeAgent(ctx, params.Arguments)
	case "invoke_parallel":
		result = s.callInvokeParallel(ctx, params.Arguments)
	case "list_agents":
		result = s.callListAgents()
	case "read_file":
		result = s.callReadFile(ctx, params.Arguments)
	case "glob_files":
		result = s.callGlobFiles(ctx, params.Arguments)
	case "grep_files":
		result = s.callGrepFiles(ctx, params.Arguments)
	case "run_command":
		result = s.callRunCommand(ctx, params.Arguments)
	default:
		return s.errorResponse(req.ID, ErrMethodNotFound, "Unknown tool", nil)
	}

	return &Response{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result:  result,
	}
}

// Tool handlers

func (s *Server) callInvokeAgent(ctx context.Context, args map[string]interface{}) CallToolResult {
	agent, _ := args["agent"].(string)
	input, _ := args["input"].(string)

	if agent == "" || input == "" {
		return CallToolResult{
			Content: []ContentBlock{NewErrorContent(fmt.Errorf("agent and input are required"))},
			IsError: true,
		}
	}

	result, err := s.runner.Invoke(ctx, agent, input)
	if err != nil {
		return CallToolResult{
			Content: []ContentBlock{NewErrorContent(err)},
			IsError: true,
		}
	}

	return CallToolResult{
		Content: []ContentBlock{NewTextContent(result.Output)},
		IsError: !result.Success,
	}
}

func (s *Server) callInvokeParallel(ctx context.Context, args map[string]interface{}) CallToolResult {
	agentsStr, _ := args["agents"].(string)
	input, _ := args["input"].(string)

	if agentsStr == "" || input == "" {
		return CallToolResult{
			Content: []ContentBlock{NewErrorContent(fmt.Errorf("agents and input are required"))},
			IsError: true,
		}
	}

	// Parse agent names
	agentNames := strings.Split(agentsStr, ",")
	tasks := make([]local.AgentTask, len(agentNames))
	for i, name := range agentNames {
		tasks[i] = local.AgentTask{
			Agent: strings.TrimSpace(name),
			Input: input,
		}
	}

	results, err := s.runner.InvokeParallel(ctx, tasks)
	if err != nil {
		return CallToolResult{
			Content: []ContentBlock{NewErrorContent(err)},
			IsError: true,
		}
	}

	// Format results
	var output strings.Builder
	hasError := false
	for _, result := range results {
		status := "SUCCESS"
		if !result.Success {
			status = "FAILED"
			hasError = true
		}
		output.WriteString(fmt.Sprintf("## %s [%s]\n\n%s\n\n", result.Agent, status, result.Output))
	}

	return CallToolResult{
		Content: []ContentBlock{NewTextContent(output.String())},
		IsError: hasError,
	}
}

func (s *Server) callListAgents() CallToolResult {
	infos := s.runner.ListAgentInfo()

	var output strings.Builder
	output.WriteString("# Available Agents\n\n")
	for _, info := range infos {
		output.WriteString(fmt.Sprintf("## %s\n%s\n\n", info.Name, info.Description))
	}

	return CallToolResult{
		Content: []ContentBlock{NewTextContent(output.String())},
	}
}

func (s *Server) callReadFile(ctx context.Context, args map[string]interface{}) CallToolResult {
	path, _ := args["path"].(string)
	if path == "" {
		return CallToolResult{
			Content: []ContentBlock{NewErrorContent(fmt.Errorf("path is required"))},
			IsError: true,
		}
	}

	content, err := s.runner.ToolSet().ReadFile(ctx, path)
	if err != nil {
		return CallToolResult{
			Content: []ContentBlock{NewErrorContent(err)},
			IsError: true,
		}
	}

	return CallToolResult{
		Content: []ContentBlock{NewTextContent(content)},
	}
}

func (s *Server) callGlobFiles(ctx context.Context, args map[string]interface{}) CallToolResult {
	pattern, _ := args["pattern"].(string)
	if pattern == "" {
		return CallToolResult{
			Content: []ContentBlock{NewErrorContent(fmt.Errorf("pattern is required"))},
			IsError: true,
		}
	}

	files, err := s.runner.ToolSet().GlobFiles(ctx, pattern)
	if err != nil {
		return CallToolResult{
			Content: []ContentBlock{NewErrorContent(err)},
			IsError: true,
		}
	}

	output := strings.Join(files, "\n")
	if output == "" {
		output = "No files found"
	}

	return CallToolResult{
		Content: []ContentBlock{NewTextContent(output)},
	}
}

func (s *Server) callGrepFiles(ctx context.Context, args map[string]interface{}) CallToolResult {
	pattern, _ := args["pattern"].(string)
	filePattern, _ := args["file_pattern"].(string)

	if pattern == "" {
		return CallToolResult{
			Content: []ContentBlock{NewErrorContent(fmt.Errorf("pattern is required"))},
			IsError: true,
		}
	}

	matches, err := s.runner.ToolSet().GrepFiles(ctx, pattern, filePattern)
	if err != nil {
		return CallToolResult{
			Content: []ContentBlock{NewErrorContent(err)},
			IsError: true,
		}
	}

	var output strings.Builder
	for _, match := range matches {
		output.WriteString(fmt.Sprintf("%s:%d: %s\n", match.File, match.Line, match.Content))
	}

	if output.Len() == 0 {
		output.WriteString("No matches found")
	}

	return CallToolResult{
		Content: []ContentBlock{NewTextContent(output.String())},
	}
}

func (s *Server) callRunCommand(ctx context.Context, args map[string]interface{}) CallToolResult {
	command, _ := args["command"].(string)
	if command == "" {
		return CallToolResult{
			Content: []ContentBlock{NewErrorContent(fmt.Errorf("command is required"))},
			IsError: true,
		}
	}

	result, err := s.runner.ToolSet().RunShell(ctx, command)
	if err != nil {
		return CallToolResult{
			Content: []ContentBlock{NewErrorContent(err)},
			IsError: true,
		}
	}

	var output strings.Builder
	if result.Stdout != "" {
		output.WriteString(result.Stdout)
	}
	if result.Stderr != "" {
		if output.Len() > 0 {
			output.WriteString("\n")
		}
		output.WriteString("STDERR:\n")
		output.WriteString(result.Stderr)
	}

	return CallToolResult{
		Content: []ContentBlock{NewTextContent(output.String())},
		IsError: !result.Success(),
	}
}

// handleResourcesList returns an empty resource list (agents don't expose resources).
func (s *Server) handleResourcesList(req *Request) *Response {
	return &Response{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result:  ListResourcesResult{Resources: []ResourceInfo{}},
	}
}

// handleResourcesRead is not implemented.
func (s *Server) handleResourcesRead(_ context.Context, req *Request) *Response {
	return s.errorResponse(req.ID, ErrMethodNotFound, "Resources not supported", nil)
}

// handlePromptsList returns an empty prompt list.
func (s *Server) handlePromptsList(req *Request) *Response {
	return &Response{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result:  ListPromptsResult{Prompts: []PromptInfo{}},
	}
}

// handlePromptsGet is not implemented.
func (s *Server) handlePromptsGet(req *Request) *Response {
	return s.errorResponse(req.ID, ErrMethodNotFound, "Prompts not supported", nil)
}

// Helper methods

func (s *Server) errorResponse(id json.RawMessage, code int, message string, data interface{}) *Response {
	return &Response{
		JSONRPC: "2.0",
		ID:      id,
		Error: &ErrorResponse{
			Code:    code,
			Message: message,
			Data:    data,
		},
	}
}

func (s *Server) writeResponse(w io.Writer, resp *Response) error {
	data, err := json.Marshal(resp)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(w, string(data))
	return err
}

func (s *Server) writeError(w io.Writer, id json.RawMessage, code int, message string, data interface{}) {
	resp := s.errorResponse(id, code, message, data)
	_ = s.writeResponse(w, resp)
}
