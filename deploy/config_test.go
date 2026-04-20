package deploy

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNewDeployConfig(t *testing.T) {
	cfg := NewDeployConfig()

	// Check defaults
	if cfg.Provider != DefaultProvider {
		t.Errorf("expected default provider %s, got %s", DefaultProvider, cfg.Provider)
	}
	if cfg.Stack.Image.Tag != "latest" {
		t.Errorf("expected default image tag 'latest', got %s", cfg.Stack.Image.Tag)
	}
	if cfg.Stack.Resources.Port != 8080 {
		t.Errorf("expected default port 8080, got %d", cfg.Stack.Resources.Port)
	}
	if cfg.Stack.Resources.CPU != "0.25" {
		t.Errorf("expected default CPU '0.25', got %s", cfg.Stack.Resources.CPU)
	}
	if cfg.Stack.Resources.Memory != "512" {
		t.Errorf("expected default memory '512', got %s", cfg.Stack.Resources.Memory)
	}
}

func TestGetProviderName(t *testing.T) {
	// Test default
	cfg := NewDeployConfig()
	if cfg.GetProviderName() != DefaultProvider {
		t.Errorf("expected %s, got %s", DefaultProvider, cfg.GetProviderName())
	}

	// Test config override
	cfg.Provider = "ecs"
	if cfg.GetProviderName() != "ecs" {
		t.Errorf("expected 'ecs', got %s", cfg.GetProviderName())
	}

	// Test env var override
	t.Setenv(EnvDeployProvider, "cloudrun")

	if cfg.GetProviderName() != "cloudrun" {
		t.Errorf("expected 'cloudrun' from env, got %s", cfg.GetProviderName())
	}
}

func TestValidate(t *testing.T) {
	tests := []struct {
		name    string
		cfg     func() *DeployConfig
		wantErr bool
	}{
		{
			name: "valid config",
			cfg: func() *DeployConfig {
				cfg := NewDeployConfig()
				cfg.Stack.Name = "test-stack"
				cfg.Stack.Project = "test-project"
				cfg.Stack.Image.Repository = "nginx"
				return cfg
			},
			wantErr: false,
		},
		{
			name: "missing stack name",
			cfg: func() *DeployConfig {
				cfg := NewDeployConfig()
				cfg.Stack.Project = "test-project"
				cfg.Stack.Image.Repository = "nginx"
				return cfg
			},
			wantErr: true,
		},
		{
			name: "missing project",
			cfg: func() *DeployConfig {
				cfg := NewDeployConfig()
				cfg.Stack.Name = "test-stack"
				cfg.Stack.Image.Repository = "nginx"
				return cfg
			},
			wantErr: true,
		},
		{
			name: "missing image",
			cfg: func() *DeployConfig {
				cfg := NewDeployConfig()
				cfg.Stack.Name = "test-stack"
				cfg.Stack.Project = "test-project"
				cfg.Stack.Image.Repository = ""
				cfg.Stack.Image.Tag = ""
				return cfg
			},
			wantErr: true,
		},
		{
			name: "valid with build config",
			cfg: func() *DeployConfig {
				cfg := NewDeployConfig()
				cfg.Stack.Name = "test-stack"
				cfg.Stack.Project = "test-project"
				cfg.Stack.Image.Repository = ""
				cfg.Stack.Image.Build = &BuildConfig{
					Context:    ".",
					Dockerfile: "Dockerfile",
				}
				return cfg
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := tt.cfg()
			err := cfg.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestLoadDeployConfig(t *testing.T) {
	// Create a temp config file
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "deploy.yaml")

	content := `
stack:
  name: test-stack
  project: test-project
  region: us-west-2
  image:
    repository: myapp
    tag: v1.0.0
  resources:
    cpu: "1"
    memory: "2048"
    port: 3000
provider: lightsail
dry_run: true
`
	if err := os.WriteFile(configPath, []byte(content), 0600); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	cfg, err := LoadDeployConfig(configPath)
	if err != nil {
		t.Fatalf("LoadDeployConfig failed: %v", err)
	}

	// Verify loaded values
	if cfg.Stack.Name != "test-stack" {
		t.Errorf("expected stack name 'test-stack', got %s", cfg.Stack.Name)
	}
	if cfg.Stack.Project != "test-project" {
		t.Errorf("expected project 'test-project', got %s", cfg.Stack.Project)
	}
	if cfg.Stack.Region != "us-west-2" {
		t.Errorf("expected region 'us-west-2', got %s", cfg.Stack.Region)
	}
	if cfg.Stack.Image.Repository != "myapp" {
		t.Errorf("expected image repository 'myapp', got %s", cfg.Stack.Image.Repository)
	}
	if cfg.Stack.Image.Tag != "v1.0.0" {
		t.Errorf("expected image tag 'v1.0.0', got %s", cfg.Stack.Image.Tag)
	}
	if cfg.Stack.Resources.CPU != "1" {
		t.Errorf("expected CPU '1', got %s", cfg.Stack.Resources.CPU)
	}
	if cfg.Stack.Resources.Memory != "2048" {
		t.Errorf("expected memory '2048', got %s", cfg.Stack.Resources.Memory)
	}
	if cfg.Stack.Resources.Port != 3000 {
		t.Errorf("expected port 3000, got %d", cfg.Stack.Resources.Port)
	}
	if cfg.Provider != "lightsail" {
		t.Errorf("expected provider 'lightsail', got %s", cfg.Provider)
	}
	if !cfg.DryRun {
		t.Error("expected DryRun to be true")
	}
}

func TestLoadDeployConfigNotFound(t *testing.T) {
	_, err := LoadDeployConfig("/nonexistent/path/config.yaml")
	if err == nil {
		t.Error("expected error for non-existent file")
	}
}

func TestEnvOverrides(t *testing.T) {
	// Set env vars (t.Setenv auto-cleans up after test)
	t.Setenv(EnvDeployProvider, "ecs")
	t.Setenv(EnvPulumiBackend, "s3://my-bucket/pulumi")
	t.Setenv(EnvDryRun, "true")
	t.Setenv(EnvAutoApprove, "1")

	// Create a minimal config file
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "deploy.yaml")

	content := `
stack:
  name: test-stack
  project: test-project
  image:
    repository: myapp
provider: lightsail
dry_run: false
auto_approve: false
`
	if err := os.WriteFile(configPath, []byte(content), 0600); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	cfg, err := LoadDeployConfig(configPath)
	if err != nil {
		t.Fatalf("LoadDeployConfig failed: %v", err)
	}

	// Verify env overrides
	if cfg.Provider != "ecs" {
		t.Errorf("expected provider 'ecs' from env, got %s", cfg.Provider)
	}
	if cfg.PulumiBackend != "s3://my-bucket/pulumi" {
		t.Errorf("expected pulumi backend from env, got %s", cfg.PulumiBackend)
	}
	if !cfg.DryRun {
		t.Error("expected DryRun to be true from env")
	}
	if !cfg.AutoApprove {
		t.Error("expected AutoApprove to be true from env")
	}
}
