// Package local provides an embedded local mode for running agents in-process.
package local

import (
	"fmt"
	"os/exec"
	"strings"
)

// PrereqCheck represents the result of checking a single prerequisite.
type PrereqCheck struct {
	// Name is the CLI tool name.
	Name string `json:"name"`

	// Found indicates if the tool was found in PATH.
	Found bool `json:"found"`

	// Path is the full path to the tool (if found).
	Path string `json:"path,omitempty"`

	// Version is the tool version (if detectable).
	Version string `json:"version,omitempty"`

	// Error is the error message if the tool was not found.
	Error string `json:"error,omitempty"`
}

// PrereqResult holds the results of prerequisite validation.
type PrereqResult struct {
	// Checks contains results for each prerequisite.
	Checks []PrereqCheck `json:"checks"`

	// AllFound is true if all prerequisites were found.
	AllFound bool `json:"all_found"`

	// Missing lists the names of missing prerequisites.
	Missing []string `json:"missing,omitempty"`
}

// CheckPrerequisites validates that required CLI tools are available.
func CheckPrerequisites(requires []string) *PrereqResult {
	result := &PrereqResult{
		Checks:   make([]PrereqCheck, 0, len(requires)),
		AllFound: true,
	}

	for _, req := range requires {
		check := CheckCLI(req)
		result.Checks = append(result.Checks, check)

		if !check.Found {
			result.AllFound = false
			result.Missing = append(result.Missing, req)
		}
	}

	return result
}

// CheckCLI checks if a CLI tool is available in PATH.
func CheckCLI(name string) PrereqCheck {
	check := PrereqCheck{Name: name}

	// Look up the command in PATH
	path, err := exec.LookPath(name)
	if err != nil {
		check.Found = false
		check.Error = fmt.Sprintf("not found in PATH: %v", err)
		return check
	}

	check.Found = true
	check.Path = path

	// Try to get version
	check.Version = getToolVersion(name)

	return check
}

// getToolVersion attempts to get the version of a CLI tool.
func getToolVersion(name string) string {
	// Common version flags to try
	versionFlags := []string{"--version", "-v", "version", "-V"}

	for _, flag := range versionFlags {
		cmd := exec.Command(name, flag)
		output, err := cmd.Output()
		if err == nil && len(output) > 0 {
			// Return first line, trimmed
			lines := strings.Split(string(output), "\n")
			if len(lines) > 0 {
				version := strings.TrimSpace(lines[0])
				// Limit length
				if len(version) > 100 {
					version = version[:100] + "..."
				}
				return version
			}
		}
	}

	return ""
}

// ValidateAgentPrerequisites checks prerequisites for a single agent.
func ValidateAgentPrerequisites(agent *AgentSpec) *PrereqResult {
	return CheckPrerequisites(agent.Requires)
}

// ValidateTeamPrerequisites checks prerequisites for all agents in a team.
func ValidateTeamPrerequisites(team *TeamSpec, agents map[string]*AgentSpec) *TeamPrereqResult {
	result := &TeamPrereqResult{
		AgentResults: make(map[string]*PrereqResult),
		AllFound:     true,
		Missing:      make(map[string][]string),
	}

	// Collect all unique requirements
	allRequires := make(map[string][]string) // CLI -> agents that need it

	for _, agentName := range team.Agents {
		agent, ok := agents[agentName]
		if !ok {
			continue
		}

		for _, req := range agent.Requires {
			allRequires[req] = append(allRequires[req], agentName)
		}
	}

	// Check each unique requirement once
	checkedCLIs := make(map[string]PrereqCheck)
	for cli := range allRequires {
		checkedCLIs[cli] = CheckCLI(cli)
	}

	// Build per-agent results
	for _, agentName := range team.Agents {
		agent, ok := agents[agentName]
		if !ok {
			continue
		}

		agentResult := &PrereqResult{
			Checks:   make([]PrereqCheck, 0, len(agent.Requires)),
			AllFound: true,
		}

		for _, req := range agent.Requires {
			check := checkedCLIs[req]
			agentResult.Checks = append(agentResult.Checks, check)

			if !check.Found {
				agentResult.AllFound = false
				agentResult.Missing = append(agentResult.Missing, req)
				result.AllFound = false
				result.Missing[agentName] = append(result.Missing[agentName], req)
			}
		}

		result.AgentResults[agentName] = agentResult
	}

	return result
}

// TeamPrereqResult holds prerequisite results for an entire team.
type TeamPrereqResult struct {
	// AgentResults maps agent name to its prerequisite results.
	AgentResults map[string]*PrereqResult `json:"agent_results"`

	// AllFound is true if all prerequisites for all agents were found.
	AllFound bool `json:"all_found"`

	// Missing maps agent name to list of missing CLIs.
	Missing map[string][]string `json:"missing,omitempty"`
}

// PrintSummary returns a human-readable summary of the prerequisite check.
func (r *TeamPrereqResult) PrintSummary() string {
	if r.AllFound {
		return "All prerequisites satisfied."
	}

	var sb strings.Builder
	sb.WriteString("Missing prerequisites:\n")

	for agent, missing := range r.Missing {
		sb.WriteString(fmt.Sprintf("  %s: %s\n", agent, strings.Join(missing, ", ")))
	}

	sb.WriteString("\nInstall missing tools to continue.")
	return sb.String()
}

// CommonCLITools maps common tool names to installation hints.
var CommonCLITools = map[string]string{
	"gh":            "GitHub CLI - https://cli.github.com/ or `brew install gh`",
	"git":           "Git - https://git-scm.com/ or `brew install git`",
	"go":            "Go - https://go.dev/dl/ or `brew install go`",
	"node":          "Node.js - https://nodejs.org/ or `brew install node`",
	"npm":           "npm - included with Node.js",
	"jq":            "jq - https://jqlang.github.io/jq/ or `brew install jq`",
	"curl":          "curl - https://curl.se/ or `brew install curl`",
	"docker":        "Docker - https://docs.docker.com/get-docker/",
	"kubectl":       "kubectl - https://kubernetes.io/docs/tasks/tools/",
	"aws":           "AWS CLI - https://aws.amazon.com/cli/ or `brew install awscli`",
	"gcloud":        "Google Cloud SDK - https://cloud.google.com/sdk/docs/install",
	"terraform":     "Terraform - https://www.terraform.io/downloads or `brew install terraform`",
	"golangci-lint": "golangci-lint - https://golangci-lint.run/usage/install/ or `brew install golangci-lint`",
	"schangelog":    "schangelog - https://github.com/grokify/structured-changelog",
	"gocoverbadge":  "gocoverbadge - `go install github.com/grokify/gocoverbadge@latest`",
}

// GetInstallHint returns installation instructions for a CLI tool.
func GetInstallHint(cli string) string {
	if hint, ok := CommonCLITools[cli]; ok {
		return hint
	}
	return fmt.Sprintf("Install %s and ensure it's in your PATH", cli)
}

// PrintMissingWithHints returns missing prerequisites with installation hints.
func (r *TeamPrereqResult) PrintMissingWithHints() string {
	if r.AllFound {
		return "All prerequisites satisfied."
	}

	// Collect unique missing tools
	uniqueMissing := make(map[string]bool)
	for _, missing := range r.Missing {
		for _, cli := range missing {
			uniqueMissing[cli] = true
		}
	}

	var sb strings.Builder
	sb.WriteString("Missing prerequisites:\n\n")

	for cli := range uniqueMissing {
		sb.WriteString(fmt.Sprintf("  %s\n", cli))
		sb.WriteString(fmt.Sprintf("    %s\n\n", GetInstallHint(cli)))
	}

	return sb.String()
}
