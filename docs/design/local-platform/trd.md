# TRD: AgentKit Local Platform

**Status:** Draft
**Version:** 1.0.0
**Owner:** PlexusOne Team
**Last Updated:** 2025-01-20

## Overview

This Technical Requirements Document specifies the implementation details for the AgentKit Local Platform, enabling local multi-agent orchestration with filesystem access and multi-provider LLM support.

### Execution Modes

AgentKit Local supports **two execution modes** built on a shared core:

| Mode | Binary | Transport | State | Priority |
|------|--------|-----------|-------|----------|
| **CLI** | `agent-cli` | Stdin/stdout | Filesystem | P0 (Phase 1-3) |
| **Service** | `agent-server` | HTTP/gRPC | In-memory + Redis | P1 (Phase 4) |

**Phase 1-3 focus on CLI mode.** Service mode reuses all core components (DAG executor, spec loader, LLM client, state backend) with an HTTP/gRPC transport layer added on top.

## Architecture

### System Context

```
┌─────────────────────────────────────────────────────────────────────┐
│                           User                                       │
│                             │                                        │
│                             ▼                                        │
│                      ┌─────────────┐                                │
│                      │  agent-cli  │                                │
│                      └──────┬──────┘                                │
│                             │                                        │
├─────────────────────────────┼───────────────────────────────────────┤
│                             ▼                                        │
│  ┌─────────────────────────────────────────────────────────────┐   │
│  │                    platforms/local                           │   │
│  │  ┌───────────┐  ┌───────────┐  ┌───────────┐  ┌──────────┐ │   │
│  │  │  Runner   │  │    DAG    │  │   State   │  │   Spec   │ │   │
│  │  │           │  │  Executor │  │  Backend  │  │  Loader  │ │   │
│  │  └─────┬─────┘  └─────┬─────┘  └─────┬─────┘  └────┬─────┘ │   │
│  │        │              │              │             │        │   │
│  │        ▼              ▼              ▼             ▼        │   │
│  │  ┌───────────────────────────────────────────────────────┐ │   │
│  │  │                  EmbeddedAgent                         │ │   │
│  │  │  ┌─────────┐  ┌─────────┐  ┌─────────────────────────┐│ │   │
│  │  │  │  Tools  │  │  LLM    │  │      Agent Loop         ││ │   │
│  │  │  │         │  │ Client  │  │  (tool calling cycle)   ││ │   │
│  │  │  └─────────┘  └────┬────┘  └─────────────────────────┘│ │   │
│  │  └────────────────────┼───────────────────────────────────┘ │   │
│  └───────────────────────┼─────────────────────────────────────┘   │
│                          │                                          │
├──────────────────────────┼──────────────────────────────────────────┤
│                          ▼                                          │
│  ┌─────────────────────────────────────────────────────────────┐   │
│  │                       omnillm                                │   │
│  │  ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────┐ ┌───────┐ │   │
│  │  │Anthropic │ │  OpenAI  │ │  Gemini  │ │ xAI  │ │Ollama │ │   │
│  │  └──────────┘ └──────────┘ └──────────┘ └──────┘ └───────┘ │   │
│  └─────────────────────────────────────────────────────────────┘   │
│                                                                     │
│  ┌─────────────────────────────────────────────────────────────┐   │
│  │                    State Backends                            │   │
│  │  ┌──────────────┐  ┌──────────────┐  ┌──────────────────┐  │   │
│  │  │  Filesystem  │  │    Redis     │  │    In-Memory     │  │   │
│  │  │ .agent-state │  │  (optional)  │  │    (default)     │  │   │
│  │  └──────────────┘  └──────────────┘  └──────────────────┘  │   │
│  └─────────────────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────────────────┘
```

### Component Architecture

```
platforms/local/
├── cmd/
│   ├── agent-cli/
│   │   └── main.go           # CLI entry point (NEW - Phase 1)
│   └── agent-server/
│       └── main.go           # Service entry point (NEW - Phase 4)
│
├── # Core (shared between CLI and Service)
├── agent.go                  # EmbeddedAgent (EXISTS)
├── config.go                 # Config loading (EXISTS)
├── tools.go                  # Tool implementations (EXISTS)
├── runner.go                 # Orchestration (EXISTS - extend)
├── dag.go                    # DAG executor (NEW)
├── llm.go                    # omnillm LLMClient (NEW)
├── state.go                  # State backend interface (NEW)
├── state_file.go             # Filesystem state (NEW)
├── state_redis.go            # Redis state (NEW)
├── spec.go                   # multi-agent-spec loader (NEW)
├── mapping.go                # Tool/model mapping (NEW)
├── report.go                 # Report generation (NEW)
│
├── # Service-specific (Phase 4)
├── server.go                 # HTTP server (NEW - Phase 4)
├── handlers.go               # API handlers (NEW - Phase 4)
├── streaming.go              # SSE/WebSocket (NEW - Phase 4)
│
└── schema.json               # Config schema (EXISTS)
```

**Dependency Flow:**

```
┌─────────────┐     ┌──────────────┐
│  agent-cli  │     │ agent-server │
└──────┬──────┘     └──────┬───────┘
       │                   │
       └─────────┬─────────┘
                 ▼
    ┌────────────────────────┐
    │      Shared Core       │
    │  ┌──────┐ ┌──────────┐│
    │  │ DAG  │ │  Spec    ││
    │  │Exec  │ │  Loader  ││
    │  └──────┘ └──────────┘│
    │  ┌──────┐ ┌──────────┐│
    │  │State │ │  LLM     ││
    │  │Backend│ │ Client  ││
    │  └──────┘ └──────────┘│
    │  ┌──────┐ ┌──────────┐│
    │  │Report│ │ Mapping  ││
    │  │ Gen  │ │(tool/mdl)││
    │  └──────┘ └──────────┘│
    └────────────────────────┘
                 │
                 ▼
    ┌────────────────────────┐
    │  Existing Components   │
    │  agent.go, runner.go   │
    │  tools.go, config.go   │
    └────────────────────────┘
```

## Multi-Agent Spec Compliance

AgentKit Local implements the [multi-agent-spec](https://github.com/plexusone/multi-agent-spec) schema for portable workflow definitions.

### Schema References

| Schema | Path | Purpose |
|--------|------|---------|
| Agent | `schema/agent/agent.schema.json` | Agent definition with YAML frontmatter |
| Team | `schema/orchestration/team.schema.json` | Team composition and DAG workflow |
| Deployment | `schema/deployment/deployment.schema.json` | Runtime configuration |
| Agent Result | `schema/report/agent-result.schema.json` | Per-agent execution result |
| Team Report | `schema/report/team-report.schema.json` | Workflow execution report |

### Platform Configuration

AgentKit Local is registered as `agentkit-local` platform in multi-agent-spec:

```json
{
  "name": "local-dev",
  "platform": "agentkit-local",
  "mode": "single-process",
  "priority": "p1",
  "runtime": {
    "defaults": {
      "timeout": "5m",
      "retry": {"max_attempts": 2, "backoff": "exponential"}
    }
  },
  "config": {
    "workspace": ".",
    "llm": {
      "provider": "anthropic",
      "model": "claude-sonnet-4-20250514"
    },
    "mcp": {"enabled": true, "transport": "stdio"}
  }
}
```

### Tool Mapping Implementation

```go
// CanonicalToolMap maps multi-agent-spec tools to AgentKit tools
var CanonicalToolMap = map[string]string{
    "Read":      "read",
    "Write":     "write",
    "Edit":      "write",  // Edit uses write with diff
    "Glob":      "glob",
    "Grep":      "grep",
    "Bash":      "shell",
    "WebSearch": "shell",  // Via curl/external
    "WebFetch":  "shell",  // Via curl/external
    "Task":      "",       // Handled by orchestration
}

func mapCanonicalTools(canonical []string) []string {
    var local []string
    for _, tool := range canonical {
        if mapped, ok := CanonicalToolMap[tool]; ok && mapped != "" {
            local = append(local, mapped)
        }
    }
    return local
}
```

### Model Mapping Implementation

```go
// CanonicalModelMap maps multi-agent-spec models to provider-specific IDs
var CanonicalModelMap = map[string]map[string]string{
    "haiku": {
        "anthropic": "claude-3-5-haiku-20241022",
        "openai":    "gpt-4o-mini",
        "gemini":    "gemini-2.0-flash",
    },
    "sonnet": {
        "anthropic": "claude-sonnet-4-20250514",
        "openai":    "gpt-4o",
        "gemini":    "gemini-2.5-pro",
    },
    "opus": {
        "anthropic": "claude-opus-4-20250514",
        "openai":    "gpt-4.5",
        "gemini":    "gemini-2.5-pro",
    },
}

func mapCanonicalModel(canonical, provider string) string {
    if models, ok := CanonicalModelMap[canonical]; ok {
        if model, ok := models[provider]; ok {
            return model
        }
    }
    return canonical // Pass through if not canonical
}
```

## Technical Specifications

### 1. CLI Interface (`cmd/agent-cli/main.go`)

#### Commands

```go
// Command structure using urfave/cli/v2
app := &cli.App{
    Name:  "agent-cli",
    Usage: "Run AI agents locally with multi-provider LLM support",
    Commands: []*cli.Command{
        {
            Name:   "run",
            Usage:  "Run a single agent",
            Action: runAgent,
            Flags: []cli.Flag{
                &cli.StringFlag{Name: "agent", Aliases: []string{"a"}, Required: true},
                &cli.StringFlag{Name: "task", Aliases: []string{"t"}, Required: true},
                &cli.StringFlag{Name: "config", Aliases: []string{"c"}, Value: "agentkit.yaml"},
                &cli.StringFlag{Name: "session", Aliases: []string{"s"}},
                &cli.StringFlag{Name: "provider", Value: "anthropic"},
                &cli.StringFlag{Name: "model"},
                &cli.StringFlag{Name: "output", Aliases: []string{"o"}, Value: "text"},
            },
        },
        {
            Name:   "workflow",
            Usage:  "Run a multi-agent workflow",
            Action: runWorkflow,
            Flags: []cli.Flag{
                &cli.StringFlag{Name: "spec", Required: true},
                &cli.StringFlag{Name: "steps", Usage: "Comma-separated step IDs to run"},
                &cli.StringFlag{Name: "config", Aliases: []string{"c"}, Value: "agentkit.yaml"},
                &cli.StringFlag{Name: "session", Aliases: []string{"s"}},
                &cli.StringFlag{Name: "provider", Value: "anthropic"},
                &cli.BoolFlag{Name: "dry-run"},
            },
        },
        {
            Name:   "list",
            Usage:  "List available agents",
            Action: listAgents,
        },
    },
}
```

#### Output Formats

| Format | Description | Use Case |
|--------|-------------|----------|
| `text` | Human-readable terminal output | Interactive use |
| `json` | Structured JSON output | Programmatic consumption |
| `toon` | Token-optimized notation | LLM consumption (8x smaller) |

### 2. DAG Executor (`dag.go`)

#### Workflow Specification Format

```json
{
  "name": "prd-workflow",
  "version": "1.0.0",
  "steps": [
    {
      "id": "problem-discovery",
      "agent": "problem-discovery",
      "depends_on": [],
      "timeout": "5m",
      "retry": {"max_attempts": 2, "backoff": "exponential"}
    },
    {
      "id": "user-research",
      "agent": "user-research",
      "depends_on": [],
      "timeout": "5m"
    },
    {
      "id": "solution-ideation",
      "agent": "solution-ideation",
      "depends_on": ["problem-discovery", "user-research"],
      "input_mapping": {
        "problem": "$.problem-discovery.output.problem_statement",
        "users": "$.user-research.output.personas"
      }
    }
  ]
}
```

#### DAG Execution Algorithm

```go
// DAGExecutor manages workflow execution
type DAGExecutor struct {
    steps    map[string]*WorkflowStep
    runner   *Runner
    state    StateBackend
    logger   *slog.Logger
}

// Execute runs the workflow respecting dependencies
func (e *DAGExecutor) Execute(ctx context.Context, workflowPath string) (*WorkflowResult, error) {
    workflow, err := e.loadWorkflow(workflowPath)
    if err != nil {
        return nil, err
    }

    // Validate DAG (check for cycles)
    if err := e.validateDAG(workflow); err != nil {
        return nil, err
    }

    // Topological sort
    order := e.topologicalSort(workflow.Steps)

    // Group by dependency level for parallel execution
    levels := e.groupByLevel(order, workflow.Steps)

    results := make(map[string]*StepResult)

    for _, level := range levels {
        // Execute all steps in this level in parallel
        levelResults, err := e.executeLevel(ctx, level, results)
        if err != nil {
            return nil, err
        }

        for stepID, result := range levelResults {
            results[stepID] = result
            e.state.Set(ctx, stepID, result)
        }
    }

    return &WorkflowResult{
        Workflow: workflow.Name,
        Steps:    results,
        Success:  e.allSuccessful(results),
    }, nil
}

// topologicalSort returns steps in dependency order
func (e *DAGExecutor) topologicalSort(steps []WorkflowStep) []string {
    // Kahn's algorithm
    inDegree := make(map[string]int)
    graph := make(map[string][]string)

    for _, step := range steps {
        inDegree[step.ID] = len(step.DependsOn)
        for _, dep := range step.DependsOn {
            graph[dep] = append(graph[dep], step.ID)
        }
    }

    var queue []string
    for _, step := range steps {
        if inDegree[step.ID] == 0 {
            queue = append(queue, step.ID)
        }
    }

    var order []string
    for len(queue) > 0 {
        node := queue[0]
        queue = queue[1:]
        order = append(order, node)

        for _, neighbor := range graph[node] {
            inDegree[neighbor]--
            if inDegree[neighbor] == 0 {
                queue = append(queue, neighbor)
            }
        }
    }

    return order
}

// groupByLevel groups steps that can run in parallel
func (e *DAGExecutor) groupByLevel(order []string, steps []WorkflowStep) [][]string {
    stepMap := make(map[string]*WorkflowStep)
    for i := range steps {
        stepMap[steps[i].ID] = &steps[i]
    }

    levels := make(map[string]int)
    for _, stepID := range order {
        step := stepMap[stepID]
        maxDepLevel := -1
        for _, dep := range step.DependsOn {
            if levels[dep] > maxDepLevel {
                maxDepLevel = levels[dep]
            }
        }
        levels[stepID] = maxDepLevel + 1
    }

    // Group by level
    maxLevel := 0
    for _, level := range levels {
        if level > maxLevel {
            maxLevel = level
        }
    }

    result := make([][]string, maxLevel+1)
    for stepID, level := range levels {
        result[level] = append(result[level], stepID)
    }

    return result
}
```

### 3. omnillm Integration (`llm.go`)

#### LLMClient Implementation

```go
package local

import (
    "context"
    "github.com/plexusone/omnillm"
)

// OmniLLMClient implements LLMClient using omnillm
type OmniLLMClient struct {
    client *omnillm.ChatClient
    model  string
}

// NewOmniLLMClient creates a new omnillm-based LLM client
func NewOmniLLMClient(cfg LLMConfig) (*OmniLLMClient, error) {
    providerCfg := omnillm.ProviderConfig{
        Provider: toOmniProvider(cfg.Provider),
        APIKey:   resolveEnvVar(cfg.APIKey),
    }
    if cfg.BaseURL != "" {
        providerCfg.BaseURL = cfg.BaseURL
    }

    client, err := omnillm.NewClient(omnillm.ClientConfig{
        Providers: []omnillm.ProviderConfig{providerCfg},
    })
    if err != nil {
        return nil, fmt.Errorf("failed to create omnillm client: %w", err)
    }

    return &OmniLLMClient{
        client: client,
        model:  cfg.Model,
    }, nil
}

// Complete generates a completion for the given messages
func (c *OmniLLMClient) Complete(ctx context.Context, messages []Message, tools []ToolDefinition) (*CompletionResponse, error) {
    // Convert to omnillm format
    omniMessages := make([]omnillm.Message, len(messages))
    for i, msg := range messages {
        omniMessages[i] = omnillm.Message{
            Role:       msg.Role,
            Content:    msg.Content,
            ToolCallID: msg.ToolID,
        }
    }

    // Convert tools
    omniTools := make([]omnillm.Tool, len(tools))
    for i, tool := range tools {
        omniTools[i] = omnillm.Tool{
            Type: "function",
            Function: omnillm.Function{
                Name:        tool.Name,
                Description: tool.Description,
                Parameters:  tool.Parameters,
            },
        }
    }

    // Make request
    resp, err := c.client.CreateChatCompletion(ctx, &omnillm.ChatCompletionRequest{
        Model:    c.model,
        Messages: omniMessages,
        Tools:    omniTools,
    })
    if err != nil {
        return nil, err
    }

    // Convert response
    result := &CompletionResponse{
        Content: resp.Choices[0].Message.Content,
        Done:    true,
    }

    // Convert tool calls
    for _, tc := range resp.Choices[0].Message.ToolCalls {
        result.ToolCalls = append(result.ToolCalls, ToolCall{
            ID:        tc.ID,
            Name:      tc.Function.Name,
            Arguments: parseArguments(tc.Function.Arguments),
        })
        result.Done = false
    }

    return result, nil
}

func toOmniProvider(provider string) omnillm.ProviderName {
    switch provider {
    case "anthropic":
        return omnillm.ProviderNameAnthropic
    case "openai":
        return omnillm.ProviderNameOpenAI
    case "gemini":
        return omnillm.ProviderNameGemini
    case "xai":
        return omnillm.ProviderNameXAI
    case "ollama":
        return omnillm.ProviderNameOllama
    default:
        return omnillm.ProviderNameOpenAI
    }
}
```

### 4. State Backend Interface (`state.go`)

```go
package local

import (
    "context"
    "encoding/json"
)

// StateBackend defines the interface for state persistence
type StateBackend interface {
    // Get retrieves a value by key
    Get(ctx context.Context, key string) (json.RawMessage, error)

    // Set stores a value by key
    Set(ctx context.Context, key string, value any) error

    // Delete removes a value by key
    Delete(ctx context.Context, key string) error

    // List returns all keys with optional prefix filter
    List(ctx context.Context, prefix string) ([]string, error)

    // Close cleans up resources
    Close() error
}

// Session represents a workflow session with state
type Session struct {
    ID        string       `json:"id"`
    Workspace string       `json:"workspace"`
    State     StateBackend `json:"-"`
    CreatedAt time.Time    `json:"created_at"`
    UpdatedAt time.Time    `json:"updated_at"`
}

// NewSession creates a new session with the specified backend
func NewSession(id, workspace string, backend StateBackend) *Session {
    now := time.Now()
    return &Session{
        ID:        id,
        Workspace: workspace,
        State:     backend,
        CreatedAt: now,
        UpdatedAt: now,
    }
}
```

#### Filesystem Backend (`state_file.go`)

```go
package local

import (
    "context"
    "encoding/json"
    "os"
    "path/filepath"
)

// FileStateBackend stores state in filesystem
type FileStateBackend struct {
    baseDir string
}

// NewFileStateBackend creates a filesystem-based state backend
func NewFileStateBackend(sessionID, workspace string) (*FileStateBackend, error) {
    baseDir := filepath.Join(workspace, ".agent-state", sessionID)
    if err := os.MkdirAll(baseDir, 0755); err != nil {
        return nil, fmt.Errorf("failed to create state directory: %w", err)
    }
    return &FileStateBackend{baseDir: baseDir}, nil
}

func (f *FileStateBackend) Get(ctx context.Context, key string) (json.RawMessage, error) {
    path := filepath.Join(f.baseDir, key+".json")
    data, err := os.ReadFile(path)
    if os.IsNotExist(err) {
        return nil, nil
    }
    if err != nil {
        return nil, err
    }
    return json.RawMessage(data), nil
}

func (f *FileStateBackend) Set(ctx context.Context, key string, value any) error {
    data, err := json.MarshalIndent(value, "", "  ")
    if err != nil {
        return err
    }
    path := filepath.Join(f.baseDir, key+".json")
    return os.WriteFile(path, data, 0644)
}

func (f *FileStateBackend) Delete(ctx context.Context, key string) error {
    path := filepath.Join(f.baseDir, key+".json")
    return os.Remove(path)
}

func (f *FileStateBackend) List(ctx context.Context, prefix string) ([]string, error) {
    entries, err := os.ReadDir(f.baseDir)
    if err != nil {
        return nil, err
    }

    var keys []string
    for _, entry := range entries {
        if entry.IsDir() {
            continue
        }
        name := strings.TrimSuffix(entry.Name(), ".json")
        if prefix == "" || strings.HasPrefix(name, prefix) {
            keys = append(keys, name)
        }
    }
    return keys, nil
}

func (f *FileStateBackend) Close() error {
    return nil
}
```

### 5. Specification Loader (`spec.go`) - multi-agent-spec Compliant

```go
package local

import (
    "bytes"
    "os"
    "path/filepath"
    "strings"

    "gopkg.in/yaml.v3"
)

// SpecLoader loads multi-agent-spec compliant specifications
type SpecLoader struct {
    specsDir string
    provider string // For model mapping
}

// NewSpecLoader creates a new spec loader
func NewSpecLoader(specsDir, provider string) *SpecLoader {
    return &SpecLoader{specsDir: specsDir, provider: provider}
}

// AgentFrontmatter represents YAML frontmatter in agent markdown (multi-agent-spec)
type AgentFrontmatter struct {
    Name         string   `yaml:"name"`
    Description  string   `yaml:"description"`
    Model        string   `yaml:"model"`        // Canonical: haiku, sonnet, opus
    Tools        []string `yaml:"tools"`        // Canonical: Read, Write, Glob, Grep, Bash
    Dependencies []string `yaml:"dependencies"` // Agents this can spawn
    Requires     []string `yaml:"requires"`     // External binaries
    Instructions string   `yaml:"instructions"` // Optional inline instructions
    Tasks        []Task   `yaml:"tasks"`        // Validation tasks
}

// Task represents a validation task in agent spec
type Task struct {
    ID             string `yaml:"id"`
    Description    string `yaml:"description"`
    Type           string `yaml:"type"` // command, pattern, file, manual
    Command        string `yaml:"command,omitempty"`
    Pattern        string `yaml:"pattern,omitempty"`
    File           string `yaml:"file,omitempty"`
    Files          string `yaml:"files,omitempty"`
    Required       bool   `yaml:"required"`
    ExpectedOutput string `yaml:"expected_output,omitempty"`
    HumanInLoop    string `yaml:"human_in_loop,omitempty"`
}

// LoadAgentFromMarkdown parses multi-agent-spec markdown with YAML frontmatter
func (l *SpecLoader) LoadAgentFromMarkdown(path string) (*AgentConfig, error) {
    content, err := os.ReadFile(path)
    if err != nil {
        return nil, err
    }

    // Parse YAML frontmatter (between --- markers)
    var frontmatter AgentFrontmatter
    var body string

    if bytes.HasPrefix(content, []byte("---\n")) {
        parts := bytes.SplitN(content[4:], []byte("\n---"), 2)
        if len(parts) == 2 {
            if err := yaml.Unmarshal(parts[0], &frontmatter); err != nil {
                return nil, fmt.Errorf("failed to parse frontmatter: %w", err)
            }
            body = string(parts[1])
        }
    } else {
        body = string(content)
    }

    // Use frontmatter name or derive from filename
    name := frontmatter.Name
    if name == "" {
        name = strings.TrimSuffix(filepath.Base(path), ".md")
    }

    // Map canonical tools to local tools
    localTools := mapCanonicalTools(frontmatter.Tools)

    // Map canonical model to provider-specific model
    model := mapCanonicalModel(frontmatter.Model, l.provider)

    // Use frontmatter instructions or markdown body
    instructions := frontmatter.Instructions
    if instructions == "" {
        instructions = strings.TrimSpace(body)
    }

    return &AgentConfig{
        Name:         name,
        Description:  frontmatter.Description,
        Instructions: instructions,
        Tools:        localTools,
        Model:        model,
    }, nil
}

// TeamSpec represents multi-agent-spec team.schema.json
type TeamSpec struct {
    Name         string       `json:"name" yaml:"name"`
    Version      string       `json:"version" yaml:"version"`
    Description  string       `json:"description" yaml:"description"`
    Agents       []string     `json:"agents" yaml:"agents"`
    Orchestrator string       `json:"orchestrator" yaml:"orchestrator"`
    Context      string       `json:"context" yaml:"context"`
    Workflow     WorkflowSpec `json:"workflow" yaml:"workflow"`
}

// WorkflowSpec represents workflow configuration
type WorkflowSpec struct {
    Type  string         `json:"type" yaml:"type"` // orchestrated, sequential, parallel, dag
    Steps []WorkflowStep `json:"steps" yaml:"steps"`
}

// WorkflowStep represents a step in multi-agent-spec workflow
type WorkflowStep struct {
    Name      string      `json:"name" yaml:"name"`
    Agent     string      `json:"agent" yaml:"agent"`
    DependsOn []string    `json:"depends_on" yaml:"depends_on"`
    Inputs    []PortSpec  `json:"inputs" yaml:"inputs"`
    Outputs   []PortSpec  `json:"outputs" yaml:"outputs"`
    Timeout   string      `json:"timeout,omitempty" yaml:"timeout,omitempty"`
    Retry     *RetrySpec  `json:"retry,omitempty" yaml:"retry,omitempty"`
    Condition string      `json:"condition,omitempty" yaml:"condition,omitempty"`
}

// PortSpec represents typed input/output ports
type PortSpec struct {
    Name        string      `json:"name" yaml:"name"`
    Type        string      `json:"type" yaml:"type"` // string, number, boolean, object, array, file
    Description string      `json:"description" yaml:"description"`
    Required    bool        `json:"required" yaml:"required"`
    From        string      `json:"from,omitempty" yaml:"from,omitempty"` // e.g., "step-name.output_port"
    Schema      interface{} `json:"schema,omitempty" yaml:"schema,omitempty"`
    Default     interface{} `json:"default,omitempty" yaml:"default,omitempty"`
}

// RetrySpec represents retry configuration
type RetrySpec struct {
    MaxAttempts     int      `json:"max_attempts" yaml:"max_attempts"`
    Backoff         string   `json:"backoff" yaml:"backoff"` // exponential, fixed, linear
    InitialDelay    string   `json:"initial_delay" yaml:"initial_delay"`
    MaxDelay        string   `json:"max_delay" yaml:"max_delay"`
    RetryableErrors []string `json:"retryable_errors" yaml:"retryable_errors"`
}

// DeploymentSpec represents multi-agent-spec deployment.schema.json
type DeploymentSpec struct {
    Team    string         `json:"team" yaml:"team"`
    Targets []TargetSpec   `json:"targets" yaml:"targets"`
}

// TargetSpec represents a deployment target
type TargetSpec struct {
    Name     string           `json:"name" yaml:"name"`
    Platform string           `json:"platform" yaml:"platform"` // agentkit-local
    Mode     string           `json:"mode" yaml:"mode"`         // single-process
    Priority string           `json:"priority" yaml:"priority"` // p1, p2, p3
    Output   string           `json:"output" yaml:"output"`
    Runtime  RuntimeSpec      `json:"runtime" yaml:"runtime"`
    Config   LocalConfigSpec  `json:"config" yaml:"config"`
}

// RuntimeSpec represents runtime configuration
type RuntimeSpec struct {
    Defaults      StepRuntimeSpec            `json:"defaults" yaml:"defaults"`
    Steps         map[string]StepRuntimeSpec `json:"steps" yaml:"steps"`
    Observability ObservabilitySpec          `json:"observability" yaml:"observability"`
}

// StepRuntimeSpec represents per-step runtime config
type StepRuntimeSpec struct {
    Timeout     string        `json:"timeout" yaml:"timeout"`
    Retry       *RetrySpec    `json:"retry" yaml:"retry"`
    Condition   string        `json:"condition" yaml:"condition"`
    Concurrency int           `json:"concurrency" yaml:"concurrency"`
    Resources   ResourcesSpec `json:"resources" yaml:"resources"`
}

// ResourcesSpec represents resource limits
type ResourcesSpec struct {
    CPU    string `json:"cpu" yaml:"cpu"`
    Memory string `json:"memory" yaml:"memory"`
    GPU    int    `json:"gpu" yaml:"gpu"`
}

// ObservabilitySpec represents observability configuration
type ObservabilitySpec struct {
    Tracing TracingSpec `json:"tracing" yaml:"tracing"`
    Metrics MetricsSpec `json:"metrics" yaml:"metrics"`
    Logging LoggingSpec `json:"logging" yaml:"logging"`
}

// TracingSpec, MetricsSpec, LoggingSpec for observability
type TracingSpec struct {
    Enabled    bool    `json:"enabled" yaml:"enabled"`
    Exporter   string  `json:"exporter" yaml:"exporter"`
    Endpoint   string  `json:"endpoint" yaml:"endpoint"`
    SampleRate float64 `json:"sample_rate" yaml:"sample_rate"`
}

type MetricsSpec struct {
    Enabled  bool   `json:"enabled" yaml:"enabled"`
    Exporter string `json:"exporter" yaml:"exporter"`
    Endpoint string `json:"endpoint" yaml:"endpoint"`
}

type LoggingSpec struct {
    Level  string `json:"level" yaml:"level"`
    Format string `json:"format" yaml:"format"`
}

// LocalConfigSpec represents agentkit-local specific config
type LocalConfigSpec struct {
    Workspace string     `json:"workspace" yaml:"workspace"`
    LLM       LLMSpec    `json:"llm" yaml:"llm"`
    MCP       MCPSpec    `json:"mcp" yaml:"mcp"`
}

type LLMSpec struct {
    Provider string `json:"provider" yaml:"provider"`
    Model    string `json:"model" yaml:"model"`
    APIKey   string `json:"api_key" yaml:"api_key"` // ${ENV_VAR} syntax
}

type MCPSpec struct {
    Enabled   bool   `json:"enabled" yaml:"enabled"`
    Transport string `json:"transport" yaml:"transport"`
}

// LoadTeam loads a multi-agent-spec team definition
func (l *SpecLoader) LoadTeam(path string) (*TeamSpec, error) {
    data, err := os.ReadFile(path)
    if err != nil {
        return nil, err
    }

    var team TeamSpec
    if err := json.Unmarshal(data, &team); err != nil {
        return nil, fmt.Errorf("failed to parse team spec: %w", err)
    }

    return &team, nil
}

// LoadDeployment loads a multi-agent-spec deployment definition
func (l *SpecLoader) LoadDeployment(path string) (*DeploymentSpec, error) {
    data, err := os.ReadFile(path)
    if err != nil {
        return nil, err
    }

    var deployment DeploymentSpec
    if err := json.Unmarshal(data, &deployment); err != nil {
        return nil, fmt.Errorf("failed to parse deployment spec: %w", err)
    }

    return &deployment, nil
}

// GetLocalTarget returns the agentkit-local target from deployment spec
func (d *DeploymentSpec) GetLocalTarget() (*TargetSpec, error) {
    for i := range d.Targets {
        if d.Targets[i].Platform == "agentkit-local" {
            return &d.Targets[i], nil
        }
    }
    return nil, fmt.Errorf("no agentkit-local target found in deployment")
}
```

### 6. Report Generation (`report.go`) - multi-agent-spec Compliant

```go
package local

import (
    "time"
)

// Status represents NASA Go/No-Go terminology
type Status string

const (
    StatusGO   Status = "GO"
    StatusWARN Status = "WARN"
    StatusNOGO Status = "NO-GO"
    StatusSKIP Status = "SKIP"
)

// AgentResultReport conforms to agent-result.schema.json
type AgentResultReport struct {
    AgentID    string                 `json:"agent_id"`
    StepID     string                 `json:"step_id"`
    Inputs     map[string]interface{} `json:"inputs"`
    Outputs    map[string]interface{} `json:"outputs"`
    Tasks      []TaskResult           `json:"tasks"`
    Status     Status                 `json:"status"`
    ExecutedAt time.Time              `json:"executed_at"`
    AgentModel string                 `json:"agent_model"`
    Duration   string                 `json:"duration"`
    Error      string                 `json:"error,omitempty"`
}

// TaskResult represents a validation task result
type TaskResult struct {
    ID         string                 `json:"id"`
    Status     Status                 `json:"status"`
    Detail     string                 `json:"detail"`
    DurationMS int64                  `json:"duration_ms"`
    Metadata   map[string]interface{} `json:"metadata,omitempty"`
}

// TeamReport conforms to team-report.schema.json
type TeamReport struct {
    Project     string            `json:"project"`
    Version     string            `json:"version"`
    Phase       string            `json:"phase"`
    Teams       []TeamMemberReport `json:"teams"`
    Status      Status            `json:"status"`
    GeneratedAt time.Time         `json:"generated_at"`
    GeneratedBy string            `json:"generated_by"`
}

// TeamMemberReport represents a team member's result
type TeamMemberReport struct {
    ID        string       `json:"id"`
    Name      string       `json:"name"`
    AgentID   string       `json:"agent_id"`
    Model     string       `json:"model"`
    DependsOn []string     `json:"depends_on"`
    Tasks     []TaskResult `json:"tasks"`
    Status    Status       `json:"status"`
}

// AggregateStatus determines overall status from member statuses
func AggregateStatus(members []TeamMemberReport) Status {
    hasWarn := false
    for _, m := range members {
        if m.Status == StatusNOGO {
            return StatusNOGO
        }
        if m.Status == StatusWARN {
            hasWarn = true
        }
    }
    if hasWarn {
        return StatusWARN
    }
    return StatusGO
}

// GenerateTeamReport creates a multi-agent-spec compliant report
func GenerateTeamReport(project, version, phase string, results map[string]*AgentResultReport) *TeamReport {
    var members []TeamMemberReport
    for stepID, result := range results {
        members = append(members, TeamMemberReport{
            ID:      stepID,
            Name:    result.AgentID,
            AgentID: result.AgentID,
            Model:   result.AgentModel,
            Tasks:   result.Tasks,
            Status:  result.Status,
        })
    }

    return &TeamReport{
        Project:     project,
        Version:     version,
        Phase:       phase,
        Teams:       members,
        Status:      AggregateStatus(members),
        GeneratedAt: time.Now(),
        GeneratedBy: "agentkit-local",
    }
}
```

## Comparison: Implementation Approach vs Google ADK

### Feature Comparison

| Feature | Google ADK | AgentKit Local | Notes |
|---------|------------|----------------|-------|
| **Agent Loop** | Built-in with transfer support | Built-in (agent.go:105-169) | Both complete |
| **Tool System** | Manual implementation required | Built-in (tools.go) | AgentKit advantage |
| **Sequential Execution** | `SequentialAgent` | `Runner.InvokeSequential` | Both complete |
| **Parallel Execution** | `ParallelAgent` | `Runner.InvokeParallel` | Both complete |
| **DAG Execution** | Not available | `DAGExecutor` (new) | AgentKit advantage |
| **LLM Provider** | Gemini SDK only | omnillm (5+ providers) | AgentKit advantage |
| **Session State** | `session.State` | `StateBackend` interface | Equivalent |
| **Memory (long-term)** | `memory.Service` | Planned | ADK advantage |
| **CLI Tool** | Not available | `agent-cli` (new) | AgentKit advantage |
| **MCP Server** | Built-in | Built-in (config.go) | Both complete |
| **A2A Protocol** | Built-in | Via agentkit/a2a | Both complete |
| **Spec Loading** | Code-only | JSON/YAML/Markdown | AgentKit advantage |

### Why Build Our Own

| Reason | Justification |
|--------|---------------|
| **LLM Vendor Lock-in** | ADK is Gemini-focused. Enterprise customers require multi-provider support. omnillm already supports 5+ providers with fallback. |
| **No DAG Support** | ADK only provides sequential/parallel/loop. Complex workflows like PRD creation require arbitrary dependency graphs. |
| **No CLI Tool** | ADK is a library only. We need a standalone CLI for developer workflows and CI/CD integration. |
| **Code-Only Config** | ADK requires Go code to define agents. We need spec files (JSON/YAML/Markdown) for non-developer workflow definition. |
| **Existing Investment** | `platforms/local/` already has agent loop, tools, config, runner. Building DAG executor and CLI is incremental. |

### What We Reuse from ADK Patterns

| Pattern | Usage |
|---------|-------|
| Agent interface design | `EmbeddedAgent` follows similar patterns |
| Session state concept | `StateBackend` mirrors `session.State` |
| Tool definition schema | JSON Schema for parameters |
| Event-based execution | `AgentResult` for step outcomes |

## Testing Strategy

### Unit Tests

| Component | Test Coverage Target |
|-----------|---------------------|
| DAG executor | 90% - cycle detection, topological sort, parallel grouping |
| State backends | 90% - all CRUD operations |
| Spec loader | 80% - JSON, YAML, Markdown parsing |
| omnillm client | 70% - mock LLM responses |

### Integration Tests

| Scenario | Description |
|----------|-------------|
| Single agent execution | Run agent with real LLM (Claude Haiku for speed) |
| DAG workflow | Execute 3-step DAG with dependencies |
| State persistence | Verify state survives CLI restart |
| Provider fallback | Primary fails, fallback succeeds |

### Example Workflows for Testing

```yaml
# test/workflows/simple-dag.yaml
name: simple-dag-test
steps:
  - id: step-a
    agent: echo-agent
    depends_on: []
  - id: step-b
    agent: echo-agent
    depends_on: []
  - id: step-c
    agent: echo-agent
    depends_on: [step-a, step-b]
```

## Security Considerations

| Concern | Mitigation |
|---------|------------|
| Shell injection | Workspace sandboxing in tools.go (validatePath) |
| Secret exposure | API keys via env vars, never in spec files |
| Filesystem escape | Path validation prevents `..` traversal |
| Runaway execution | Timeout configuration per step |

## Implementation Plan

### Phase 1: CLI Foundation (P0)

- [ ] `cmd/agent-cli/main.go` - CLI entry point with urfave/cli
- [ ] `llm.go` - omnillm LLMClient implementation
- [ ] `state_file.go` - Filesystem state backend
- [ ] `mapping.go` - Canonical tool/model mapping
- [ ] `spec.go` - multi-agent-spec agent loading (Markdown + YAML frontmatter)
- [ ] Unit tests for new components

### Phase 2: DAG Orchestration (P0)

- [ ] `dag.go` - DAG executor with topological sort and parallel execution
- [ ] `spec.go` - multi-agent-spec team/workflow loading
- [ ] `report.go` - multi-agent-spec report generation (GO/WARN/NO-GO)
- [ ] Typed port data flow between steps
- [ ] Integration tests with sample workflows
- [ ] Documentation

### Phase 3: Enhanced CLI (P1)

- [ ] `state_redis.go` - Redis state backend
- [ ] `spec.go` - multi-agent-spec deployment config loading
- [ ] `agent-cli validate` command for spec validation
- [ ] Conditional step execution
- [ ] CI/CD integration examples

### Phase 4: Service Mode (P1)

- [ ] `cmd/agent-server/main.go` - HTTP server entry point
- [ ] `server.go` - HTTP server with graceful shutdown
- [ ] `handlers.go` - REST API handlers
  - `POST /workflows` - Submit workflow
  - `GET /workflows/:id` - Get workflow status
  - `GET /workflows/:id/stream` - SSE stream
  - `POST /agents/:name/invoke` - Invoke single agent
- [ ] `streaming.go` - SSE/WebSocket for real-time output
- [ ] Session management with warm LLM connections
- [ ] Health check endpoint

### Phase 5: Polish (P2)

- [ ] `agent-cli interactive` - REPL mode
- [ ] Tool approval workflow
- [ ] Custom tool registration API
- [ ] MCP server integration

## Dependencies

### New Dependencies

| Package | Version | Purpose |
|---------|---------|---------|
| `github.com/urfave/cli/v2` | v2.27+ | CLI framework |
| `github.com/redis/go-redis/v9` | v9.x | Redis client (optional) |

### Existing Dependencies (Already in agentkit)

| Package | Purpose |
|---------|---------|
| `github.com/plexusone/omnillm` | Multi-provider LLM |
| `gopkg.in/yaml.v3` | YAML parsing |

## File Changes Summary

### Phase 1-3: CLI Mode (~1,130 lines)

| File | Action | Lines (Est.) |
|------|--------|--------------|
| `cmd/agent-cli/main.go` | Create | 150 |
| `dag.go` | Create | 200 |
| `llm.go` | Create | 100 |
| `state.go` | Create | 50 |
| `state_file.go` | Create | 80 |
| `state_redis.go` | Create | 100 |
| `spec.go` | Create | 250 (multi-agent-spec compliant) |
| `report.go` | Create | 100 (multi-agent-spec report generation) |
| `mapping.go` | Create | 50 (tool/model mapping) |
| `runner.go` | Extend | +50 |
| **Subtotal** | | **~1,130 lines** |

### Phase 4: Service Mode (~400 lines)

| File | Action | Lines (Est.) |
|------|--------|--------------|
| `cmd/agent-server/main.go` | Create | 100 |
| `server.go` | Create | 150 |
| `handlers.go` | Create | 100 |
| `streaming.go` | Create | 50 |
| **Subtotal** | | **~400 lines** |

### Total: ~1,530 lines

## multi-agent-spec Integration Points

| Component | Schema | Implementation |
|-----------|--------|----------------|
| Agent loading | `agent.schema.json` | `spec.go:LoadAgentFromMarkdown()` |
| Team loading | `team.schema.json` | `spec.go:LoadTeam()` |
| Deployment loading | `deployment.schema.json` | `spec.go:LoadDeployment()` |
| Report generation | `team-report.schema.json` | `report.go:GenerateTeamReport()` |
| Tool mapping | Canonical → Local | `mapping.go:mapCanonicalTools()` |
| Model mapping | Canonical → Provider | `mapping.go:mapCanonicalModel()` |

## API Specification (Phase 4: Service Mode)

### REST Endpoints

| Method | Path | Description |
|--------|------|-------------|
| `POST` | `/workflows` | Submit workflow for execution |
| `GET` | `/workflows/:id` | Get workflow status and results |
| `GET` | `/workflows/:id/stream` | SSE stream of workflow progress |
| `DELETE` | `/workflows/:id` | Cancel running workflow |
| `POST` | `/agents/:name/invoke` | Invoke single agent |
| `GET` | `/agents` | List available agents |
| `GET` | `/health` | Health check |

### Workflow Submission Request

```json
{
  "spec_path": "specs/teams/prd-team.json",
  "session_id": "prd-001",
  "steps": ["discovery", "solution"],
  "context": {
    "project": "my-project"
  }
}
```

### SSE Stream Events

```
event: step_start
data: {"step_id": "problem-discovery", "agent": "problem-discovery"}

event: step_progress
data: {"step_id": "problem-discovery", "content": "Analyzing..."}

event: step_complete
data: {"step_id": "problem-discovery", "status": "GO", "duration": "45s"}

event: workflow_complete
data: {"status": "GO", "report": {...}}
```
