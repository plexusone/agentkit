// Package local provides an embedded local mode for running agents in-process.
package local

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// FileStateBackend implements StateBackend using the filesystem.
// State is stored as JSON files in a directory structure:
//
//	<base_dir>/
//	  <run_id>.json
//
// This backend is suitable for single-machine deployments and development.
type FileStateBackend struct {
	baseDir string
	mu      sync.RWMutex
}

// NewFileStateBackend creates a new FileStateBackend with the given base directory.
// If the directory doesn't exist, it will be created.
func NewFileStateBackend(baseDir string) (*FileStateBackend, error) {
	// Create base directory if it doesn't exist
	if err := os.MkdirAll(baseDir, 0750); err != nil {
		return nil, fmt.Errorf("failed to create state directory: %w", err)
	}

	return &FileStateBackend{
		baseDir: baseDir,
	}, nil
}

// SaveState persists the execution state to a JSON file.
func (b *FileStateBackend) SaveState(ctx context.Context, runID string, state *ExecutionState) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	// Validate run ID to prevent path traversal
	if err := validateRunID(runID); err != nil {
		return err
	}

	// Serialize state to JSON
	data, err := state.ToJSON()
	if err != nil {
		return fmt.Errorf("failed to serialize state: %w", err)
	}

	// Write to file
	path := b.statePath(runID)
	if err := os.WriteFile(path, data, 0600); err != nil {
		return fmt.Errorf("failed to write state file: %w", err)
	}

	return nil
}

// LoadState retrieves the execution state from a JSON file.
// Returns nil, nil if no state exists for the run ID.
func (b *FileStateBackend) LoadState(ctx context.Context, runID string) (*ExecutionState, error) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	// Validate run ID to prevent path traversal
	if err := validateRunID(runID); err != nil {
		return nil, err
	}

	path := b.statePath(runID)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil // No state exists
		}
		return nil, fmt.Errorf("failed to read state file: %w", err)
	}

	state, err := FromJSON(data)
	if err != nil {
		return nil, fmt.Errorf("failed to parse state file: %w", err)
	}

	return state, nil
}

// DeleteState removes the execution state file for a given run ID.
func (b *FileStateBackend) DeleteState(ctx context.Context, runID string) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	// Validate run ID to prevent path traversal
	if err := validateRunID(runID); err != nil {
		return err
	}

	path := b.statePath(runID)
	if err := os.Remove(path); err != nil {
		if os.IsNotExist(err) {
			return nil // Already deleted
		}
		return fmt.Errorf("failed to delete state file: %w", err)
	}

	return nil
}

// ListRuns returns all run IDs, optionally filtered by workflow ID.
// If workflowID is empty, all runs are returned.
func (b *FileStateBackend) ListRuns(ctx context.Context, workflowID string) ([]string, error) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	entries, err := os.ReadDir(b.baseDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to read state directory: %w", err)
	}

	var runs []string
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}

		runID := strings.TrimSuffix(entry.Name(), ".json")

		// If filtering by workflow ID, load and check the state
		if workflowID != "" {
			state, err := b.loadStateUnlocked(runID)
			if err != nil || state == nil {
				continue
			}
			if state.WorkflowID != workflowID {
				continue
			}
		}

		runs = append(runs, runID)
	}

	return runs, nil
}

// statePath returns the file path for a given run ID.
func (b *FileStateBackend) statePath(runID string) string {
	return filepath.Join(b.baseDir, runID+".json")
}

// loadStateUnlocked loads state without acquiring the lock (for internal use).
func (b *FileStateBackend) loadStateUnlocked(runID string) (*ExecutionState, error) {
	path := b.statePath(runID)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	return FromJSON(data)
}

// validateRunID ensures the run ID is safe for use as a filename.
func validateRunID(runID string) error {
	if runID == "" {
		return fmt.Errorf("run ID cannot be empty")
	}

	// Check for path traversal attempts
	if strings.Contains(runID, "..") || strings.Contains(runID, "/") || strings.Contains(runID, "\\") {
		return fmt.Errorf("invalid run ID: contains path separators")
	}

	// Check for special characters that might cause issues
	for _, c := range runID {
		if !isValidRunIDChar(c) {
			return fmt.Errorf("invalid run ID: contains invalid character %q", c)
		}
	}

	return nil
}

// isValidRunIDChar returns true if the character is valid for a run ID.
func isValidRunIDChar(c rune) bool {
	return (c >= 'a' && c <= 'z') ||
		(c >= 'A' && c <= 'Z') ||
		(c >= '0' && c <= '9') ||
		c == '-' || c == '_'
}

// DefaultStateDir returns the default state directory path.
// This is .agent-state in the current working directory.
func DefaultStateDir() string {
	return ".agent-state"
}

// WorkspaceStateDir returns the state directory for a workspace.
// This is .agent-state within the workspace directory.
func WorkspaceStateDir(workspace string) string {
	return filepath.Join(workspace, ".agent-state")
}
