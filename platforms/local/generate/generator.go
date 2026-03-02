// Package generate provides code generation from multi-agent-spec to Go.
package generate

import (
	"bytes"
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"
	"time"
	"unicode"

	"github.com/plexusone/agentkit/platforms/local"
)

//go:embed template_*.go.tmpl template_cmd_*.go.tmpl
var templates embed.FS

// Generator generates Go code from multi-agent-spec definitions.
type Generator struct {
	// SpecDir is the directory containing the spec files.
	SpecDir string

	// OutputDir is the directory where generated code will be written.
	OutputDir string

	// ProgramName is the name of the generated CLI.
	ProgramName string

	// ModulePath is the Go module path for the generated code.
	ModulePath string
}

// GeneratorConfig holds configuration for code generation.
type GeneratorConfig struct {
	// Team is the loaded team spec.
	Team *local.TeamSpec

	// Agents is a map of agent name to agent spec.
	Agents map[string]*local.AgentSpec

	// ProgramName is the CLI program name.
	ProgramName string

	// Version is the generated code version.
	Version string

	// DefaultProvider is the default LLM provider.
	DefaultProvider string

	// DefaultModel is the default model tier.
	DefaultModel string

	// DefaultTimeout is the default execution timeout.
	DefaultTimeout time.Duration

	// SpecDir is the source spec directory.
	SpecDir string

	// GeneratedAt is when the code was generated.
	GeneratedAt time.Time
}

// AgentTemplateData holds data for agent template rendering.
type AgentTemplateData struct {
	Name        string
	NamePascal  string
	Description string
	Model       string
	Tools       []string
	Requires    []string // CLI tools required by this agent
}

// CredentialTemplateData holds data for credential template rendering.
type CredentialTemplateData struct {
	Description string
	Required    bool
	Source      string
}

// TemplateData holds all data for template rendering.
type TemplateData struct {
	ProgramName     string
	Version         string
	TeamName        string
	TeamDescription string
	Orchestrator    string
	DefaultProvider string
	DefaultModel    string
	DefaultTimeout  string
	SpecDir         string
	GeneratedAt     string
	Agents          []AgentTemplateData
	Workflow        *local.WorkflowSpec
	UniqueRequires  []string                          // All unique CLI tools required across all agents
	Credentials     map[string]CredentialTemplateData // All credentials required across all agents
}

// Generate generates Go code from the spec directory.
func (g *Generator) Generate() error {
	// Load spec
	loader := local.NewSpecLoader(g.SpecDir)

	// Try common team file names
	var team *local.TeamSpec
	var agents map[string]*local.AgentSpec
	var err error

	teamFiles := []string{
		"team.json",
		"prd-team.json",
		g.ProgramName + "-team.json",
		filepath.Base(g.SpecDir) + "-team.json",
	}

	for _, teamFile := range teamFiles {
		team, agents, err = loader.LoadTeam(teamFile)
		if err == nil {
			break
		}
	}

	if team == nil {
		return fmt.Errorf("failed to load spec (tried %v): %w", teamFiles, err)
	}

	// Create output directory
	if err := os.MkdirAll(g.OutputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// Create instructions directory
	instructionsDir := filepath.Join(g.OutputDir, "instructions")
	if err := os.MkdirAll(instructionsDir, 0755); err != nil {
		return fmt.Errorf("failed to create instructions directory: %w", err)
	}

	// Prepare template data
	data := g.prepareTemplateData(team, agents)

	// Generate main.go (Cobra root command)
	if err := g.generateFile("template_main.go.tmpl", "main.go", data); err != nil {
		return fmt.Errorf("failed to generate main.go: %w", err)
	}

	// Generate agents.go (agent definitions and Team struct)
	if err := g.generateFile("template_agents.go.tmpl", "agents.go", data); err != nil {
		return fmt.Errorf("failed to generate agents.go: %w", err)
	}

	// Generate cmd_run.go (run and resume commands)
	if err := g.generateFile("template_cmd_run.go.tmpl", "cmd_run.go", data); err != nil {
		return fmt.Errorf("failed to generate cmd_run.go: %w", err)
	}

	// Generate cmd_agent.go (agent subcommands)
	if err := g.generateFile("template_cmd_agent.go.tmpl", "cmd_agent.go", data); err != nil {
		return fmt.Errorf("failed to generate cmd_agent.go: %w", err)
	}

	// Generate cmd_prereqs.go (prerequisite checking)
	if err := g.generateFile("template_cmd_prereqs.go.tmpl", "cmd_prereqs.go", data); err != nil {
		return fmt.Errorf("failed to generate cmd_prereqs.go: %w", err)
	}

	// Generate cmd_creds.go (credential management)
	if err := g.generateFile("template_cmd_creds.go.tmpl", "cmd_creds.go", data); err != nil {
		return fmt.Errorf("failed to generate cmd_creds.go: %w", err)
	}

	// Generate cmd_describe.go (AI assistant discovery)
	if err := g.generateFile("template_cmd_describe.go.tmpl", "cmd_describe.go", data); err != nil {
		return fmt.Errorf("failed to generate cmd_describe.go: %w", err)
	}

	// Generate cmd_respond.go (human-in-the-loop)
	if err := g.generateFile("template_cmd_respond.go.tmpl", "cmd_respond.go", data); err != nil {
		return fmt.Errorf("failed to generate cmd_respond.go: %w", err)
	}

	// Generate tools.go (only if it doesn't exist - user-modifiable)
	toolsPath := filepath.Join(g.OutputDir, "tools.go")
	if _, err := os.Stat(toolsPath); os.IsNotExist(err) {
		if err := g.generateFile("template_tools.go.tmpl", "tools.go", data); err != nil {
			return fmt.Errorf("failed to generate tools.go: %w", err)
		}
	}

	// Generate workflow.go
	if err := g.generateWorkflow(team, data); err != nil {
		return fmt.Errorf("failed to generate workflow.go: %w", err)
	}

	// Write instruction files
	for _, agentName := range team.Agents {
		if agent, ok := agents[agentName]; ok {
			instructionPath := filepath.Join(instructionsDir, agentName+".md")
			if err := os.WriteFile(instructionPath, []byte(agent.Instructions), 0600); err != nil {
				return fmt.Errorf("failed to write instructions for %s: %w", agentName, err)
			}
		}
	}

	// Generate go.mod if it doesn't exist
	goModPath := filepath.Join(g.OutputDir, "go.mod")
	if _, err := os.Stat(goModPath); os.IsNotExist(err) {
		if err := g.generateGoMod(); err != nil {
			return fmt.Errorf("failed to generate go.mod: %w", err)
		}
	}

	return nil
}

// prepareTemplateData prepares data for template rendering.
func (g *Generator) prepareTemplateData(team *local.TeamSpec, agents map[string]*local.AgentSpec) *TemplateData {
	data := &TemplateData{
		ProgramName:     g.ProgramName,
		Version:         team.Version,
		TeamName:        team.Name,
		TeamDescription: team.Description,
		Orchestrator:    team.Orchestrator,
		DefaultProvider: "anthropic",
		DefaultModel:    "sonnet",
		DefaultTimeout:  "10*time.Minute",
		SpecDir:         g.SpecDir,
		GeneratedAt:     time.Now().Format(time.RFC3339),
		Workflow:        team.Workflow,
		Agents:          []AgentTemplateData{},
		Credentials:     make(map[string]CredentialTemplateData),
	}

	// Collect unique CLI requirements across all agents
	uniqueRequires := make(map[string]bool)

	for _, agentName := range team.Agents {
		if agent, ok := agents[agentName]; ok {
			agentData := AgentTemplateData{
				Name:        agent.Name,
				NamePascal:  toPascalCase(agent.Name),
				Description: escapeString(agent.Description),
				Model:       agent.Model,
				Tools:       local.MapCanonicalTools(agent.Tools),
				Requires:    agent.Requires,
			}
			if agentData.Model == "" {
				agentData.Model = "sonnet"
			}
			data.Agents = append(data.Agents, agentData)

			// Collect requires for team-wide list
			for _, req := range agent.Requires {
				uniqueRequires[req] = true
			}

			// Collect credentials for team-wide list
			for name, cred := range agent.Credentials {
				// Use agent-prefixed name to avoid collisions
				credName := agentName + "." + name
				data.Credentials[credName] = CredentialTemplateData{
					Description: escapeString(cred.Description),
					Required:    cred.Required,
					Source:      cred.Source,
				}
			}
		}
	}

	// Convert to sorted slice
	for req := range uniqueRequires {
		data.UniqueRequires = append(data.UniqueRequires, req)
	}

	return data
}

// generateFile generates a single file from a template.
func (g *Generator) generateFile(templateName, outputName string, data *TemplateData) error {
	// Read template
	tmplContent, err := templates.ReadFile(templateName)
	if err != nil {
		return fmt.Errorf("failed to read template %s: %w", templateName, err)
	}

	// Parse template
	tmpl, err := template.New(templateName).Parse(string(tmplContent))
	if err != nil {
		return fmt.Errorf("failed to parse template %s: %w", templateName, err)
	}

	// Execute template
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return fmt.Errorf("failed to execute template %s: %w", templateName, err)
	}

	// Write output
	outputPath := filepath.Join(g.OutputDir, outputName)
	if err := os.WriteFile(outputPath, buf.Bytes(), 0600); err != nil {
		return fmt.Errorf("failed to write %s: %w", outputName, err)
	}

	return nil
}

// generateWorkflow generates workflow.go with the DAG definition.
func (g *Generator) generateWorkflow(team *local.TeamSpec, _ *TemplateData) error {
	var buf bytes.Buffer

	buf.WriteString("// Code generated by agentkit generate. DO NOT EDIT.\n\n")
	buf.WriteString("package main\n\n")
	buf.WriteString("import \"github.com/plexusone/agentkit/platforms/local\"\n\n")

	// Generate workflow spec
	buf.WriteString("// TeamWorkflow defines the execution workflow.\n")
	buf.WriteString("var TeamWorkflow = &local.WorkflowSpec{\n")

	if team.Workflow != nil {
		buf.WriteString(fmt.Sprintf("\tType: %q,\n", team.Workflow.Type))

		if len(team.Workflow.Steps) > 0 {
			buf.WriteString("\tSteps: []local.StepSpec{\n")
			for _, step := range team.Workflow.Steps {
				buf.WriteString("\t\t{\n")
				buf.WriteString(fmt.Sprintf("\t\t\tName:  %q,\n", step.Name))
				buf.WriteString(fmt.Sprintf("\t\t\tAgent: %q,\n", step.Agent))

				if len(step.DependsOn) > 0 {
					buf.WriteString("\t\t\tDependsOn: []string{")
					for i, dep := range step.DependsOn {
						if i > 0 {
							buf.WriteString(", ")
						}
						buf.WriteString(fmt.Sprintf("%q", dep))
					}
					buf.WriteString("},\n")
				}

				buf.WriteString("\t\t},\n")
			}
			buf.WriteString("\t},\n")
		}
	} else {
		buf.WriteString("\tType: \"orchestrated\",\n")
	}

	buf.WriteString("}\n")

	outputPath := filepath.Join(g.OutputDir, "workflow.go")
	return os.WriteFile(outputPath, buf.Bytes(), 0600)
}

// generateGoMod generates a go.mod file.
func (g *Generator) generateGoMod() error {
	modulePath := g.ModulePath
	if modulePath == "" {
		modulePath = "generated/" + g.ProgramName
	}

	content := fmt.Sprintf(`module %s

go 1.21

require (
	github.com/plexusone/agentkit v0.5.0
	github.com/plexusone/omnivault v0.3.0
	github.com/spf13/cobra v1.8.0
)
`, modulePath)

	outputPath := filepath.Join(g.OutputDir, "go.mod")
	return os.WriteFile(outputPath, []byte(content), 0600)
}

// toPascalCase converts a hyphenated string to PascalCase.
func toPascalCase(s string) string {
	parts := strings.Split(s, "-")
	var result strings.Builder
	for _, part := range parts {
		if len(part) > 0 {
			result.WriteRune(unicode.ToUpper(rune(part[0])))
			result.WriteString(part[1:])
		}
	}
	return result.String()
}

// escapeString escapes a string for use in Go code.
func escapeString(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	s = strings.ReplaceAll(s, "\n", `\n`)
	return s
}
