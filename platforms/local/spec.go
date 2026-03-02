// Package local provides an embedded local mode for running agents in-process.
package local

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// TeamSpec represents a multi-agent-spec team definition.
// See: https://github.com/plexusone/multi-agent-spec/blob/main/schema/orchestration/team.schema.json
type TeamSpec struct {
	// Name is the team identifier (e.g., "stats-agent-team").
	Name string `json:"name"`

	// Version is the semantic version of the team definition.
	Version string `json:"version"`

	// Description is a brief description of the team's purpose.
	Description string `json:"description,omitempty"`

	// Agents lists agent names in the team (references agent definitions).
	Agents []string `json:"agents"`

	// Orchestrator is the name of the orchestrator agent (must be in agents list).
	Orchestrator string `json:"orchestrator,omitempty"`

	// Workflow defines agent coordination patterns.
	Workflow *WorkflowSpec `json:"workflow,omitempty"`

	// Context is shared context for all agents.
	Context string `json:"context,omitempty"`
}

// WorkflowSpec defines the workflow execution pattern.
type WorkflowSpec struct {
	// Type is the execution pattern: "sequential", "parallel", "dag", "orchestrated".
	Type string `json:"type"`

	// Steps are the ordered steps in the workflow.
	Steps []StepSpec `json:"steps,omitempty"`
}

// StepSpec represents a single step in the workflow.
type StepSpec struct {
	// Name is the step identifier (e.g., "pm-validation").
	Name string `json:"name"`

	// Agent is the agent to execute this step.
	Agent string `json:"agent"`

	// DependsOn lists steps that must complete before this step.
	DependsOn []string `json:"depends_on,omitempty"`

	// Inputs are data inputs consumed by this step.
	Inputs []PortSpec `json:"inputs,omitempty"`

	// Outputs are data outputs produced by this step.
	Outputs []PortSpec `json:"outputs,omitempty"`
}

// PortSpec represents a data port (input or output).
type PortSpec struct {
	// Name is the port identifier.
	Name string `json:"name"`

	// Type is the data type: "string", "number", "boolean", "object", "array", "file".
	Type string `json:"type,omitempty"`

	// Description is a human-readable description.
	Description string `json:"description,omitempty"`

	// Required indicates if this input is required (inputs only).
	Required *bool `json:"required,omitempty"`

	// From is the source reference as "step_name.output_name" (inputs only).
	From string `json:"from,omitempty"`

	// Default is the default value if not provided (inputs only).
	Default any `json:"default,omitempty"`
}

// AgentSpec represents a multi-agent-spec agent definition.
// See: https://github.com/plexusone/multi-agent-spec/blob/main/schema/agent/agent.schema.json
type AgentSpec struct {
	// Name is the unique identifier for the agent.
	Name string `json:"name"`

	// Description explains the agent's purpose and capabilities.
	Description string `json:"description"`

	// Model is the model capability tier: "haiku", "sonnet", "opus".
	Model string `json:"model,omitempty"`

	// Tools lists the canonical tools the agent can use.
	Tools []string `json:"tools,omitempty"`

	// Skills lists the skills the agent can invoke.
	Skills []string `json:"skills,omitempty"`

	// Dependencies lists other agents this agent depends on.
	Dependencies []string `json:"dependencies,omitempty"`

	// Requires lists external tools or binaries required.
	Requires []string `json:"requires,omitempty"`

	// Credentials defines secrets/credentials the agent needs.
	// Values can be omnivault URIs (env://VAR, op://vault/item, aws-sm://secret).
	Credentials map[string]CredentialSpec `json:"credentials,omitempty"`

	// Instructions is the system prompt for the agent.
	Instructions string `json:"instructions,omitempty"`

	// Tasks defines tasks this agent can perform.
	Tasks []TaskSpec `json:"tasks,omitempty"`
}

// CredentialSpec defines a credential requirement.
// Can be unmarshaled from either a string (URI) or an object.
type CredentialSpec struct {
	// Description explains what this credential is for.
	Description string `json:"description,omitempty"`

	// Required indicates if this credential must be present.
	Required bool `json:"required,omitempty"`

	// Source is the omnivault URI (env://VAR, op://vault/item, aws-sm://secret).
	Source string `json:"source"`
}

// UnmarshalJSON allows CredentialSpec to be either a string or object.
func (c *CredentialSpec) UnmarshalJSON(data []byte) error {
	// Try as string first
	var uri string
	if err := json.Unmarshal(data, &uri); err == nil {
		c.Source = uri
		c.Required = true // Default to required when using short form
		return nil
	}

	// Try as object
	type credentialSpecAlias CredentialSpec
	var obj credentialSpecAlias
	if err := json.Unmarshal(data, &obj); err != nil {
		return err
	}
	*c = CredentialSpec(obj)
	return nil
}

// TaskSpec represents a task an agent can perform.
type TaskSpec struct {
	// ID is the unique task identifier.
	ID string `json:"id"`

	// Description explains what this task validates or accomplishes.
	Description string `json:"description"`

	// Type is how the task is executed: "command", "pattern", "file", "manual".
	Type string `json:"type,omitempty"`

	// Command is the shell command to execute (for type: command).
	Command string `json:"command,omitempty"`

	// Pattern is the regex pattern to search for (for type: pattern).
	Pattern string `json:"pattern,omitempty"`

	// File is the file path to check (for type: file).
	File string `json:"file,omitempty"`

	// Files is the glob pattern for files to check (for type: pattern).
	Files string `json:"files,omitempty"`

	// Required indicates if task failure causes agent to report NO-GO.
	Required *bool `json:"required,omitempty"`

	// ExpectedOutput describes what constitutes success.
	ExpectedOutput string `json:"expected_output,omitempty"`

	// HumanInLoop describes when to prompt for human intervention.
	HumanInLoop string `json:"human_in_loop,omitempty"`
}

// LoadTeamSpec loads a team specification from a JSON file.
func LoadTeamSpec(path string) (*TeamSpec, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read team spec: %w", err)
	}

	var spec TeamSpec
	if err := json.Unmarshal(data, &spec); err != nil {
		return nil, fmt.Errorf("failed to parse team spec: %w", err)
	}

	return &spec, nil
}

// LoadAgentSpec loads an agent specification from a JSON or Markdown file.
// Markdown files must have YAML frontmatter with agent metadata.
func LoadAgentSpec(path string) (*AgentSpec, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read agent spec: %w", err)
	}

	// Check if this is a markdown file with frontmatter
	if strings.HasSuffix(path, ".md") {
		return parseMarkdownAgent(data)
	}

	// Parse as JSON
	var spec AgentSpec
	if err := json.Unmarshal(data, &spec); err != nil {
		return nil, fmt.Errorf("failed to parse agent spec: %w", err)
	}

	return &spec, nil
}

// parseMarkdownAgent parses an agent spec from markdown with YAML frontmatter.
// Format:
//
//	---
//	name: agent-name
//	description: Agent description
//	model: sonnet
//	tools: [Read, Write]
//	---
//	# Instructions in markdown
func parseMarkdownAgent(data []byte) (*AgentSpec, error) {
	content := string(data)

	// Check for frontmatter delimiter
	if !strings.HasPrefix(content, "---") {
		return nil, fmt.Errorf("markdown agent file must start with YAML frontmatter (---)")
	}

	// Find end of frontmatter
	endIdx := strings.Index(content[3:], "---")
	if endIdx == -1 {
		return nil, fmt.Errorf("missing closing frontmatter delimiter (---)")
	}
	endIdx += 3 // Adjust for the initial offset

	frontmatter := content[3:endIdx]
	instructions := strings.TrimSpace(content[endIdx+3:])

	// Parse YAML frontmatter manually (simple key: value parsing)
	spec := &AgentSpec{
		Instructions: instructions,
	}

	for _, line := range strings.Split(frontmatter, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		colonIdx := strings.Index(line, ":")
		if colonIdx == -1 {
			continue
		}

		key := strings.TrimSpace(line[:colonIdx])
		value := strings.TrimSpace(line[colonIdx+1:])

		switch key {
		case "name":
			spec.Name = value
		case "description":
			spec.Description = value
		case "model":
			spec.Model = value
		case "tools":
			spec.Tools = parseYAMLArray(value)
		case "skills":
			spec.Skills = parseYAMLArray(value)
		case "dependencies":
			spec.Dependencies = parseYAMLArray(value)
		case "requires":
			spec.Requires = parseYAMLArray(value)
		}
	}

	return spec, nil
}

// parseYAMLArray parses a simple YAML array like [a, b, c] or - items.
func parseYAMLArray(value string) []string {
	// Handle inline array format: [a, b, c]
	if strings.HasPrefix(value, "[") && strings.HasSuffix(value, "]") {
		inner := value[1 : len(value)-1]
		var result []string
		for _, item := range strings.Split(inner, ",") {
			item = strings.TrimSpace(item)
			if item != "" {
				result = append(result, item)
			}
		}
		return result
	}
	// Single value
	if value != "" {
		return []string{value}
	}
	return nil
}

// SpecLoader loads multi-agent-spec configurations.
type SpecLoader struct {
	// BaseDir is the directory containing spec files.
	BaseDir string

	// AgentsDir is the directory containing agent definitions.
	// Defaults to "agents" relative to BaseDir.
	AgentsDir string

	// TeamsDir is the directory containing team definitions.
	// Defaults to same as BaseDir.
	TeamsDir string
}

// NewSpecLoader creates a new spec loader for the given directory.
// It auto-detects common directory structures.
func NewSpecLoader(baseDir string) *SpecLoader {
	loader := &SpecLoader{
		BaseDir:   baseDir,
		AgentsDir: filepath.Join(baseDir, "agents"),
		TeamsDir:  baseDir,
	}

	// Auto-detect specs/ subdirectory structure
	if _, err := os.Stat(filepath.Join(baseDir, "specs", "agents")); err == nil {
		loader.AgentsDir = filepath.Join(baseDir, "specs", "agents")
	}
	if _, err := os.Stat(filepath.Join(baseDir, "specs", "teams")); err == nil {
		loader.TeamsDir = filepath.Join(baseDir, "specs", "teams")
	}

	return loader
}

// LoadTeam loads a team and its agents from the spec directory.
func (l *SpecLoader) LoadTeam(teamFile string) (*TeamSpec, map[string]*AgentSpec, error) {
	// Try to find team file
	teamPath := l.findTeamFile(teamFile)
	if teamPath == "" {
		return nil, nil, fmt.Errorf("team file not found: %s", teamFile)
	}

	team, err := LoadTeamSpec(teamPath)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to load team: %w", err)
	}

	// Load agent specs
	agents := make(map[string]*AgentSpec)
	for _, agentName := range team.Agents {
		agent, err := l.loadAgent(agentName)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to load agent %s: %w", agentName, err)
		}
		agents[agentName] = agent
	}

	return team, agents, nil
}

// findTeamFile searches for the team file in common locations.
func (l *SpecLoader) findTeamFile(teamFile string) string {
	candidates := []string{
		filepath.Join(l.TeamsDir, teamFile),
		filepath.Join(l.BaseDir, teamFile),
		filepath.Join(l.BaseDir, "specs", "teams", teamFile),
	}

	// Also try with different extensions
	for _, base := range []string{teamFile, strings.TrimSuffix(teamFile, ".json")} {
		candidates = append(candidates,
			filepath.Join(l.TeamsDir, base+".json"),
			filepath.Join(l.TeamsDir, base+"-team.json"),
		)
	}

	for _, path := range candidates {
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}

	return ""
}

// loadAgent loads an agent spec, trying JSON and markdown formats.
func (l *SpecLoader) loadAgent(agentName string) (*AgentSpec, error) {
	// Try different file formats
	candidates := []string{
		filepath.Join(l.AgentsDir, agentName+".json"),
		filepath.Join(l.AgentsDir, agentName+".md"),
		filepath.Join(l.AgentsDir, agentName+".yaml"),
	}

	for _, path := range candidates {
		if _, err := os.Stat(path); err == nil {
			return LoadAgentSpec(path)
		}
	}

	return nil, fmt.Errorf("agent file not found: %s (tried .json, .md, .yaml)", agentName)
}

// ToConfig converts multi-agent-spec to local Config format.
func (l *SpecLoader) ToConfig(team *TeamSpec, agents map[string]*AgentSpec, llmCfg LLMConfig) (*Config, error) {
	cfg := DefaultConfig()
	cfg.LLM = llmCfg

	// Convert each agent spec to agent config
	for _, agentName := range team.Agents {
		spec, ok := agents[agentName]
		if !ok {
			return nil, fmt.Errorf("agent %s not found in specs", agentName)
		}

		agentCfg := AgentConfig{
			Name:         spec.Name,
			Description:  spec.Description,
			Instructions: spec.Instructions,
			Tools:        MapCanonicalTools(spec.Tools),
		}

		// Map canonical model to provider-specific model
		if spec.Model != "" {
			providerName := ProviderNameFromString(llmCfg.Provider)
			agentCfg.Model = MapCanonicalModel(spec.Model, providerName)
		}

		cfg.Agents = append(cfg.Agents, agentCfg)
	}

	return &cfg, nil
}

// LoadFromSpec loads configuration from multi-agent-spec files.
// It expects a team.json file and an agents/ directory with agent definitions.
func LoadFromSpec(specDir string, llmCfg LLMConfig) (*Config, *TeamSpec, error) {
	loader := NewSpecLoader(specDir)

	// Try to find team.json
	teamPath := "team.json"
	if _, err := os.Stat(filepath.Join(specDir, teamPath)); os.IsNotExist(err) {
		// Try looking in common locations
		alternatives := []string{
			"config/team.json",
			"spec/team.json",
		}
		for _, alt := range alternatives {
			if _, err := os.Stat(filepath.Join(specDir, alt)); err == nil {
				teamPath = alt
				break
			}
		}
	}

	team, agents, err := loader.LoadTeam(teamPath)
	if err != nil {
		return nil, nil, err
	}

	cfg, err := loader.ToConfig(team, agents, llmCfg)
	if err != nil {
		return nil, nil, err
	}

	// Set workspace to spec directory
	absDir, err := filepath.Abs(specDir)
	if err != nil {
		return nil, nil, fmt.Errorf("invalid spec directory: %w", err)
	}
	cfg.Workspace = absDir

	return cfg, team, nil
}

// WorkflowType represents the type of workflow execution.
type WorkflowType string

const (
	// WorkflowSequential executes steps one after another.
	WorkflowSequential WorkflowType = "sequential"

	// WorkflowParallel executes all steps concurrently.
	WorkflowParallel WorkflowType = "parallel"

	// WorkflowDAG executes steps based on dependency graph.
	WorkflowDAG WorkflowType = "dag"

	// WorkflowOrchestrated uses an orchestrator agent to coordinate.
	WorkflowOrchestrated WorkflowType = "orchestrated"
)

// GetWorkflowType returns the workflow type from a TeamSpec.
func GetWorkflowType(team *TeamSpec) WorkflowType {
	if team.Workflow == nil {
		return WorkflowOrchestrated
	}

	switch strings.ToLower(team.Workflow.Type) {
	case "sequential":
		return WorkflowSequential
	case "parallel":
		return WorkflowParallel
	case "dag":
		return WorkflowDAG
	case "orchestrated":
		return WorkflowOrchestrated
	default:
		return WorkflowOrchestrated
	}
}

// BuildDAG creates a dependency graph from workflow steps.
// Returns a map of step name to list of dependent step names (steps that depend on this step).
func BuildDAG(workflow *WorkflowSpec) map[string][]string {
	if workflow == nil || len(workflow.Steps) == 0 {
		return nil
	}

	// Build adjacency list (forward edges: step -> dependents)
	dag := make(map[string][]string)

	// Initialize all steps
	for _, step := range workflow.Steps {
		if _, ok := dag[step.Name]; !ok {
			dag[step.Name] = []string{}
		}
	}

	// Add edges based on depends_on
	for _, step := range workflow.Steps {
		for _, dep := range step.DependsOn {
			dag[dep] = append(dag[dep], step.Name)
		}
	}

	return dag
}

// GetRootSteps returns steps with no dependencies (entry points).
func GetRootSteps(workflow *WorkflowSpec) []string {
	if workflow == nil || len(workflow.Steps) == 0 {
		return nil
	}

	var roots []string
	for _, step := range workflow.Steps {
		if len(step.DependsOn) == 0 {
			roots = append(roots, step.Name)
		}
	}
	return roots
}

// TopologicalSort returns steps in execution order respecting dependencies.
// Returns an error if there are circular dependencies.
func TopologicalSort(workflow *WorkflowSpec) ([]string, error) {
	if workflow == nil || len(workflow.Steps) == 0 {
		return nil, nil
	}

	// Build reverse adjacency list (step -> dependencies)
	deps := make(map[string][]string)
	inDegree := make(map[string]int)

	for _, step := range workflow.Steps {
		deps[step.Name] = step.DependsOn
		inDegree[step.Name] = len(step.DependsOn)
	}

	// Find all steps with no dependencies
	var queue []string
	for _, step := range workflow.Steps {
		if inDegree[step.Name] == 0 {
			queue = append(queue, step.Name)
		}
	}

	// Kahn's algorithm for topological sort
	var sorted []string
	for len(queue) > 0 {
		// Pop from queue
		current := queue[0]
		queue = queue[1:]
		sorted = append(sorted, current)

		// Find steps that depend on current
		for _, step := range workflow.Steps {
			for _, dep := range step.DependsOn {
				if dep == current {
					inDegree[step.Name]--
					if inDegree[step.Name] == 0 {
						queue = append(queue, step.Name)
					}
					break
				}
			}
		}
	}

	// Check for cycles
	if len(sorted) != len(workflow.Steps) {
		return nil, fmt.Errorf("circular dependency detected in workflow")
	}

	return sorted, nil
}

// GetStepByName finds a step by name in the workflow.
func GetStepByName(workflow *WorkflowSpec, name string) *StepSpec {
	if workflow == nil {
		return nil
	}
	for i := range workflow.Steps {
		if workflow.Steps[i].Name == name {
			return &workflow.Steps[i]
		}
	}
	return nil
}
