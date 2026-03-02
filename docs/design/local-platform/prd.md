# PRD: AgentKit Local Platform

**Status:** Draft
**Version:** 1.0.0
**Owner:** PlexusOne Team
**Last Updated:** 2025-01-20

## Problem Statement

### Primary Problem

Developers building multi-agent AI applications need a way to run orchestrated agent workflows locally that:

1. Work with any LLM provider (not locked to a single vendor)
2. Integrate with existing AI CLI tools (Claude Code, Kiro, Gemini CLI)
3. Support filesystem access for code generation and analysis tasks
4. Enable DAG-based workflow orchestration with complex dependencies
5. Persist state across agent invocations for multi-turn interactions

### Current Solutions and Gaps

| Solution | LLM Agnostic | DAG Orchestration | Filesystem Tools | CLI Integration | Memory/State |
|----------|--------------|-------------------|------------------|-----------------|--------------|
| Google ADK | No (Gemini-focused) | No (seq/parallel only) | No | No | Yes (session) |
| CrewAI | Yes | No | Limited | No | Yes |
| AutoGen | Yes | No | Limited | No | Yes |
| LangGraph | Yes | Yes | Limited | No | Yes |
| Claude Code | No (Anthropic only) | No | Yes | N/A | No |
| **AgentKit Local** | **Yes** | **Yes** | **Yes** | **Yes** | **Yes** |

### User Segments

1. **AI Application Developers** - Building production multi-agent systems
2. **CLI Tool Authors** - Creating AI-powered developer tools
3. **Workflow Automation Engineers** - Orchestrating complex AI pipelines
4. **Enterprise Teams** - Need vendor-agnostic, auditable agent systems

## Goals and Non-Goals

### Goals

1. **G1:** Provide a CLI tool (`agent-cli`) for running single agents or multi-agent workflows locally
2. **G2:** Provide a service mode (`agent-server`) for long-running agent orchestration with streaming
3. **G3:** Support arbitrary DAG-based workflow orchestration (not just sequential/parallel)
4. **G4:** Integrate with omnillm for multi-provider LLM support
5. **G5:** Enable persistent memory/state via pluggable backends (filesystem, Redis)
6. **G6:** Load agent and workflow specifications from multi-agent-spec format
7. **G7:** Expose agents via MCP for integration with AI CLI tools

### Execution Modes

AgentKit Local targets **two execution modes** with a shared core:

| Mode | Entry Point | Use Case | Priority |
|------|-------------|----------|----------|
| **CLI** | `agent-cli` | One-shot execution, CI/CD, scripting | P0 (Phase 1) |
| **Service** | `agent-server` | Long-running, streaming, warm connections | P1 (Phase 2) |

```
┌─────────────────────────────────────────────────────────────┐
│                    Shared Core                               │
│  ┌─────────┐ ┌─────────┐ ┌─────────┐ ┌─────────┐           │
│  │  DAG    │ │  State  │ │  Spec   │ │  LLM    │           │
│  │Executor │ │ Backend │ │ Loader  │ │ Client  │           │
│  └─────────┘ └─────────┘ └─────────┘ └─────────┘           │
├──────────────────────┬──────────────────────────────────────┤
│      CLI Mode        │           Service Mode               │
│  ┌────────────────┐  │  ┌────────────────────────────────┐ │
│  │  agent-cli     │  │  │  agent-server                  │ │
│  │  - run         │  │  │  - HTTP/gRPC API               │ │
│  │  - workflow    │  │  │  - Streaming responses         │ │
│  │  - list        │  │  │  - Session management          │ │
│  │  - validate    │  │  │  - Warm LLM connections        │ │
│  └────────────────┘  │  └────────────────────────────────┘ │
└──────────────────────┴──────────────────────────────────────┘
```

**Why both modes?**

| Capability | CLI | Service |
|------------|-----|---------|
| One-shot execution | ✅ | ✅ |
| CI/CD integration | ✅ | ❌ |
| Streaming responses | ❌ | ✅ |
| Warm LLM connections | ❌ | ✅ |
| Session persistence across calls | Via filesystem | In-memory + persistent |
| Human-in-the-loop intervention | Limited | ✅ |
| Resource efficiency (many workflows) | ❌ (cold start) | ✅ |

**Start with CLI** because:
- Simpler to implement and test
- Matches existing AI CLI tool patterns (Claude Code, Kiro)
- No daemon management complexity
- State persistence via filesystem is sufficient initially

**Add Service later** when needed for:
- Real-time streaming of agent reasoning
- High-throughput workflow execution
- Interactive applications requiring low latency

### Non-Goals

- N1: Replace distributed/serverless deployments (use AgentCore, Kubernetes for that)
- N2: Provide a GUI or web interface
- N3: Implement custom LLM inference (use omnillm)
- N4: Support non-Go agent implementations in this package

## Multi-Agent Spec Compliance

AgentKit Local **MUST** implement the [multi-agent-spec](https://github.com/plexusone/multi-agent-spec) schema for workflow definitions. This ensures portability across platforms (Claude Code, Kiro CLI, AWS AgentCore, Kubernetes).

### Required Schema Support

| Schema | Location | Purpose |
|--------|----------|---------|
| Agent Schema | `schema/agent/agent.schema.json` | Agent definitions with tools, model, instructions |
| Team Schema | `schema/orchestration/team.schema.json` | Team composition with DAG workflow |
| Deployment Schema | `schema/deployment/deployment.schema.json` | Runtime configuration for agentkit-local |

### Agent Definition Format (Markdown + YAML Frontmatter)

```yaml
---
name: problem-discovery
description: User-centric problem definition expert
model: sonnet
tools: [Read, Grep, Glob, WebSearch]
dependencies: []
instructions: |
  Focus on identifying REAL problems before solutions.
tasks:
  - id: problem-statement
    description: Clear problem statement defined
    type: pattern
    pattern: "problem_statement"
    required: true
---

## Role
User-centric problem definition expert focused on identifying real problems...

## Instructions
1. Define primary and secondary problems...
```

### Team Definition Format (JSON)

```json
{
  "name": "prd-team",
  "version": "1.0.0",
  "description": "PRD creation and validation team",
  "agents": ["problem-discovery", "user-research", "market-intel", "solution-ideation"],
  "orchestrator": "prd-lead",
  "workflow": {
    "type": "dag",
    "steps": [
      {
        "name": "problem-discovery",
        "agent": "problem-discovery",
        "depends_on": [],
        "outputs": [{"name": "problem_statement", "type": "object"}]
      },
      {
        "name": "user-research",
        "agent": "user-research",
        "depends_on": [],
        "outputs": [{"name": "personas", "type": "array"}]
      },
      {
        "name": "solution-ideation",
        "agent": "solution-ideation",
        "depends_on": ["problem-discovery", "user-research"],
        "inputs": [
          {"name": "problem", "from": "problem-discovery.problem_statement"},
          {"name": "users", "from": "user-research.personas"}
        ]
      }
    ]
  }
}
```

### Deployment Configuration Format (JSON)

```json
{
  "team": "prd-team",
  "targets": [
    {
      "name": "local-dev",
      "platform": "agentkit-local",
      "mode": "single-process",
      "priority": "p1",
      "runtime": {
        "defaults": {
          "timeout": "5m",
          "retry": {"max_attempts": 2, "backoff": "exponential"}
        },
        "steps": {
          "prd-lead": {"timeout": "10m"}
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
  ]
}
```

### Tool Mapping (Canonical → AgentKit)

| Canonical Tool | AgentKit Tool | Notes |
|----------------|---------------|-------|
| Read | read | File reading |
| Write | write | File writing |
| Glob | glob | Pattern matching |
| Grep | grep | Content search |
| Bash | shell | Command execution |
| Edit | write | File modification |
| WebSearch | shell | Via curl/external |
| WebFetch | shell | Via curl/external |
| Task | (orchestration) | Spawn sub-agents |

### Model Mapping (Canonical → Provider)

| Canonical | Anthropic | OpenAI | Gemini |
|-----------|-----------|--------|--------|
| haiku | claude-3-5-haiku-20241022 | gpt-4o-mini | gemini-2.0-flash |
| sonnet | claude-sonnet-4-20250514 | gpt-4o | gemini-2.5-pro |
| opus | claude-opus-4-20250514 | gpt-4.5 | gemini-2.5-pro |

### Report Output Format

AgentKit Local must produce reports conforming to `schema/report/`:

```json
{
  "project": "github.com/org/repo",
  "version": "1.0.0",
  "teams": [
    {
      "id": "problem-discovery",
      "name": "Problem Discovery",
      "agent_id": "problem-discovery",
      "model": "sonnet",
      "status": "GO",
      "tasks": [
        {"id": "problem-statement", "status": "GO", "detail": "Defined"}
      ]
    }
  ],
  "status": "GO",
  "generated_at": "2025-01-20T10:30:00Z"
}
```

**Status Values:** GO | WARN | NO-GO | SKIP (NASA Go/No-Go terminology)

## Requirements

### Functional Requirements

#### FR1: CLI Interface

| ID | Requirement | Priority | Acceptance Criteria |
|----|-------------|----------|---------------------|
| FR1.1 | Run a single agent with a task | P0 | `agent-cli run --agent <name> --task "..."` executes agent and returns result |
| FR1.2 | Run a workflow from spec file | P0 | `agent-cli workflow --spec <file>` executes DAG workflow |
| FR1.3 | List available agents | P1 | `agent-cli list` shows all configured agents |
| FR1.4 | Interactive mode | P2 | `agent-cli interactive` provides REPL for agent interaction |
| FR1.5 | Output format selection | P1 | `--output json|text|toon` controls output format |

#### FR2: DAG Orchestration

| ID | Requirement | Priority | Acceptance Criteria |
|----|-------------|----------|---------------------|
| FR2.1 | Load workflow from JSON/YAML | P0 | Parse workflow spec with steps and dependencies |
| FR2.2 | Topological sort execution | P0 | Execute steps respecting `depends_on` constraints |
| FR2.3 | Parallel execution of independent steps | P0 | Steps without dependencies run concurrently |
| FR2.4 | Context passing between steps | P0 | Step outputs available to dependent steps |
| FR2.5 | Conditional execution | P1 | Skip steps based on previous step outputs |
| FR2.6 | Loop/iteration support | P1 | Repeat step sequences until condition met |

#### FR3: LLM Integration

| ID | Requirement | Priority | Acceptance Criteria |
|----|-------------|----------|---------------------|
| FR3.1 | omnillm integration | P0 | Use omnillm.ChatClient for all LLM calls |
| FR3.2 | Provider selection via config | P0 | Support anthropic, openai, gemini, xai, ollama |
| FR3.3 | Per-agent model override | P1 | Agent config can specify different model |
| FR3.4 | Fallback providers | P1 | Automatic failover if primary provider fails |
| FR3.5 | Streaming responses | P2 | Stream LLM output for real-time feedback |

#### FR4: Memory and State

| ID | Requirement | Priority | Acceptance Criteria |
|----|-------------|----------|---------------------|
| FR4.1 | Session state persistence | P0 | State persists across CLI invocations within session |
| FR4.2 | Filesystem backend | P0 | Store state in `.agent-state/` directory |
| FR4.3 | Redis backend | P1 | Store state in Redis for distributed scenarios |
| FR4.4 | Agent output history | P0 | Previous agent outputs accessible to later agents |
| FR4.5 | Conversation memory | P1 | Multi-turn conversation history per agent |

#### FR5: Tool System

| ID | Requirement | Priority | Acceptance Criteria |
|----|-------------|----------|---------------------|
| FR5.1 | Filesystem tools | P0 | read, write, glob, grep tools (already implemented) |
| FR5.2 | Shell execution | P0 | shell tool for command execution (already implemented) |
| FR5.3 | Workspace sandboxing | P0 | Tools scoped to workspace directory (already implemented) |
| FR5.4 | Custom tool registration | P2 | API for registering additional tools |
| FR5.5 | Tool approval workflow | P2 | Require user approval for dangerous operations |

#### FR6: Specification Loading (multi-agent-spec Compliance)

| ID | Requirement | Priority | Acceptance Criteria |
|----|-------------|----------|---------------------|
| FR6.1 | Load agent specs from Markdown | P0 | Parse multi-agent-spec `agent.schema.json` format (YAML frontmatter + body) |
| FR6.2 | Load team specs from JSON | P0 | Parse multi-agent-spec `team.schema.json` with workflow DAG |
| FR6.3 | Load deployment specs from JSON | P0 | Parse multi-agent-spec `deployment.schema.json` for runtime config |
| FR6.4 | Validate specs against schemas | P0 | JSON Schema validation with clear error messages |
| FR6.5 | Map canonical tools to local tools | P0 | Transform Read/Write/Glob/Grep/Bash to local tool names |
| FR6.6 | Map canonical models to providers | P0 | Transform haiku/sonnet/opus to provider-specific model IDs |
| FR6.7 | Support typed ports for data flow | P1 | Parse inputs/outputs with `from` references for cross-step data |
| FR6.8 | Generate conformant reports | P0 | Output reports matching `team-report.schema.json` with GO/WARN/NO-GO status |

### Non-Functional Requirements

| ID | Requirement | Target | Measurement |
|----|-------------|--------|-------------|
| NFR1 | Single agent latency overhead | < 100ms | Time from CLI start to first LLM call |
| NFR2 | Parallel agent efficiency | > 90% | (parallel time) / (longest single agent time) |
| NFR3 | Memory usage | < 100MB | Peak RSS for 10-agent workflow |
| NFR4 | Error recovery | 100% | Graceful handling of LLM failures |

## User Journeys

### Journey 1: Run PRD Discovery Phase

```bash
# User has agent specs in specs/agents/
# User wants to run 3 discovery agents in parallel

$ agent-cli workflow --spec specs/teams/prd-team.json \
                     --steps discovery \
                     --session prd-001

[discovery] Starting parallel execution of 3 agents...
  [problem-discovery] Running...
  [user-research] Running...
  [market-intel] Running...
[discovery] All agents completed in 45s

Results written to .agent-state/prd-001/discovery.json
```

### Journey 2: Single Agent with Memory

```bash
# User wants to iterate with an agent across multiple turns

$ agent-cli run --agent problem-discovery \
                --session my-session \
                --task "Analyze the authentication problem"

[problem-discovery] Analyzing...
Problem Statement: Users are experiencing friction during login...

$ agent-cli run --agent problem-discovery \
                --session my-session \
                --task "Now consider mobile users specifically"

[problem-discovery] Building on previous analysis...
Mobile-Specific Issues: Touch targets are too small...
```

### Journey 3: Full Release Workflow

```bash
# User runs release validation workflow

$ agent-cli workflow --spec specs/teams/release-team.json \
                     --provider anthropic \
                     --session release-v1.2.0

[pm-validation] Checking version and scope...
[pm-validation] Recommended version: v1.2.0 (minor)

[qa-validation] Running build and tests...
[docs-validation] Checking documentation...
[security-validation] Scanning for vulnerabilities...

[release-validation] All checks passed. GO for release.

Summary:
  PM: GO
  QA: GO (tests: 142 passed, 0 failed)
  Docs: GO (changelog updated)
  Security: GO (0 vulnerabilities)

Release ready. Run with --execute to create tag.
```

## Comparison: AgentKit Local vs Google ADK

### Why Not Just Use Google ADK?

| Capability | Google ADK | AgentKit Local | Gap Impact |
|------------|------------|----------------|------------|
| **LLM Provider** | Gemini-focused | Any via omnillm | **High** - Vendor lock-in unacceptable for enterprise |
| **DAG Orchestration** | Sequential, Parallel, Loop only | Arbitrary DAG | **High** - Complex workflows need dependency graphs |
| **Filesystem Tools** | Not built-in | Built-in, sandboxed | **Medium** - Must add manually with ADK |
| **CLI Tool** | No CLI entry point | `agent-cli` binary | **High** - Need standalone execution |
| **Workflow Specs** | Code-defined only | JSON/YAML/Markdown specs | **Medium** - Specs enable non-code workflow definition |
| **Memory Backend** | In-memory, GORM | In-memory, File, Redis | **Low** - Similar capabilities |
| **MCP Support** | Yes | Yes | Parity |
| **A2A Protocol** | Yes | Via agentkit/a2a | Parity |

### What ADK Provides That We Leverage

AgentKit already uses these ADK capabilities:

- **Agent interface patterns** - `agent.Agent` interface design
- **Session management concepts** - State scoping patterns
- **A2A protocol** - Via `agentkit/a2a` package

### What We Build Beyond ADK

| Feature | Implementation |
|---------|----------------|
| DAG executor | Custom topological sort with parallel execution |
| Spec loader | Parse Markdown/JSON/YAML agent definitions |
| CLI tool | Standalone binary using platforms/local |
| omnillm integration | LLMClient implementation using omnillm.ChatClient |
| Filesystem state | JSON files in `.agent-state/` |
| Redis state | KVS backend integration |

## Success Metrics

| Metric | Target | Measurement Method |
|--------|--------|-------------------|
| Workflow execution success rate | > 99% | Automated test suite |
| Provider coverage | 5+ providers | Integration tests per provider |
| Spec format support | 3 formats | JSON, YAML, Markdown parsing tests |
| Documentation completeness | 100% | All public APIs documented |
| Example coverage | 3+ examples | Working examples for common use cases |

## Risks and Mitigations

| Risk | Likelihood | Impact | Mitigation |
|------|------------|--------|------------|
| omnillm API changes | Medium | High | Pin version, integration tests |
| Complex DAG edge cases | Medium | Medium | Comprehensive test suite, cycle detection |
| Tool security (shell execution) | High | High | Workspace sandboxing, command allowlists |
| Provider rate limits | High | Medium | Retry with backoff, fallback providers |

## Implementation Phases

### Phase 1: CLI Foundation (P0)

- [ ] `cmd/agent-cli/main.go` entry point
- [ ] omnillm-based LLMClient implementation
- [ ] Single agent execution via CLI
- [ ] Filesystem state backend
- [ ] multi-agent-spec agent loading (Markdown + YAML frontmatter)

### Phase 2: DAG Orchestration (P0)

- [ ] multi-agent-spec team/workflow loading (JSON)
- [ ] DAG executor with topological sort
- [ ] Parallel execution of independent steps
- [ ] Typed port data flow between steps
- [ ] multi-agent-spec report generation (GO/WARN/NO-GO)

### Phase 3: Enhanced CLI Features (P1)

- [ ] Redis state backend
- [ ] multi-agent-spec deployment config loading
- [ ] Per-agent model override via canonical mapping
- [ ] Conditional execution
- [ ] `agent-cli validate` for spec validation

### Phase 4: Service Mode (P1)

- [ ] `cmd/agent-server/main.go` entry point
- [ ] HTTP API for workflow submission
- [ ] Streaming responses via SSE/WebSocket
- [ ] Session management with warm connections
- [ ] Health check and graceful shutdown

### Phase 5: Polish (P2)

- [ ] Interactive REPL mode (`agent-cli interactive`)
- [ ] Tool approval workflow
- [ ] Custom tool registration
- [ ] MCP server integration for AI CLI tools

## Appendix: Existing Implementation

The `platforms/local/` package already provides:

| File | Status | Functionality |
|------|--------|---------------|
| `config.go` | Complete | YAML/JSON config loading, validation |
| `tools.go` | Complete | Read, Write, Glob, Grep, Shell tools |
| `agent.go` | Complete | EmbeddedAgent with tool-calling loop |
| `runner.go` | Partial | Sequential and parallel execution (no DAG) |

**Estimated new code:** ~600 lines Go

- CLI entry point: ~150 lines
- DAG executor: ~200 lines
- omnillm LLMClient: ~100 lines
- State backends: ~150 lines
