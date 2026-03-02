// Package local provides an embedded local mode for running agents in-process.
package local

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// DAGExecutor executes workflows based on dependency graphs.
type DAGExecutor struct {
	// agents maps agent names to their implementations.
	agents map[string]*EmbeddedAgent

	// stateBackend persists execution state.
	stateBackend StateBackend

	// maxConcurrency limits parallel step execution.
	maxConcurrency int

	// hitlHandler handles human-in-the-loop requests.
	// If nil, HITL requests cause the workflow to pause.
	hitlHandler HITLHandler
}

// NewDAGExecutor creates a new DAG executor.
func NewDAGExecutor(agents map[string]*EmbeddedAgent, stateBackend StateBackend) *DAGExecutor {
	return &DAGExecutor{
		agents:         agents,
		stateBackend:   stateBackend,
		maxConcurrency: 10, // Default max concurrency
	}
}

// SetMaxConcurrency sets the maximum number of concurrent steps.
func (e *DAGExecutor) SetMaxConcurrency(n int) {
	if n > 0 {
		e.maxConcurrency = n
	}
}

// SetHITLHandler sets the handler for human-in-the-loop requests.
// If not set, HITL requests cause the workflow to pause and wait for resume.
func (e *DAGExecutor) SetHITLHandler(handler HITLHandler) {
	e.hitlHandler = handler
}

// ExecuteWorkflow executes a workflow according to its type.
func (e *DAGExecutor) ExecuteWorkflow(ctx context.Context, team *TeamSpec, input string) (*WorkflowResult, error) {
	runID := generateRunID()

	// Initialize execution state
	state := NewExecutionState(runID, team.Name)

	// Add steps to state
	if team.Workflow != nil {
		for _, step := range team.Workflow.Steps {
			state.AddStep(step.Name, step.Agent, step.DependsOn)
		}
	}

	// Save initial state
	if e.stateBackend != nil {
		if err := e.stateBackend.SaveState(ctx, runID, state); err != nil {
			return nil, fmt.Errorf("failed to save initial state: %w", err)
		}
	}

	state.Start()

	// Execute based on workflow type
	var result *WorkflowResult
	var err error

	switch GetWorkflowType(team) {
	case WorkflowSequential:
		result, err = e.executeSequential(ctx, team, input, state)
	case WorkflowParallel:
		result, err = e.executeParallel(ctx, team, input, state)
	case WorkflowDAG:
		result, err = e.executeDAG(ctx, team, input, state)
	case WorkflowOrchestrated:
		result, err = e.executeOrchestrated(ctx, team, input, state)
	default:
		err = fmt.Errorf("unknown workflow type: %s", team.Workflow.Type)
	}

	// Update final state
	if err != nil {
		state.Fail(err.Error())
	} else {
		state.Complete()
	}

	// Save final state
	if e.stateBackend != nil {
		if saveErr := e.stateBackend.SaveState(ctx, runID, state); saveErr != nil {
			// Log but don't fail the workflow
			fmt.Printf("warning: failed to save final state: %v\n", saveErr)
		}
	}

	if result != nil {
		result.RunID = runID
		result.State = state
	}

	return result, err
}

// executeSequential executes steps one after another.
func (e *DAGExecutor) executeSequential(ctx context.Context, team *TeamSpec, input string, state *ExecutionState) (*WorkflowResult, error) {
	if team.Workflow == nil || len(team.Workflow.Steps) == 0 {
		return nil, fmt.Errorf("no steps defined in workflow")
	}

	result := &WorkflowResult{
		WorkflowID:  team.Name,
		StepResults: make(map[string]*StepResult),
	}

	// Use topological sort to get execution order
	sorted, err := TopologicalSort(team.Workflow)
	if err != nil {
		return nil, err
	}

	// Execute steps in order
	currentInput := input
	for _, stepName := range sorted {
		step := GetStepByName(team.Workflow, stepName)
		if step == nil {
			continue
		}

		stepResult, err := e.executeStep(ctx, step, currentInput, state)
		if err != nil {
			return result, fmt.Errorf("step %s failed: %w", stepName, err)
		}

		result.StepResults[stepName] = stepResult

		// Use step output as input for next step
		if stepResult.Output != "" {
			currentInput = stepResult.Output
		}
	}

	// Final output is the last step's output
	if len(sorted) > 0 {
		lastStep := sorted[len(sorted)-1]
		if lastResult, ok := result.StepResults[lastStep]; ok {
			result.FinalOutput = lastResult.Output
		}
	}

	return result, nil
}

// executeParallel executes all steps concurrently.
func (e *DAGExecutor) executeParallel(ctx context.Context, team *TeamSpec, input string, state *ExecutionState) (*WorkflowResult, error) {
	if team.Workflow == nil || len(team.Workflow.Steps) == 0 {
		return nil, fmt.Errorf("no steps defined in workflow")
	}

	result := &WorkflowResult{
		WorkflowID:  team.Name,
		StepResults: make(map[string]*StepResult),
	}

	// Create a semaphore for max concurrency
	sem := make(chan struct{}, e.maxConcurrency)

	var wg sync.WaitGroup
	var mu sync.Mutex
	var firstError error

	for _, step := range team.Workflow.Steps {
		step := step // Capture for goroutine
		wg.Add(1)

		go func() {
			defer wg.Done()

			// Acquire semaphore
			sem <- struct{}{}
			defer func() { <-sem }()

			stepResult, err := e.executeStep(ctx, &step, input, state)

			mu.Lock()
			defer mu.Unlock()

			if err != nil && firstError == nil {
				firstError = fmt.Errorf("step %s failed: %w", step.Name, err)
			}
			result.StepResults[step.Name] = stepResult
		}()
	}

	wg.Wait()

	if firstError != nil {
		return result, firstError
	}

	return result, nil
}

// executeDAG executes steps based on dependency graph with parallelism.
func (e *DAGExecutor) executeDAG(ctx context.Context, team *TeamSpec, input string, state *ExecutionState) (*WorkflowResult, error) {
	if team.Workflow == nil || len(team.Workflow.Steps) == 0 {
		return nil, fmt.Errorf("no steps defined in workflow")
	}

	result := &WorkflowResult{
		WorkflowID:  team.Name,
		StepResults: make(map[string]*StepResult),
	}

	// Build step lookup
	stepMap := make(map[string]*StepSpec)
	for i := range team.Workflow.Steps {
		stepMap[team.Workflow.Steps[i].Name] = &team.Workflow.Steps[i]
	}

	// Track completion
	completed := make(map[string]bool)
	var mu sync.Mutex
	var firstError error

	// Semaphore for concurrency control
	sem := make(chan struct{}, e.maxConcurrency)

	// Execute until all steps complete or error
	for len(completed) < len(team.Workflow.Steps) {
		// Find ready steps (dependencies satisfied)
		var ready []*StepSpec
		mu.Lock()
		for _, step := range team.Workflow.Steps {
			if completed[step.Name] {
				continue
			}

			// Check if all dependencies are satisfied
			allDepsComplete := true
			for _, dep := range step.DependsOn {
				if !completed[dep] {
					allDepsComplete = false
					break
				}
			}

			if allDepsComplete {
				ready = append(ready, stepMap[step.Name])
			}
		}
		mu.Unlock()

		if len(ready) == 0 && len(completed) < len(team.Workflow.Steps) {
			// No ready steps but not all complete - likely circular dependency
			return result, fmt.Errorf("workflow deadlock: no ready steps but %d steps incomplete",
				len(team.Workflow.Steps)-len(completed))
		}

		// Execute ready steps in parallel
		var wg sync.WaitGroup
		for _, step := range ready {
			step := step
			wg.Add(1)

			go func() {
				defer wg.Done()

				// Acquire semaphore
				sem <- struct{}{}
				defer func() { <-sem }()

				// Collect inputs from dependencies
				stepInput := input
				if len(step.DependsOn) > 0 {
					mu.Lock()
					// Use output from first dependency as input (simplified)
					if depResult, ok := result.StepResults[step.DependsOn[0]]; ok {
						stepInput = depResult.Output
					}
					mu.Unlock()
				}

				stepResult, err := e.executeStep(ctx, step, stepInput, state)

				mu.Lock()
				defer mu.Unlock()

				if err != nil && firstError == nil {
					firstError = fmt.Errorf("step %s failed: %w", step.Name, err)
				}
				result.StepResults[step.Name] = stepResult
				completed[step.Name] = true
			}()
		}

		wg.Wait()

		if firstError != nil {
			return result, firstError
		}
	}

	// Find terminal steps (no other steps depend on them)
	depCount := make(map[string]int)
	for _, step := range team.Workflow.Steps {
		for _, dep := range step.DependsOn {
			depCount[dep]++
		}
	}

	for _, step := range team.Workflow.Steps {
		if depCount[step.Name] == 0 {
			if stepResult, ok := result.StepResults[step.Name]; ok {
				result.FinalOutput = stepResult.Output
				break
			}
		}
	}

	return result, nil
}

// executeOrchestrated uses an orchestrator agent to coordinate.
func (e *DAGExecutor) executeOrchestrated(ctx context.Context, team *TeamSpec, input string, state *ExecutionState) (*WorkflowResult, error) {
	if team.Orchestrator == "" {
		return nil, fmt.Errorf("orchestrator not specified in team")
	}

	orchestrator, ok := e.agents[team.Orchestrator]
	if !ok {
		return nil, fmt.Errorf("orchestrator agent %s not found", team.Orchestrator)
	}

	result := &WorkflowResult{
		WorkflowID:  team.Name,
		StepResults: make(map[string]*StepResult),
	}

	// Execute orchestrator with full context
	startTime := time.Now()
	state.MarkStepRunning("orchestrator", input)

	agentResult, err := orchestrator.Invoke(ctx, input)

	stepResult := &StepResult{
		StepID:    "orchestrator",
		AgentName: team.Orchestrator,
		Duration:  time.Since(startTime),
	}

	if err != nil {
		stepResult.Error = err.Error()
		state.MarkStepFailed("orchestrator", err.Error())
		return result, fmt.Errorf("orchestrator failed: %w", err)
	}

	stepResult.Output = agentResult.Output
	state.MarkStepCompleted("orchestrator", agentResult.Output)

	result.StepResults["orchestrator"] = stepResult
	result.FinalOutput = agentResult.Output

	return result, nil
}

// executeStep executes a single workflow step.
func (e *DAGExecutor) executeStep(ctx context.Context, step *StepSpec, input string, state *ExecutionState) (*StepResult, error) {
	agent, ok := e.agents[step.Agent]
	if !ok {
		return nil, fmt.Errorf("agent %s not found", step.Agent)
	}

	// Check if step has a pending HITL response
	stepState := state.Steps[step.Name]
	if stepState != nil && stepState.HITLResponse != nil {
		// Append HITL response to input
		input = fmt.Sprintf("%s\n\n[Human Response]: %s", input, stepState.HITLResponse.Response)
	}

	startTime := time.Now()
	state.MarkStepRunning(step.Name, input)

	agentResult, err := agent.Invoke(ctx, input)

	result := &StepResult{
		StepID:    step.Name,
		AgentName: step.Agent,
		Duration:  time.Since(startTime),
	}

	// Check if agent is requesting HITL
	if IsHITLError(err) {
		hitlRequest := GetHITLRequest(err)

		// Try to handle HITL synchronously if handler is set
		if e.hitlHandler != nil {
			response, handlerErr := e.hitlHandler(state.RunID, step.Name, hitlRequest)
			if handlerErr != nil {
				result.Error = handlerErr.Error()
				state.MarkStepFailed(step.Name, handlerErr.Error())
				return result, handlerErr
			}
			if response != nil {
				// Got immediate response, continue with it
				state.ProvideHITLResponse(step.Name, response)
				// Re-execute step with response
				return e.executeStep(ctx, step, input, state)
			}
		}

		// No immediate response - pause for async HITL
		state.MarkStepWaitingHITL(step.Name, hitlRequest)
		state.PauseForHITL()

		// Save state so it can be resumed
		if e.stateBackend != nil {
			if saveErr := e.stateBackend.SaveState(ctx, state.RunID, state); saveErr != nil {
				fmt.Printf("warning: failed to save HITL state: %v\n", saveErr)
			}
		}

		result.Error = "waiting for human input"
		return result, ErrHITLRequired
	}

	if err != nil {
		result.Error = err.Error()
		state.MarkStepFailed(step.Name, err.Error())
		return result, err
	}

	result.Output = agentResult.Output
	state.MarkStepCompleted(step.Name, agentResult.Output)

	// Save state after each step
	if e.stateBackend != nil {
		if saveErr := e.stateBackend.SaveState(ctx, state.RunID, state); saveErr != nil {
			// Log but don't fail the step
			fmt.Printf("warning: failed to save state after step %s: %v\n", step.Name, saveErr)
		}
	}

	return result, nil
}

// WorkflowResult holds the result of a workflow execution.
type WorkflowResult struct {
	// RunID is the unique identifier for this execution.
	RunID string `json:"run_id"`

	// WorkflowID is the team/workflow name.
	WorkflowID string `json:"workflow_id"`

	// StepResults contains results for each step.
	StepResults map[string]*StepResult `json:"step_results"`

	// FinalOutput is the final output from the workflow.
	FinalOutput string `json:"final_output"`

	// State is the execution state.
	State *ExecutionState `json:"state,omitempty"`
}

// StepResult holds the result of a single step execution.
type StepResult struct {
	// StepID is the step identifier.
	StepID string `json:"step_id"`

	// AgentName is the agent that executed this step.
	AgentName string `json:"agent_name"`

	// Output is the step's output.
	Output string `json:"output"`

	// Error is the error message if the step failed.
	Error string `json:"error,omitempty"`

	// Duration is how long the step took to execute.
	Duration time.Duration `json:"duration"`
}

// generateRunID generates a unique run ID.
func generateRunID() string {
	return fmt.Sprintf("run-%d", time.Now().UnixNano())
}

// ResumeWorkflow resumes a previously saved workflow execution.
func (e *DAGExecutor) ResumeWorkflow(ctx context.Context, runID string, team *TeamSpec) (*WorkflowResult, error) {
	if e.stateBackend == nil {
		return nil, fmt.Errorf("state backend not configured")
	}

	// Load existing state
	state, err := e.stateBackend.LoadState(ctx, runID)
	if err != nil {
		return nil, fmt.Errorf("failed to load state: %w", err)
	}
	if state == nil {
		return nil, fmt.Errorf("no state found for run %s", runID)
	}

	if state.IsComplete() {
		return nil, fmt.Errorf("workflow already completed")
	}

	// Find incomplete steps and re-execute
	result := &WorkflowResult{
		RunID:       runID,
		WorkflowID:  team.Name,
		StepResults: make(map[string]*StepResult),
		State:       state,
	}

	// Re-execute based on workflow type
	switch GetWorkflowType(team) {
	case WorkflowDAG:
		// DAG can resume from partial completion
		return e.resumeDAG(ctx, team, state, result)
	default:
		return nil, fmt.Errorf("resume not supported for workflow type %s", team.Workflow.Type)
	}
}

// resumeDAG resumes a DAG workflow from partial completion.
func (e *DAGExecutor) resumeDAG(ctx context.Context, team *TeamSpec, state *ExecutionState, result *WorkflowResult) (*WorkflowResult, error) {
	// Build step lookup
	stepMap := make(map[string]*StepSpec)
	for i := range team.Workflow.Steps {
		stepMap[team.Workflow.Steps[i].Name] = &team.Workflow.Steps[i]
	}

	// Execute remaining steps
	var mu sync.Mutex
	var firstError error
	sem := make(chan struct{}, e.maxConcurrency)

	for !state.IsComplete() {
		// Get ready steps
		readySteps := state.GetReadySteps()
		if len(readySteps) == 0 {
			break
		}

		var wg sync.WaitGroup
		for _, stepName := range readySteps {
			step := stepMap[stepName]
			if step == nil {
				continue
			}

			wg.Add(1)
			go func(s *StepSpec) {
				defer wg.Done()

				sem <- struct{}{}
				defer func() { <-sem }()

				// Get input from dependencies
				input := ""
				if len(s.DependsOn) > 0 {
					mu.Lock()
					if depStep, ok := state.Steps[s.DependsOn[0]]; ok {
						input = depStep.Output
					}
					mu.Unlock()
				}

				stepResult, err := e.executeStep(ctx, s, input, state)

				mu.Lock()
				defer mu.Unlock()

				if err != nil && firstError == nil {
					firstError = err
				}
				result.StepResults[s.Name] = stepResult
			}(step)
		}

		wg.Wait()

		if firstError != nil {
			state.Fail(firstError.Error())
			return result, firstError
		}
	}

	state.Complete()

	// Save final state
	if e.stateBackend != nil {
		if err := e.stateBackend.SaveState(ctx, state.RunID, state); err != nil {
			fmt.Printf("warning: failed to save final state: %v\n", err)
		}
	}

	return result, nil
}
