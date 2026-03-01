// Package local provides an embedded local mode for running agents in-process
// with filesystem access. This enables CLI assistants like Gemini CLI and Codex
// that lack native sub-agents to leverage multi-agent orchestration via MCP.
package local

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// Config holds configuration for local embedded mode.
type Config struct {
	// Mode should be "local" for embedded mode.
	Mode string `yaml:"mode" json:"mode"`

	// Workspace is the root directory for filesystem access.
	// Defaults to current working directory.
	Workspace string `yaml:"workspace" json:"workspace"`

	// Agents defines the available agents.
	Agents []AgentConfig `yaml:"agents" json:"agents"`

	// MCP configures the MCP server interface.
	MCP MCPConfig `yaml:"mcp" json:"mcp"`

	// LLM configures the language model.
	LLM LLMConfig `yaml:"llm" json:"llm"`

	// Timeouts for various operations.
	Timeouts TimeoutConfig `yaml:"timeouts" json:"timeouts"`
}

// AgentConfig defines a single agent.
type AgentConfig struct {
	// Name is the unique identifier for the agent.
	Name string `yaml:"name" json:"name"`

	// Description explains when to use this agent.
	Description string `yaml:"description" json:"description"`

	// Instructions is the system prompt or path to a markdown file.
	Instructions string `yaml:"instructions" json:"instructions"`

	// Tools lists the tools available to this agent.
	// Available: read, write, glob, grep, shell
	Tools []string `yaml:"tools" json:"tools"`

	// Model overrides the default LLM model for this agent.
	Model string `yaml:"model,omitempty" json:"model,omitempty"`

	// MaxTokens limits the response length.
	MaxTokens int `yaml:"max_tokens,omitempty" json:"max_tokens,omitempty"`
}

// MCPConfig configures the MCP server interface.
type MCPConfig struct {
	// Enabled determines if the MCP server is active.
	Enabled bool `yaml:"enabled" json:"enabled"`

	// Transport is the MCP transport type: "stdio" or "http".
	Transport string `yaml:"transport" json:"transport"`

	// Port is used when transport is "http".
	Port int `yaml:"port,omitempty" json:"port,omitempty"`

	// ServerName is the name reported in MCP server info.
	ServerName string `yaml:"server_name,omitempty" json:"server_name,omitempty"`

	// ServerVersion is the version reported in MCP server info.
	ServerVersion string `yaml:"server_version,omitempty" json:"server_version,omitempty"`
}

// LLMConfig configures the language model provider.
type LLMConfig struct {
	// Provider is the LLM provider: "openai", "anthropic", "gemini", "ollama".
	Provider string `yaml:"provider" json:"provider"`

	// Model is the default model to use.
	Model string `yaml:"model" json:"model"`

	// APIKey is the API key (can use env var reference like ${OPENAI_API_KEY}).
	APIKey string `yaml:"api_key,omitempty" json:"api_key,omitempty"` //nolint:gosec // G117: Config needs API key field

	// BaseURL overrides the API base URL.
	BaseURL string `yaml:"base_url,omitempty" json:"base_url,omitempty"`

	// Temperature controls randomness (0.0-1.0).
	Temperature float64 `yaml:"temperature,omitempty" json:"temperature,omitempty"`
}

// TimeoutConfig defines timeouts for various operations.
type TimeoutConfig struct {
	// AgentInvoke is the timeout for a single agent invocation.
	AgentInvoke Duration `yaml:"agent_invoke" json:"agent_invoke"`

	// ShellCommand is the timeout for shell command execution.
	ShellCommand Duration `yaml:"shell_command" json:"shell_command"`

	// FileRead is the timeout for file read operations.
	FileRead Duration `yaml:"file_read" json:"file_read"`

	// ParallelTotal is the total timeout for parallel agent execution.
	ParallelTotal Duration `yaml:"parallel_total" json:"parallel_total"`
}

// Duration is a time.Duration that supports human-readable strings in JSON/YAML.
// Examples: "5m", "30s", "2h30m", "100ms"
type Duration time.Duration

// UnmarshalJSON implements json.Unmarshaler for Duration.
func (d *Duration) UnmarshalJSON(b []byte) error {
	var v interface{}
	if err := json.Unmarshal(b, &v); err != nil {
		return err
	}
	switch value := v.(type) {
	case float64:
		*d = Duration(time.Duration(value))
	case string:
		dur, err := time.ParseDuration(value)
		if err != nil {
			return fmt.Errorf("invalid duration %q: %w", value, err)
		}
		*d = Duration(dur)
	default:
		return fmt.Errorf("invalid duration type: %T", v)
	}
	return nil
}

// MarshalJSON implements json.Marshaler for Duration.
func (d Duration) MarshalJSON() ([]byte, error) {
	return json.Marshal(time.Duration(d).String())
}

// UnmarshalYAML implements yaml.Unmarshaler for Duration.
func (d *Duration) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var v interface{}
	if err := unmarshal(&v); err != nil {
		return err
	}
	switch value := v.(type) {
	case int:
		*d = Duration(time.Duration(value))
	case int64:
		*d = Duration(time.Duration(value))
	case float64:
		*d = Duration(time.Duration(value))
	case string:
		dur, err := time.ParseDuration(value)
		if err != nil {
			return fmt.Errorf("invalid duration %q: %w", value, err)
		}
		*d = Duration(dur)
	default:
		return fmt.Errorf("invalid duration type: %T", v)
	}
	return nil
}

// Duration returns the time.Duration value.
func (d Duration) Duration() time.Duration {
	return time.Duration(d)
}

// DefaultConfig returns a configuration with sensible defaults.
func DefaultConfig() Config {
	return Config{
		Mode:      "local",
		Workspace: ".",
		Agents:    []AgentConfig{},
		MCP: MCPConfig{
			Enabled:       true,
			Transport:     "stdio",
			ServerName:    "agentkit-local",
			ServerVersion: "1.0.0",
		},
		LLM: LLMConfig{
			Provider:    "openai",
			Model:       "gpt-4o",
			Temperature: 0.7,
		},
		Timeouts: TimeoutConfig{
			AgentInvoke:   Duration(5 * time.Minute),
			ShellCommand:  Duration(2 * time.Minute),
			FileRead:      Duration(30 * time.Second),
			ParallelTotal: Duration(10 * time.Minute),
		},
	}
}

// LoadConfig loads configuration from a JSON or YAML file.
// The format is detected by file extension (.json, .yaml, .yml).
func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	cfg := DefaultConfig()

	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".json":
		if err := json.Unmarshal(data, &cfg); err != nil {
			return nil, fmt.Errorf("failed to parse JSON config: %w", err)
		}
	case ".yaml", ".yml":
		if err := yaml.Unmarshal(data, &cfg); err != nil {
			return nil, fmt.Errorf("failed to parse YAML config: %w", err)
		}
	default:
		return nil, fmt.Errorf("unsupported config format %q (use .json, .yaml, or .yml)", ext)
	}

	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	return &cfg, nil
}

// ConfigFormat specifies the configuration file format.
type ConfigFormat string

const (
	// FormatJSON indicates JSON format.
	FormatJSON ConfigFormat = "json"
	// FormatYAML indicates YAML format.
	FormatYAML ConfigFormat = "yaml"
)

// LoadConfigFromBytes loads configuration from bytes with explicit format.
func LoadConfigFromBytes(data []byte, format ConfigFormat) (*Config, error) {
	cfg := DefaultConfig()

	switch format {
	case FormatJSON:
		if err := json.Unmarshal(data, &cfg); err != nil {
			return nil, fmt.Errorf("failed to parse JSON config: %w", err)
		}
	case FormatYAML:
		if err := yaml.Unmarshal(data, &cfg); err != nil {
			return nil, fmt.Errorf("failed to parse YAML config: %w", err)
		}
	default:
		return nil, fmt.Errorf("unsupported config format: %s", format)
	}

	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	return &cfg, nil
}

// Validate checks that the configuration is valid.
func (c *Config) Validate() error {
	if c.Mode != "local" {
		return fmt.Errorf("mode must be 'local', got %q", c.Mode)
	}

	if c.Workspace == "" {
		c.Workspace = "."
	}

	// Resolve workspace to absolute path
	absWorkspace, err := filepath.Abs(c.Workspace)
	if err != nil {
		return fmt.Errorf("invalid workspace path: %w", err)
	}
	c.Workspace = absWorkspace

	// Validate workspace exists
	info, err := os.Stat(c.Workspace)
	if err != nil {
		return fmt.Errorf("workspace does not exist: %w", err)
	}
	if !info.IsDir() {
		return fmt.Errorf("workspace is not a directory: %s", c.Workspace)
	}

	// Validate agents
	agentNames := make(map[string]bool)
	for i, agent := range c.Agents {
		if agent.Name == "" {
			return fmt.Errorf("agent %d: name is required", i)
		}
		if agentNames[agent.Name] {
			return fmt.Errorf("duplicate agent name: %s", agent.Name)
		}
		agentNames[agent.Name] = true

		if agent.Instructions == "" {
			return fmt.Errorf("agent %s: instructions required", agent.Name)
		}

		// Validate tools
		validTools := map[string]bool{
			"read":  true,
			"write": true,
			"glob":  true,
			"grep":  true,
			"shell": true,
		}
		for _, tool := range agent.Tools {
			if !validTools[tool] {
				return fmt.Errorf("agent %s: unknown tool %q", agent.Name, tool)
			}
		}
	}

	// Validate MCP config
	if c.MCP.Enabled {
		if c.MCP.Transport != "stdio" && c.MCP.Transport != "http" {
			return fmt.Errorf("mcp.transport must be 'stdio' or 'http'")
		}
		if c.MCP.Transport == "http" && c.MCP.Port == 0 {
			c.MCP.Port = 8080
		}
	}

	return nil
}

// GetAgentConfig returns the configuration for a specific agent.
func (c *Config) GetAgentConfig(name string) (*AgentConfig, error) {
	for i := range c.Agents {
		if c.Agents[i].Name == name {
			return &c.Agents[i], nil
		}
	}
	return nil, fmt.Errorf("agent not found: %s", name)
}

// ListAgentNames returns the names of all configured agents.
func (c *Config) ListAgentNames() []string {
	names := make([]string, len(c.Agents))
	for i, agent := range c.Agents {
		names[i] = agent.Name
	}
	return names
}
