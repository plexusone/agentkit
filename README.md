# AgentKit

[![Go CI][go-ci-svg]][go-ci-url]
[![Go Lint][go-lint-svg]][go-lint-url]
[![Go SAST][go-sast-svg]][go-sast-url]
[![Go Report Card][goreport-svg]][goreport-url]
[![Docs][docs-godoc-svg]][docs-godoc-url]
[![Docs][docs-mkdoc-svg]][docs-mkdoc-url]
[![Visualization][viz-svg]][viz-url]
[![License][license-svg]][license-url]

 [go-ci-svg]: https://github.com/plexusone/agentkit/actions/workflows/go-ci.yaml/badge.svg?branch=main
 [go-ci-url]: https://github.com/plexusone/agentkit/actions/workflows/go-ci.yaml
 [go-lint-svg]: https://github.com/plexusone/agentkit/actions/workflows/go-lint.yaml/badge.svg?branch=main
 [go-lint-url]: https://github.com/plexusone/agentkit/actions/workflows/go-lint.yaml
 [go-sast-svg]: https://github.com/plexusone/agentkit/actions/workflows/go-sast-codeql.yaml/badge.svg?branch=main
 [go-sast-url]: https://github.com/plexusone/agentkit/actions/workflows/go-sast-codeql.yaml
 [goreport-svg]: https://goreportcard.com/badge/github.com/plexusone/agentkit
 [goreport-url]: https://goreportcard.com/report/github.com/plexusone/agentkit
 [docs-godoc-svg]: https://pkg.go.dev/badge/github.com/plexusone/agentkit
 [docs-godoc-url]: https://pkg.go.dev/github.com/plexusone/agentkit
 [docs-mkdoc-svg]: https://img.shields.io/badge/Go-dev%20guide-blue.svg
 [docs-mkdoc-url]: https://plexusone.dev/agentkit
 [viz-svg]: https://img.shields.io/badge/Go-visualizaton-blue.svg
 [viz-url]: https://mango-dune-07a8b7110.1.azurestaticapps.net/?repo=plexusone%2Fagentkit
 [loc-svg]: https://tokei.rs/b1/github/plexusone/agentkit
 [repo-url]: https://github.com/plexusone/agentkit
 [license-svg]: https://img.shields.io/badge/license-MIT-blue.svg
 [license-url]: https://github.com/plexusone/agentkit/blob/main/LICENSE

A Go library for building AI agent applications. Provides server factories, LLM abstractions, workflow orchestration, and multi-runtime deployment support.

## Features

- 🏭 **Server Factories** - A2A and HTTP servers in 5 lines (saves ~475 lines per project)
- 🧠 **Multi-Provider LLM** - Gemini, Claude, OpenAI, xAI, Ollama via OmniLLM
- 🔀 **Workflow Orchestration** - Type-safe graph-based execution with Eino
- ☁️ **Multi-Runtime Deployment** - Kubernetes (Helm) or AWS AgentCore
- 🔒 **VaultGuard Integration** - Security-gated credential access

## Architecture

```
agentkit/
├── # Core (platform-agnostic)
├── a2a/             # A2A protocol server factory
├── agent/           # Base agent framework
├── config/          # Configuration management
├── http/            # HTTP client utilities
├── httpserver/      # HTTP server factory
├── llm/             # Multi-provider LLM abstraction
├── orchestration/   # Eino workflow orchestration
│
├── # Platform-specific
└── platforms/
    ├── agentcore/   # AWS Bedrock AgentCore runtime
    └── kubernetes/  # Kubernetes + Helm deployment
```

## Installation

```bash
go get github.com/plexusone/agentkit
```

## Quick Start

### Complete Agent with HTTP + A2A Servers

```go
package main

import (
    "context"

    "github.com/plexusone/agentkit/a2a"
    "github.com/plexusone/agentkit/agent"
    "github.com/plexusone/agentkit/config"
    "github.com/plexusone/agentkit/httpserver"
)

func main() {
    ctx := context.Background()
    cfg := config.LoadConfig()

    // Create agent
    ba, _ := agent.NewBaseAgent(cfg, "research-agent", 30)
    researchAgent := NewResearchAgent(ba, cfg)

    // HTTP server - 5 lines
    httpServer, _ := httpserver.NewBuilder("research-agent", 8001).
        WithHandlerFunc("/research", researchAgent.HandleResearch).
        WithDualModeLog().
        Build()

    // A2A server - 5 lines
    a2aServer, _ := a2a.NewServer(a2a.Config{
        Agent:       researchAgent.ADKAgent(),
        Port:        "9001",
        Description: "Research agent for web search",
    })

    // Start servers
    a2aServer.StartAsync(ctx)
    httpServer.Start()
}
```

**This replaces ~100 lines of boilerplate with ~15 lines.**

### A2A Server Factory

```go
import "github.com/plexusone/agentkit/a2a"

// Create and start A2A server
server, _ := a2a.NewServer(a2a.Config{
    Agent:       myAgent,           // Google ADK agent
    Port:        "9001",            // Empty = random port
    Description: "My agent",
})

server.Start(ctx)  // Blocking
// or
server.StartAsync(ctx)  // Non-blocking

// Useful methods
server.URL()          // "http://localhost:9001"
server.AgentCardURL() // "http://localhost:9001/.well-known/agent.json"
server.InvokeURL()    // "http://localhost:9001/invoke"
server.Stop(ctx)      // Graceful shutdown
```

### HTTP Server Factory

```go
import "github.com/plexusone/agentkit/httpserver"

// Config-based
server, _ := httpserver.New(httpserver.Config{
    Name: "my-agent",
    Port: 8001,
    HandlerFuncs: map[string]http.HandlerFunc{
        "/process": agent.HandleProcess,
    },
})

// Builder pattern (fluent API)
server, _ := httpserver.NewBuilder("my-agent", 8001).
    WithHandlerFunc("/research", agent.HandleResearch).
    WithHandlerFunc("/synthesize", agent.HandleSynthesize).
    WithHandler("/orchestrate", orchestration.NewHTTPHandler(exec)).
    WithTimeouts(30*time.Second, 120*time.Second, 60*time.Second).
    WithDualModeLog().
    Build()

server.Start()
```

### Basic Agent

```go
import (
    "github.com/plexusone/agentkit/agent"
    "github.com/plexusone/agentkit/config"
)

cfg := config.LoadConfig()

ba, err := agent.NewBaseAgent(cfg, "my-agent", 30)
if err != nil {
    log.Fatal(err)
}
defer ba.Close()

// Utility methods
content, err := ba.FetchURL(ctx, url, maxSizeMB)
ba.LogInfo("message %s", arg)
ba.LogError("error %s", arg)
```

### Secure Agent with VaultGuard

```go
ba, secCfg, err := agent.NewBaseAgentSecure(ctx, "secure-agent", 30,
    config.WithPolicy(nil), // Use default policy
)
if err != nil {
    log.Fatalf("Security check failed: %v", err)
}
defer ba.Close()
defer secCfg.Close()

log.Printf("Security score: %d", secCfg.SecurityResult().Score)
```

### Workflow Orchestration with Eino

```go
import (
    "github.com/cloudwego/eino/compose"
    "github.com/plexusone/agentkit/orchestration"
)

// Build workflow graph
builder := orchestration.NewGraphBuilder[*Input, *Output]("my-workflow")
graph := builder.Graph()

// Add nodes using Eino's InvokableLambda
processLambda := compose.InvokableLambda(processFunc)
graph.AddLambdaNode("process", processLambda)

formatLambda := compose.InvokableLambda(formatFunc)
graph.AddLambdaNode("format", formatLambda)

// Connect nodes
builder.AddStartEdge("process")
builder.AddEdge("process", "format")
builder.AddEndEdge("format")

// Execute
finalGraph := builder.Build()
executor := orchestration.NewExecutor(finalGraph, "my-workflow")
result, err := executor.Execute(ctx, input)

// Expose as HTTP handler
handler := orchestration.NewHTTPHandler(executor)
http.Handle("/execute", handler)
```

## Multi-Runtime Deployment

AgentKit supports two deployment runtimes:

| Aspect | Kubernetes | AWS AgentCore |
|--------|------------|---------------|
| Distributions | EKS, GKE, AKS, Minikube, kind | AWS only |
| Config tool | Helm | CDK / Terraform |
| Scaling | HPA | Automatic |
| Isolation | Containers | Firecracker microVMs |
| Pricing | Always-on | Pay-per-use |

### Kubernetes Deployment

```go
import "github.com/plexusone/agentkit/platforms/kubernetes"

// Load and validate Helm values
values, errs := kubernetes.LoadAndValidate("values.yaml")

// Merge base and overlay values
values, err := kubernetes.LoadAndMerge("values.yaml", "values-prod.yaml")
```

Example `values.yaml`:

```yaml
global:
  image:
    registry: ghcr.io/myorg
    pullPolicy: IfNotPresent
    tag: "latest"

namespace:
  create: true
  name: my-agents

llm:
  provider: gemini
  geminiModel: "gemini-2.0-flash-exp"

agents:
  research:
    enabled: true
    replicaCount: 1
    image:
      repository: my-research-agent
    service:
      type: ClusterIP
      port: 8001
      a2aPort: 9001
    resources:
      requests:
        cpu: 100m
        memory: 128Mi

vaultguard:
  enabled: true
  minSecurityScore: 50
```

### AWS AgentCore Deployment

```go
import "github.com/plexusone/agentkit/platforms/agentcore"

// Simple setup
server := agentcore.NewBuilder().
    WithPort(8080).
    WithAgent(researchAgent).
    WithAgent(synthesisAgent).
    WithDefaultAgent("research").
    MustBuild(ctx)

server.Start()
```

Wrap Eino executors for AgentCore:

```go
// Build Eino workflow
graph := buildOrchestrationGraph()
executor := orchestration.NewExecutor(graph, "stats-workflow")

// Wrap for AgentCore
agent := agentcore.WrapExecutor("stats", executor)

// Or with custom I/O transformation
agent := agentcore.WrapExecutorWithPrompt("stats", executor,
    func(prompt string) StatsReq { return StatsReq{Topic: prompt} },
    func(out StatsResp) string { return out.Summary },
)
```

### Same Code, Different Runtimes

```go
// Agent implementation - runtime agnostic
executor := orchestration.NewExecutor(graph, "stats")

// Runtime 1: Kubernetes
httpServer, _ := httpserver.NewBuilder("stats", 8001).
    WithHandler("/stats", orchestration.NewHTTPHandler(executor)).
    Build()

// Runtime 2: AWS AgentCore
acServer := agentcore.NewBuilder().
    WithAgent(agentcore.WrapExecutor("stats", executor)).
    MustBuild(ctx)
```

### Local Development

AgentCore code runs locally without AWS - same binary, different infrastructure:

```bash
go run main.go
curl localhost:8080/ping
curl -X POST localhost:8080/invocations -d '{"prompt":"test"}'
```

| Aspect | Local | AWS AgentCore |
|--------|-------|---------------|
| Process | Go binary | Firecracker microVM |
| Sessions | In-memory | Isolated per microVM |
| Scaling | Manual | Automatic |

No code changes needed between local development and production.

## Packages

### `a2a`

A2A (Agent-to-Agent) protocol server factory.

```go
server, _ := a2a.NewServer(a2a.Config{
    Agent:             myAgent,
    Port:              "9001",
    Description:       "My agent",
    InvokePath:        "/invoke",        // Default: /invoke
    ReadHeaderTimeout: 10 * time.Second,
    SessionService:    customService,    // Default: in-memory
})
```

### `httpserver`

HTTP server factory with builder pattern.

```go
server, _ := httpserver.NewBuilder("name", 8001).
    WithHandlerFunc("/path", handlerFunc).
    WithHandler("/path2", handler).
    WithTimeouts(read, write, idle).
    WithDualModeLog().
    Build()
```

### `agent`

Base agent implementation with LLM integration.

```go
ba, err := agent.NewBaseAgent(cfg, "name", timeoutSec)
ba, secCfg, err := agent.NewBaseAgentSecure(ctx, "name", timeout, opts...)
```

### `config`

Configuration management with VaultGuard integration.

```go
cfg := config.LoadConfig()
secCfg, err := config.LoadSecureConfig(ctx, config.WithDevPolicy())
apiKey, err := secCfg.GetCredential(ctx, "API_KEY")
```

### `llm`

LLM model factory and adapters.

```go
factory := llm.NewModelFactory(cfg)
model, err := factory.CreateModel(ctx)
```

### `orchestration`

Eino-based workflow orchestration.

```go
builder := orchestration.NewGraphBuilder[Input, Output]("name")
executor := orchestration.NewExecutor(graph, "name")
handler := orchestration.NewHTTPHandler(executor)
```

### `http`

HTTP client utilities for inter-agent communication.

```go
err := http.PostJSON(ctx, client, url, request, &response)
err := http.GetJSON(ctx, client, url, &response)
err := http.HealthCheck(ctx, client, baseURL)
```

### `platforms/kubernetes`

Helm chart value structs and validation for Kubernetes deployments.

```go
values, errs := kubernetes.LoadAndValidate("values.yaml")
values, err := kubernetes.LoadAndMerge("values.yaml", "values-prod.yaml")
```

### `platforms/agentcore`

AWS Bedrock AgentCore runtime support.

```go
server := agentcore.NewBuilder().
    WithAgent(agent).
    MustBuild(ctx)

// Wrap Eino executors
agent := agentcore.WrapExecutor("name", executor)
```

## Configuration

AgentKit loads configuration from environment variables:

| Variable | Description | Default |
|----------|-------------|---------|
| `LLM_PROVIDER` | LLM provider (gemini, claude, openai, xai, ollama) | gemini |
| `LLM_MODEL` | Model name | Provider default |
| `GEMINI_API_KEY` | Gemini API key | - |
| `CLAUDE_API_KEY` | Claude/Anthropic API key | - |
| `OPENAI_API_KEY` | OpenAI API key | - |
| `XAI_API_KEY` | xAI API key | - |
| `OLLAMA_URL` | Ollama server URL | http://localhost:11434 |
| `OBSERVABILITY_ENABLED` | Enable LLM observability | false |
| `OBSERVABILITY_PROVIDER` | Provider (opik, langfuse, phoenix) | opik |

## Benefits

AgentKit eliminates ~1,500 lines of boilerplate per project:

| Component | Lines Saved |
|-----------|-------------|
| A2A server factory | ~350 lines |
| HTTP server factory | ~125 lines |
| Shared pkg/ code | ~930 lines |

See [BENEFITS.md](BENEFITS.md) for detailed analysis.

## Companion Modules

AgentKit has companion modules for Infrastructure-as-Code (IaC) deployment:

| Module | Purpose | Dependencies |
|--------|---------|--------------|
| [agentkit-aws-cdk](https://github.com/plexusone/agentkit-aws-cdk) | AWS CDK constructs for AgentCore | 21 |
| [agentkit-aws-pulumi](https://github.com/plexusone/agentkit-aws-pulumi) | Pulumi components for AgentCore | 340 |

All modules share the same YAML/JSON configuration schema from `platforms/agentcore/iac/`.

For pure CloudFormation (no CDK/Pulumi runtime), use the built-in generator:

```go
import "github.com/plexusone/agentkit/platforms/agentcore/iac"

config, _ := iac.LoadStackConfigFromFile("config.yaml")
iac.GenerateCloudFormationFile(config, "template.yaml")
```

See [ROADMAP.md](ROADMAP.md) for planned modules including Terraform support.

## Dependencies

- [OmniLLM](https://github.com/plexusone/omnillm) - Multi-provider LLM abstraction
- [VaultGuard](https://github.com/plexusone/vaultguard) - Security-gated credentials
- [Eino](https://github.com/cloudwego/eino) - Graph-based orchestration
- [Google ADK](https://google.golang.org/adk) - Agent Development Kit
- [a2a-go](https://github.com/a2aserver/a2a-go) - A2A protocol implementation

## License

MIT License
