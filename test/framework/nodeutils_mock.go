package framework

import (
	"context"
	"sync"
	"time"
)

// MockStatsClient implements nodeutils.StatsClient for testing.
// It allows setting expected return values for CPU and memory stats.
type MockStatsClient struct {
	mu sync.RWMutex

	// CPU stats configuration
	cpuUsage float64 // CPU usage in cores (e.g., 0.5 = 500m)
	cpuErr   error

	// Memory stats configuration
	memoryUsage uint64 // Memory usage in bytes
	memoryErr   error

	// Call tracking for verification
	cpuStatsCalls    []time.Duration
	memoryStatsCalls []time.Duration
}

// NewMockStatsClient creates a new mock stats client with default values.
func NewMockStatsClient() *MockStatsClient {
	return &MockStatsClient{
		cpuUsage:    0.1,               // Default 100m CPU usage
		memoryUsage: 512 * 1024 * 1024, // Default 512Mi memory usage
	}
}

// GetCPUStats returns the configured CPU usage.
func (m *MockStatsClient) GetCPUStats(ctx context.Context, since time.Duration) (float64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.cpuStatsCalls = append(m.cpuStatsCalls, since)

	if m.cpuErr != nil {
		return 0, m.cpuErr
	}
	return m.cpuUsage, nil
}

// GetMemoryStats returns the configured memory usage.
func (m *MockStatsClient) GetMemoryStats(ctx context.Context, since time.Duration) (uint64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.memoryStatsCalls = append(m.memoryStatsCalls, since)

	if m.memoryErr != nil {
		return 0, m.memoryErr
	}
	return m.memoryUsage, nil
}

// SetCPUUsage sets the CPU usage that will be returned by GetCPUStats.
// Usage is in cores (e.g., 0.5 = 500m, 1.0 = 1000m).
func (m *MockStatsClient) SetCPUUsage(usage float64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.cpuUsage = usage
}

// SetCPUUsageMillicores sets the CPU usage in millicores.
// Convenience method that converts millicores to cores.
func (m *MockStatsClient) SetCPUUsageMillicores(millicores int64) {
	m.SetCPUUsage(float64(millicores) / 1000)
}

// SetCPUUsagePercent sets CPU usage as a percentage of a given request.
// Example: SetCPUUsagePercent(80, 1000) sets usage to 80% of 1000m = 800m.
func (m *MockStatsClient) SetCPUUsagePercent(percent int, requestMillicores int64) {
	usage := float64(requestMillicores) * float64(percent) / 100 / 1000
	m.SetCPUUsage(usage)
}

// SetCPUError sets an error to be returned by GetCPUStats.
func (m *MockStatsClient) SetCPUError(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.cpuErr = err
}

// SetMemoryUsage sets the memory usage in bytes.
func (m *MockStatsClient) SetMemoryUsage(bytes uint64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.memoryUsage = bytes
}

// SetMemoryUsageMiB sets the memory usage in MiB.
func (m *MockStatsClient) SetMemoryUsageMiB(mib int64) {
	m.SetMemoryUsage(uint64(mib * 1024 * 1024))
}

// SetMemoryUsageGiB sets the memory usage in GiB.
func (m *MockStatsClient) SetMemoryUsageGiB(gib float64) {
	m.SetMemoryUsage(uint64(gib * 1024 * 1024 * 1024))
}

// SetMemoryUsagePercent sets memory usage as a percentage of a given request.
// Example: SetMemoryUsagePercent(80, 1*1024*1024*1024) sets usage to 80% of 1Gi.
func (m *MockStatsClient) SetMemoryUsagePercent(percent int, requestBytes int64) {
	usage := uint64(float64(requestBytes) * float64(percent) / 100)
	m.SetMemoryUsage(usage)
}

// SetMemoryError sets an error to be returned by GetMemoryStats.
func (m *MockStatsClient) SetMemoryError(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.memoryErr = err
}

// GetCPUStatsCalls returns the durations passed to GetCPUStats calls.
func (m *MockStatsClient) GetCPUStatsCalls() []time.Duration {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return append([]time.Duration{}, m.cpuStatsCalls...)
}

// GetMemoryStatsCalls returns the durations passed to GetMemoryStats calls.
func (m *MockStatsClient) GetMemoryStatsCalls() []time.Duration {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return append([]time.Duration{}, m.memoryStatsCalls...)
}

// ResetCalls clears the call tracking.
func (m *MockStatsClient) ResetCalls() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.cpuStatsCalls = nil
	m.memoryStatsCalls = nil
}

// Reset resets all configuration to defaults.
func (m *MockStatsClient) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.cpuUsage = 0.1
	m.cpuErr = nil
	m.memoryUsage = 512 * 1024 * 1024
	m.memoryErr = nil
	m.cpuStatsCalls = nil
	m.memoryStatsCalls = nil
}

// MockStatsClientRegistry manages mock stats clients for different hosts.
// This allows setting up different mock behaviors for different ChainNodes.
type MockStatsClientRegistry struct {
	mu      sync.RWMutex
	clients map[string]*MockStatsClient
	// defaultClient is returned for hosts that don't have a specific mock
	defaultClient *MockStatsClient
}

// NewMockStatsClientRegistry creates a new registry with a default mock client.
func NewMockStatsClientRegistry() *MockStatsClientRegistry {
	return &MockStatsClientRegistry{
		clients:       make(map[string]*MockStatsClient),
		defaultClient: NewMockStatsClient(),
	}
}

// GetOrCreate returns the mock client for the given host, creating one if it doesn't exist.
func (r *MockStatsClientRegistry) GetOrCreate(host string) *MockStatsClient {
	r.mu.Lock()
	defer r.mu.Unlock()

	if client, ok := r.clients[host]; ok {
		return client
	}

	client := NewMockStatsClient()
	r.clients[host] = client
	return client
}

// Get returns the mock client for the given host, or the default if none exists.
func (r *MockStatsClientRegistry) Get(host string) *MockStatsClient {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if client, ok := r.clients[host]; ok {
		return client
	}
	return r.defaultClient
}

// SetDefault sets the default mock client used for unknown hosts.
func (r *MockStatsClientRegistry) SetDefault(client *MockStatsClient) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.defaultClient = client
}

// Default returns the default mock client.
func (r *MockStatsClientRegistry) Default() *MockStatsClient {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.defaultClient
}

// Factory returns a StatsClientFactory that uses this registry.
// This factory can be injected into the Reconciler for testing.
func (r *MockStatsClientRegistry) Factory() func(host string) interface{} {
	return func(host string) interface{} {
		return r.Get(host)
	}
}

// Reset clears all registered clients and resets the default.
func (r *MockStatsClientRegistry) Reset() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.clients = make(map[string]*MockStatsClient)
	r.defaultClient = NewMockStatsClient()
}
