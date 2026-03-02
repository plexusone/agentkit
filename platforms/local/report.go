// Package local provides an embedded local mode for running agents in-process.
package local

import (
	"encoding/json"
	"fmt"
	"time"
)

// ReportStatus represents the validation status following NASA Go/No-Go terminology.
type ReportStatus string

const (
	// StatusGO indicates all checks passed.
	StatusGO ReportStatus = "GO"

	// StatusWARN indicates non-blocking issues were found.
	StatusWARN ReportStatus = "WARN"

	// StatusNOGO indicates blocking issues were found.
	StatusNOGO ReportStatus = "NO-GO"

	// StatusSKIP indicates the check was skipped.
	StatusSKIP ReportStatus = "SKIP"
)

// TeamReport represents an aggregated multi-agent team validation report.
// See: https://github.com/plexusone/multi-agent-spec/blob/main/schema/report/team-report.schema.json
type TeamReport struct {
	// Schema is the JSON Schema reference for validation.
	Schema string `json:"$schema,omitempty"`

	// Project is the project identifier (e.g., "github.com/org/repo").
	Project string `json:"project"`

	// Version is the target release version.
	Version string `json:"version"`

	// Target is a human-readable target description.
	Target string `json:"target,omitempty"`

	// Phase is the current workflow phase.
	Phase string `json:"phase"`

	// Teams contains validation results from each agent/team.
	Teams []TeamResult `json:"teams"`

	// Status is the overall status computed from all teams.
	Status ReportStatus `json:"status"`

	// GeneratedAt is when the report was generated.
	GeneratedAt time.Time `json:"generated_at"`

	// GeneratedBy is the identifier of the coordinator.
	GeneratedBy string `json:"generated_by,omitempty"`
}

// TeamResult represents validation results from a single team/agent.
type TeamResult struct {
	// ID is the workflow step identifier.
	ID string `json:"id"`

	// Name is the agent name.
	Name string `json:"name"`

	// AgentID is the agent identifier matching team.json agents list.
	AgentID string `json:"agent_id,omitempty"`

	// Model is the LLM model used by this agent.
	Model string `json:"model,omitempty"`

	// DependsOn lists IDs of upstream teams this team depends on.
	DependsOn []string `json:"depends_on,omitempty"`

	// Tasks contains individual task results.
	Tasks []TaskResult `json:"tasks"`

	// Status is the overall status for this team.
	Status ReportStatus `json:"status"`
}

// TaskResult represents the result of an individual task.
type TaskResult struct {
	// ID is the task identifier.
	ID string `json:"id"`

	// Status is the task status.
	Status ReportStatus `json:"status"`

	// Detail contains additional information.
	Detail string `json:"detail,omitempty"`

	// DurationMS is the execution time in milliseconds.
	DurationMS int64 `json:"duration_ms,omitempty"`

	// Metadata contains structured data about the task execution.
	Metadata map[string]any `json:"metadata,omitempty"`
}

// AgentResult represents individual agent validation output.
// See: https://github.com/plexusone/multi-agent-spec/blob/main/schema/report/agent-result.schema.json
type AgentResultReport struct {
	// Schema is the JSON Schema reference for validation.
	Schema string `json:"$schema,omitempty"`

	// AgentID is the agent identifier.
	AgentID string `json:"agent_id"`

	// StepID is the workflow step identifier.
	StepID string `json:"step_id"`

	// Inputs contains values received from upstream agents.
	Inputs map[string]any `json:"inputs,omitempty"`

	// Outputs contains values produced for downstream agents.
	Outputs map[string]any `json:"outputs,omitempty"`

	// Checks contains individual validation checks.
	Checks []CheckResult `json:"checks"`

	// Status is the overall status for this agent.
	Status ReportStatus `json:"status"`

	// ExecutedAt is when the agent completed execution.
	ExecutedAt time.Time `json:"executed_at"`

	// AgentModel is the LLM model used.
	AgentModel string `json:"agent_model,omitempty"`

	// Duration is the execution duration.
	Duration string `json:"duration,omitempty"`

	// Error is the error message if the agent failed.
	Error string `json:"error,omitempty"`
}

// CheckResult represents the result of an individual validation check.
type CheckResult struct {
	// ID is the check identifier.
	ID string `json:"id"`

	// Status is the check status.
	Status ReportStatus `json:"status"`

	// Detail contains additional information.
	Detail string `json:"detail,omitempty"`

	// Metadata contains structured data about the check.
	Metadata map[string]any `json:"metadata,omitempty"`
}

// ReportBuilder helps construct team reports from workflow results.
type ReportBuilder struct {
	project     string
	version     string
	target      string
	phase       string
	generatedBy string
}

// NewReportBuilder creates a new report builder.
func NewReportBuilder(project, version string) *ReportBuilder {
	return &ReportBuilder{
		project: project,
		version: version,
	}
}

// SetTarget sets the target description.
func (b *ReportBuilder) SetTarget(target string) *ReportBuilder {
	b.target = target
	return b
}

// SetPhase sets the workflow phase.
func (b *ReportBuilder) SetPhase(phase string) *ReportBuilder {
	b.phase = phase
	return b
}

// SetGeneratedBy sets the coordinator identifier.
func (b *ReportBuilder) SetGeneratedBy(generatedBy string) *ReportBuilder {
	b.generatedBy = generatedBy
	return b
}

// BuildFromWorkflowResult creates a team report from a workflow result.
func (b *ReportBuilder) BuildFromWorkflowResult(result *WorkflowResult, team *TeamSpec) *TeamReport {
	report := &TeamReport{
		Schema:      "https://raw.githubusercontent.com/plexusone/multi-agent-spec/main/schema/report/team-report.schema.json",
		Project:     b.project,
		Version:     b.version,
		Target:      b.target,
		Phase:       b.phase,
		GeneratedAt: time.Now(),
		GeneratedBy: b.generatedBy,
		Teams:       []TeamResult{},
		Status:      StatusGO,
	}

	// Convert step results to team results
	if team.Workflow != nil {
		for _, step := range team.Workflow.Steps {
			teamResult := TeamResult{
				ID:        step.Name,
				Name:      step.Agent,
				AgentID:   step.Agent,
				DependsOn: step.DependsOn,
				Tasks:     []TaskResult{},
				Status:    StatusGO,
			}

			// Get step result
			if stepResult, ok := result.StepResults[step.Name]; ok {
				// Create a task result from the step
				taskResult := TaskResult{
					ID:         "execution",
					DurationMS: stepResult.Duration.Milliseconds(),
				}

				if stepResult.Error != "" {
					taskResult.Status = StatusNOGO
					taskResult.Detail = stepResult.Error
					teamResult.Status = StatusNOGO
				} else {
					taskResult.Status = StatusGO
					taskResult.Detail = "Completed successfully"
				}

				teamResult.Tasks = append(teamResult.Tasks, taskResult)
			} else {
				// Step didn't execute
				teamResult.Status = StatusSKIP
				teamResult.Tasks = append(teamResult.Tasks, TaskResult{
					ID:     "execution",
					Status: StatusSKIP,
					Detail: "Step did not execute",
				})
			}

			report.Teams = append(report.Teams, teamResult)
		}
	}

	// Compute overall status
	report.Status = computeOverallStatus(report.Teams)

	return report
}

// BuildAgentResult creates an agent result report from a step result.
func BuildAgentResult(stepResult *StepResult, stepSpec *StepSpec, model string) *AgentResultReport {
	result := &AgentResultReport{
		Schema:     "https://raw.githubusercontent.com/plexusone/multi-agent-spec/main/schema/report/agent-result.schema.json",
		AgentID:    stepResult.AgentName,
		StepID:     stepResult.StepID,
		ExecutedAt: time.Now(),
		AgentModel: model,
		Duration:   stepResult.Duration.String(),
		Checks:     []CheckResult{},
	}

	// Add execution check
	executionCheck := CheckResult{
		ID: "execution",
	}

	if stepResult.Error != "" {
		executionCheck.Status = StatusNOGO
		executionCheck.Detail = stepResult.Error
		result.Status = StatusNOGO
		result.Error = stepResult.Error
	} else {
		executionCheck.Status = StatusGO
		executionCheck.Detail = "Agent completed successfully"
		result.Status = StatusGO
	}

	result.Checks = append(result.Checks, executionCheck)

	// Add output to outputs map
	if stepResult.Output != "" {
		result.Outputs = map[string]any{
			"result": stepResult.Output,
		}
	}

	return result
}

// computeOverallStatus computes the overall status from team results.
func computeOverallStatus(teams []TeamResult) ReportStatus {
	hasWarn := false

	for _, team := range teams {
		switch team.Status {
		case StatusNOGO:
			return StatusNOGO
		case StatusWARN:
			hasWarn = true
		}
	}

	if hasWarn {
		return StatusWARN
	}

	return StatusGO
}

// ToJSON serializes the team report to JSON.
func (r *TeamReport) ToJSON() ([]byte, error) {
	return json.MarshalIndent(r, "", "  ")
}

// ToJSON serializes the agent result to JSON.
func (r *AgentResultReport) ToJSON() ([]byte, error) {
	return json.MarshalIndent(r, "", "  ")
}

// PrintSummary prints a human-readable summary of the team report.
func (r *TeamReport) PrintSummary() string {
	summary := fmt.Sprintf("=== Team Report: %s v%s ===\n", r.Project, r.Version)
	summary += fmt.Sprintf("Phase: %s\n", r.Phase)
	summary += fmt.Sprintf("Status: %s\n", r.Status)
	summary += fmt.Sprintf("Generated: %s\n\n", r.GeneratedAt.Format(time.RFC3339))

	for _, team := range r.Teams {
		summary += fmt.Sprintf("  [%s] %s: %s\n", team.Status, team.Name, team.ID)
		for _, task := range team.Tasks {
			detail := task.Detail
			if len(detail) > 50 {
				detail = detail[:50] + "..."
			}
			summary += fmt.Sprintf("    - [%s] %s: %s\n", task.Status, task.ID, detail)
		}
	}

	return summary
}

// MergeReports merges multiple team reports into one.
func MergeReports(reports []*TeamReport) *TeamReport {
	if len(reports) == 0 {
		return nil
	}

	// Use first report as base
	merged := &TeamReport{
		Schema:      reports[0].Schema,
		Project:     reports[0].Project,
		Version:     reports[0].Version,
		Target:      reports[0].Target,
		Phase:       reports[0].Phase,
		GeneratedAt: time.Now(),
		GeneratedBy: reports[0].GeneratedBy,
		Teams:       []TeamResult{},
	}

	// Collect all teams
	for _, report := range reports {
		merged.Teams = append(merged.Teams, report.Teams...)
	}

	// Compute overall status
	merged.Status = computeOverallStatus(merged.Teams)

	return merged
}
