// Package main provides the CLI entry point for AgentKit local mode.
// This CLI enables running multi-agent workflows from the command line.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/plexusone/agentkit/platforms/local"
)

const (
	programName    = "agent-cli"
	programVersion = "0.1.0"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	// Define flags
	var (
		configPath   = flag.String("config", "", "Path to config file (JSON or YAML)")
		specDir      = flag.String("spec", "", "Path to multi-agent-spec directory")
		workspace    = flag.String("workspace", ".", "Workspace directory")
		provider     = flag.String("provider", "anthropic", "LLM provider (anthropic, openai, gemini, xai, ollama)")
		model        = flag.String("model", "sonnet", "Model name or canonical tier (haiku, sonnet, opus)")
		apiKey       = flag.String("api-key", "", "API key (or use env var)")
		agent        = flag.String("agent", "", "Run a specific agent by name")
		input        = flag.String("input", "", "Input text for the agent/workflow")
		inputFile    = flag.String("input-file", "", "Read input from file")
		outputFile   = flag.String("output", "", "Write output to file")
		outputJSON   = flag.Bool("json", false, "Output results as JSON")
		resume       = flag.String("resume", "", "Resume a previous run by ID")
		timeout      = flag.Duration("timeout", 10*time.Minute, "Execution timeout")
		stateDir     = flag.String("state-dir", ".agent-state", "Directory for state persistence")
		verbose      = flag.Bool("verbose", false, "Enable verbose output")
		showVersion  = flag.Bool("version", false, "Show version and exit")
		listAgents   = flag.Bool("list-agents", false, "List available agents and exit")
		checkPrereqs = flag.Bool("check-prereqs", false, "Check CLI prerequisites and exit")
		skipPrereqs  = flag.Bool("skip-prereq-check", false, "Skip prerequisite validation")
	)

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s [options] [input]\n\n", programName)
		fmt.Fprintf(os.Stderr, "AgentKit Local Mode CLI - Run multi-agent workflows from the command line.\n\n")
		fmt.Fprintf(os.Stderr, "Options:\n")
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nExamples:\n")
		fmt.Fprintf(os.Stderr, "  %s -config config.json -input \"Analyze this codebase\"\n", programName)
		fmt.Fprintf(os.Stderr, "  %s -spec ./my-team -agent researcher -input \"Find info about Go\"\n", programName)
		fmt.Fprintf(os.Stderr, "  %s -agent researcher -input-file prompt.txt -output result.md\n", programName)
		fmt.Fprintf(os.Stderr, "  %s -resume run-123456789 -spec ./my-team\n", programName)
	}

	flag.Parse()

	// Handle version flag
	if *showVersion {
		fmt.Printf("%s version %s\n", programName, programVersion)
		return nil
	}

	// Get input from args if not specified
	inputText := *input
	if inputText == "" && *inputFile != "" {
		data, err := os.ReadFile(*inputFile)
		if err != nil {
			return fmt.Errorf("failed to read input file: %w", err)
		}
		inputText = string(data)
	}
	if inputText == "" && flag.NArg() > 0 {
		inputText = strings.Join(flag.Args(), " ")
	}

	// Load configuration
	var cfg *local.Config
	var team *local.TeamSpec
	var err error

	llmCfg := local.LLMConfig{
		Provider: *provider,
		Model:    *model,
		APIKey:   getAPIKey(*apiKey, *provider),
	}

	if *configPath != "" {
		// Load from config file
		cfg, err = local.LoadConfig(*configPath)
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}
	} else if *specDir != "" {
		// Load from multi-agent-spec directory
		cfg, team, err = local.LoadFromSpec(*specDir, llmCfg)
		if err != nil {
			return fmt.Errorf("failed to load spec: %w", err)
		}
	} else {
		return fmt.Errorf("either -config or -spec must be specified")
	}

	// Override workspace if specified
	if *workspace != "." {
		absWorkspace, err := filepath.Abs(*workspace)
		if err != nil {
			return fmt.Errorf("invalid workspace: %w", err)
		}
		cfg.Workspace = absWorkspace
	}

	// Handle list-agents flag
	if *listAgents {
		fmt.Printf("Available agents:\n")
		for _, a := range cfg.Agents {
			fmt.Printf("  - %s: %s\n", a.Name, a.Description)
		}
		return nil
	}

	// Load agent specs for prerequisite checking (if using spec directory)
	var agentSpecs map[string]*local.AgentSpec
	if *specDir != "" {
		loader := local.NewSpecLoader(*specDir)
		_, agentSpecs, _ = loader.LoadTeam("team.json")
	}

	// Check prerequisites
	if team != nil && agentSpecs != nil {
		prereqResult := local.ValidateTeamPrerequisites(team, agentSpecs)

		if *checkPrereqs {
			// Just check and report
			if prereqResult.AllFound {
				fmt.Println("All prerequisites satisfied.")
				for agentName, result := range prereqResult.AgentResults {
					if len(result.Checks) > 0 {
						fmt.Printf("\n%s:\n", agentName)
						for _, check := range result.Checks {
							status := "OK"
							if !check.Found {
								status = "MISSING"
							}
							fmt.Printf("  [%s] %s", status, check.Name)
							if check.Version != "" {
								fmt.Printf(" (%s)", check.Version)
							}
							fmt.Println()
						}
					}
				}
			} else {
				fmt.Println(prereqResult.PrintMissingWithHints())
			}
			return nil
		}

		// Validate prerequisites before execution (unless skipped)
		if !*skipPrereqs && !prereqResult.AllFound {
			fmt.Fprintln(os.Stderr, "Prerequisite check failed:")
			fmt.Fprintln(os.Stderr, prereqResult.PrintMissingWithHints())
			fmt.Fprintln(os.Stderr, "Use --skip-prereq-check to bypass this check.")
			return fmt.Errorf("missing prerequisites")
		}

		if *verbose && prereqResult.AllFound {
			fmt.Fprintln(os.Stderr, "All prerequisites satisfied.")
		}
	}

	// Require input for execution
	if inputText == "" && *resume == "" {
		return fmt.Errorf("input is required (use -input, -input-file, or provide as argument)")
	}

	// Set up context with timeout and cancellation
	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()

	// Handle signals for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigChan
		if *verbose {
			fmt.Fprintln(os.Stderr, "\nReceived interrupt, shutting down...")
		}
		cancel()
	}()

	// Create toolset
	toolSet := local.NewToolSet(cfg.Workspace)

	// Create LLM client
	llmClient, err := local.NewOmniLLMClientFromConfig(cfg.LLM)
	if err != nil {
		return fmt.Errorf("failed to create LLM client: %w", err)
	}
	defer llmClient.Close()

	// Create state backend
	stateBackend, err := local.NewFileStateBackend(filepath.Join(cfg.Workspace, *stateDir))
	if err != nil {
		return fmt.Errorf("failed to create state backend: %w", err)
	}

	// Create agents
	agents := make(map[string]*local.EmbeddedAgent)
	for _, agentCfg := range cfg.Agents {
		agent, err := local.NewEmbeddedAgent(agentCfg, toolSet, llmClient)
		if err != nil {
			return fmt.Errorf("failed to create agent %s: %w", agentCfg.Name, err)
		}
		agents[agentCfg.Name] = agent
	}

	// Execute based on mode
	var result interface{}

	if *agent != "" {
		// Single agent execution
		a, ok := agents[*agent]
		if !ok {
			return fmt.Errorf("agent %s not found", *agent)
		}

		if *verbose {
			fmt.Fprintf(os.Stderr, "Running agent: %s\n", *agent)
		}

		agentResult, err := a.Invoke(ctx, inputText)
		if err != nil {
			return fmt.Errorf("agent execution failed: %w", err)
		}
		result = agentResult
	} else if team != nil {
		// Workflow execution
		executor := local.NewDAGExecutor(agents, stateBackend)

		var workflowResult *local.WorkflowResult

		if *resume != "" {
			// Resume previous execution
			if *verbose {
				fmt.Fprintf(os.Stderr, "Resuming workflow run: %s\n", *resume)
			}
			workflowResult, err = executor.ResumeWorkflow(ctx, *resume, team)
		} else {
			// New execution
			if *verbose {
				fmt.Fprintf(os.Stderr, "Running workflow: %s\n", team.Name)
			}
			workflowResult, err = executor.ExecuteWorkflow(ctx, team, inputText)
		}

		if err != nil {
			return fmt.Errorf("workflow execution failed: %w", err)
		}

		// Build report
		builder := local.NewReportBuilder("", cfg.LLM.Model)
		builder.SetPhase("EXECUTION")
		builder.SetGeneratedBy(programName)
		report := builder.BuildFromWorkflowResult(workflowResult, team)

		if *verbose {
			fmt.Fprintln(os.Stderr, report.PrintSummary())
		}

		result = workflowResult
	} else {
		return fmt.Errorf("no workflow or agent specified")
	}

	// Output result
	output := ""
	if *outputJSON {
		data, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal result: %w", err)
		}
		output = string(data)
	} else {
		// Extract text output
		switch r := result.(type) {
		case *local.AgentResult:
			output = r.Output
		case *local.WorkflowResult:
			output = r.FinalOutput
		default:
			data, _ := json.MarshalIndent(result, "", "  ")
			output = string(data)
		}
	}

	if *outputFile != "" {
		if err := os.WriteFile(*outputFile, []byte(output), 0600); err != nil {
			return fmt.Errorf("failed to write output file: %w", err)
		}
		if *verbose {
			fmt.Fprintf(os.Stderr, "Output written to: %s\n", *outputFile)
		}
	} else {
		fmt.Println(output)
	}

	return nil
}

// getAPIKey returns the API key from flag, environment variable, or default.
func getAPIKey(flagValue, provider string) string {
	if flagValue != "" {
		return flagValue
	}

	// Check environment variables
	envVars := map[string]string{
		"anthropic": "ANTHROPIC_API_KEY",
		"openai":    "OPENAI_API_KEY",
		"gemini":    "GEMINI_API_KEY",
		"xai":       "XAI_API_KEY",
	}

	if envVar, ok := envVars[provider]; ok {
		if value := os.Getenv(envVar); value != "" {
			return value
		}
	}

	return ""
}
