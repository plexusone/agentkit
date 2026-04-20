package deploy

import (
	"fmt"
	"sort"
	"sync"
)

// ProviderName is a type alias for provider names to improve type safety.
type ProviderName string

// Standard provider names.
const (
	ProviderLightsail ProviderName = "lightsail"
	ProviderECS       ProviderName = "ecs"
	ProviderCloudRun  ProviderName = "cloudrun"
	ProviderDocker    ProviderName = "docker"
)

// ProviderFactory creates a new Provider instance.
type ProviderFactory func(cfg *DeployConfig) (Provider, error)

// registration holds a provider factory and its priority.
type registration struct {
	factory  ProviderFactory
	priority int
}

// registry is the global provider registry.
var registry = &providerRegistry{
	providers: make(map[ProviderName]registration),
}

// providerRegistry is a thread-safe registry for deployment providers.
type providerRegistry struct {
	mu        sync.RWMutex
	providers map[ProviderName]registration
}

// RegisterProvider registers a deployment provider factory with a given priority.
// Higher priority providers are preferred when multiple providers are available.
// Panics if a provider with the same name is already registered.
func RegisterProvider(name ProviderName, factory ProviderFactory, priority int) {
	registry.mu.Lock()
	defer registry.mu.Unlock()

	if _, exists := registry.providers[name]; exists {
		panic(fmt.Sprintf("deploy: provider %q already registered", name))
	}

	registry.providers[name] = registration{
		factory:  factory,
		priority: priority,
	}
}

// MustRegisterProvider is like RegisterProvider but returns an error instead of panicking.
func MustRegisterProvider(name ProviderName, factory ProviderFactory, priority int) error {
	registry.mu.Lock()
	defer registry.mu.Unlock()

	if _, exists := registry.providers[name]; exists {
		return fmt.Errorf("%w: %s", ErrProviderAlreadyRegistered, name)
	}

	registry.providers[name] = registration{
		factory:  factory,
		priority: priority,
	}
	return nil
}

// GetProvider returns a provider for the given configuration.
// It checks AGENTKIT_DEPLOY_PROVIDER env var first, then cfg.Provider, then defaults.
func GetProvider(cfg *DeployConfig) (Provider, error) {
	providerName := ProviderName(cfg.GetProviderName())
	return GetProviderByName(providerName, cfg)
}

// GetProviderByName returns a provider by its name.
func GetProviderByName(name ProviderName, cfg *DeployConfig) (Provider, error) {
	registry.mu.RLock()
	reg, exists := registry.providers[name]
	registry.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("%w: %s", ErrProviderNotFound, name)
	}

	return reg.factory(cfg)
}

// ListProviders returns a list of registered provider names, sorted by priority (highest first).
func ListProviders() []ProviderName {
	registry.mu.RLock()
	defer registry.mu.RUnlock()

	type providerWithPriority struct {
		name     ProviderName
		priority int
	}

	providers := make([]providerWithPriority, 0, len(registry.providers))
	for name, reg := range registry.providers {
		providers = append(providers, providerWithPriority{name: name, priority: reg.priority})
	}

	// Sort by priority (highest first)
	sort.Slice(providers, func(i, j int) bool {
		return providers[i].priority > providers[j].priority
	})

	names := make([]ProviderName, len(providers))
	for i, p := range providers {
		names[i] = p.name
	}

	return names
}

// HasProvider checks if a provider is registered.
func HasProvider(name ProviderName) bool {
	registry.mu.RLock()
	defer registry.mu.RUnlock()

	_, exists := registry.providers[name]
	return exists
}

// UnregisterProvider removes a provider from the registry.
// This is primarily useful for testing.
func UnregisterProvider(name ProviderName) {
	registry.mu.Lock()
	defer registry.mu.Unlock()

	delete(registry.providers, name)
}

// ResetRegistry clears all registered providers.
// This is primarily useful for testing.
func ResetRegistry() {
	registry.mu.Lock()
	defer registry.mu.Unlock()

	registry.providers = make(map[ProviderName]registration)
}

// ProviderInfo contains metadata about a registered provider.
type ProviderInfo struct {
	Name     ProviderName `json:"name"`
	Priority int          `json:"priority"`
}

// ListProviderInfo returns detailed information about registered providers.
func ListProviderInfo() []ProviderInfo {
	registry.mu.RLock()
	defer registry.mu.RUnlock()

	infos := make([]ProviderInfo, 0, len(registry.providers))
	for name, reg := range registry.providers {
		infos = append(infos, ProviderInfo{
			Name:     name,
			Priority: reg.priority,
		})
	}

	// Sort by priority (highest first)
	sort.Slice(infos, func(i, j int) bool {
		return infos[i].Priority > infos[j].Priority
	})

	return infos
}
