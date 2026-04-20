package deploy

import (
	"context"
	"testing"
)

func TestRegistryBasics(t *testing.T) {
	// Clean up registry for isolated tests
	ResetRegistry()
	defer ResetRegistry()

	// Test that initially no providers are registered
	providers := ListProviders()
	if len(providers) != 0 {
		t.Errorf("expected 0 providers after reset, got %d", len(providers))
	}

	// Register a mock provider
	mockFactory := func(cfg *DeployConfig) (Provider, error) {
		return &mockProvider{name: "test"}, nil
	}

	RegisterProvider("test", mockFactory, 50)

	// Verify registration
	if !HasProvider("test") {
		t.Error("expected provider 'test' to be registered")
	}

	providers = ListProviders()
	if len(providers) != 1 {
		t.Errorf("expected 1 provider, got %d", len(providers))
	}
	if providers[0] != "test" {
		t.Errorf("expected provider name 'test', got %s", providers[0])
	}

	// Test getting provider
	cfg := NewDeployConfig()
	cfg.Provider = "test"
	p, err := GetProvider(cfg)
	if err != nil {
		t.Fatalf("failed to get provider: %v", err)
	}
	if p.Name() != "test" {
		t.Errorf("expected provider name 'test', got %s", p.Name())
	}

	// Test getting non-existent provider
	cfg.Provider = "nonexistent"
	_, err = GetProviderByName("nonexistent", cfg)
	if err == nil {
		t.Error("expected error for non-existent provider")
	}

	// Test unregister
	UnregisterProvider("test")
	if HasProvider("test") {
		t.Error("expected provider 'test' to be unregistered")
	}
}

func TestRegistryPriority(t *testing.T) {
	ResetRegistry()
	defer ResetRegistry()

	// Register providers with different priorities
	mockFactory := func(name string) ProviderFactory {
		return func(cfg *DeployConfig) (Provider, error) {
			return &mockProvider{name: name}, nil
		}
	}

	RegisterProvider("low", mockFactory("low"), 10)
	RegisterProvider("high", mockFactory("high"), 100)
	RegisterProvider("medium", mockFactory("medium"), 50)

	providers := ListProviders()
	if len(providers) != 3 {
		t.Fatalf("expected 3 providers, got %d", len(providers))
	}

	// Providers should be sorted by priority (highest first)
	if providers[0] != "high" {
		t.Errorf("expected first provider to be 'high', got %s", providers[0])
	}
	if providers[1] != "medium" {
		t.Errorf("expected second provider to be 'medium', got %s", providers[1])
	}
	if providers[2] != "low" {
		t.Errorf("expected third provider to be 'low', got %s", providers[2])
	}
}

func TestListProviderInfo(t *testing.T) {
	ResetRegistry()
	defer ResetRegistry()

	mockFactory := func(cfg *DeployConfig) (Provider, error) {
		return &mockProvider{name: "info-test"}, nil
	}

	RegisterProvider("info-test", mockFactory, 75)

	infos := ListProviderInfo()
	if len(infos) != 1 {
		t.Fatalf("expected 1 provider info, got %d", len(infos))
	}

	if infos[0].Name != "info-test" {
		t.Errorf("expected name 'info-test', got %s", infos[0].Name)
	}
	if infos[0].Priority != 75 {
		t.Errorf("expected priority 75, got %d", infos[0].Priority)
	}
}

// mockProvider is a simple mock implementation of Provider for testing.
type mockProvider struct {
	name string
}

func (m *mockProvider) Deploy(ctx context.Context, cfg *DeployConfig) (*DeploymentStatus, error) {
	return &DeploymentStatus{
		StackName: cfg.Stack.Name,
		State:     StateSucceeded,
		Provider:  m.name,
	}, nil
}

func (m *mockProvider) Status(ctx context.Context, stackName string) (*DeploymentStatus, error) {
	return &DeploymentStatus{
		StackName: stackName,
		State:     StateSucceeded,
		Provider:  m.name,
	}, nil
}

func (m *mockProvider) Destroy(ctx context.Context, stackName string) error {
	return nil
}

func (m *mockProvider) Name() string {
	return m.name
}

func (m *mockProvider) Capabilities() Capabilities {
	return Capabilities{
		HTTPS:       true,
		Preview:     true,
		MaxMemoryMB: 1024,
	}
}

func (m *mockProvider) Close() error {
	return nil
}
