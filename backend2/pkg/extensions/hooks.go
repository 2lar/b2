package extensions

import (
	"context"
	"fmt"
	"sync"
)

// HookPoint represents a point in the application where hooks can be registered
type HookPoint string

const (
	// Command hooks
	HookBeforeCommandExecute HookPoint = "before_command_execute"
	HookAfterCommandExecute  HookPoint = "after_command_execute"
	HookCommandFailed        HookPoint = "command_failed"
	
	// Query hooks
	HookBeforeQueryExecute HookPoint = "before_query_execute"
	HookAfterQueryExecute  HookPoint = "after_query_execute"
	HookQueryFailed        HookPoint = "query_failed"
	
	// Entity lifecycle hooks
	HookBeforeEntityCreate HookPoint = "before_entity_create"
	HookAfterEntityCreate  HookPoint = "after_entity_create"
	HookBeforeEntityUpdate HookPoint = "before_entity_update"
	HookAfterEntityUpdate  HookPoint = "after_entity_update"
	HookBeforeEntityDelete HookPoint = "before_entity_delete"
	HookAfterEntityDelete  HookPoint = "after_entity_delete"
	
	// Graph operations
	HookBeforeGraphOperation HookPoint = "before_graph_operation"
	HookAfterGraphOperation  HookPoint = "after_graph_operation"
	HookGraphAnalysis        HookPoint = "graph_analysis"
	
	// Authentication & Authorization
	HookAfterAuthentication  HookPoint = "after_authentication"
	HookBeforeAuthorization  HookPoint = "before_authorization"
	HookAfterAuthorization   HookPoint = "after_authorization"
	
	// Data transformation
	HookBeforeSerialization   HookPoint = "before_serialization"
	HookAfterDeserialization  HookPoint = "after_deserialization"
	
	// Cache operations
	HookCacheMiss            HookPoint = "cache_miss"
	HookCacheHit             HookPoint = "cache_hit"
	HookCacheInvalidation    HookPoint = "cache_invalidation"
)

// Hook represents a function that can be executed at a hook point
type Hook func(ctx context.Context, data interface{}) error

// HookManager manages hooks for extension points
type HookManager struct {
	hooks map[HookPoint][]Hook
	mu    sync.RWMutex
}

// NewHookManager creates a new hook manager
func NewHookManager() *HookManager {
	return &HookManager{
		hooks: make(map[HookPoint][]Hook),
	}
}

// Register registers a hook for a specific hook point
func (m *HookManager) Register(point HookPoint, hook Hook) {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	if m.hooks[point] == nil {
		m.hooks[point] = []Hook{}
	}
	m.hooks[point] = append(m.hooks[point], hook)
}

// Execute executes all hooks for a specific hook point
func (m *HookManager) Execute(ctx context.Context, point HookPoint, data interface{}) error {
	m.mu.RLock()
	hooks := m.hooks[point]
	m.mu.RUnlock()
	
	for i, hook := range hooks {
		if err := hook(ctx, data); err != nil {
			return fmt.Errorf("hook %d at %s failed: %w", i, point, err)
		}
	}
	
	return nil
}

// ExecuteAsync executes hooks asynchronously
func (m *HookManager) ExecuteAsync(ctx context.Context, point HookPoint, data interface{}) {
	m.mu.RLock()
	hooks := m.hooks[point]
	m.mu.RUnlock()
	
	for _, hook := range hooks {
		go func(h Hook) {
			_ = h(ctx, data) // Ignore errors in async execution
		}(hook)
	}
}

// Clear removes all hooks for a specific hook point
func (m *HookManager) Clear(point HookPoint) {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	delete(m.hooks, point)
}

// ClearAll removes all registered hooks
func (m *HookManager) ClearAll() {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	m.hooks = make(map[HookPoint][]Hook)
}

// HookData represents data passed to hooks
type HookData struct {
	EntityType string                 `json:"entity_type"`
	EntityID   string                 `json:"entity_id"`
	Operation  string                 `json:"operation"`
	UserID     string                 `json:"user_id"`
	Before     interface{}            `json:"before,omitempty"`
	After      interface{}            `json:"after,omitempty"`
	Metadata   map[string]interface{} `json:"metadata,omitempty"`
}

// Plugin represents an extension plugin
type Plugin interface {
	// Name returns the plugin name
	Name() string
	
	// Version returns the plugin version
	Version() string
	
	// Initialize initializes the plugin
	Initialize(ctx context.Context) error
	
	// RegisterHooks registers the plugin's hooks
	RegisterHooks(manager *HookManager) error
	
	// Shutdown gracefully shuts down the plugin
	Shutdown(ctx context.Context) error
}

// PluginManager manages plugins
type PluginManager struct {
	plugins     map[string]Plugin
	hookManager *HookManager
	mu          sync.RWMutex
}

// NewPluginManager creates a new plugin manager
func NewPluginManager(hookManager *HookManager) *PluginManager {
	return &PluginManager{
		plugins:     make(map[string]Plugin),
		hookManager: hookManager,
	}
}

// Register registers a plugin
func (m *PluginManager) Register(plugin Plugin) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	name := plugin.Name()
	if _, exists := m.plugins[name]; exists {
		return fmt.Errorf("plugin %s already registered", name)
	}
	
	// Initialize plugin
	if err := plugin.Initialize(context.Background()); err != nil {
		return fmt.Errorf("failed to initialize plugin %s: %w", name, err)
	}
	
	// Register plugin hooks
	if err := plugin.RegisterHooks(m.hookManager); err != nil {
		return fmt.Errorf("failed to register hooks for plugin %s: %w", name, err)
	}
	
	m.plugins[name] = plugin
	return nil
}

// Unregister unregisters a plugin
func (m *PluginManager) Unregister(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	plugin, exists := m.plugins[name]
	if !exists {
		return fmt.Errorf("plugin %s not found", name)
	}
	
	// Shutdown plugin
	if err := plugin.Shutdown(context.Background()); err != nil {
		return fmt.Errorf("failed to shutdown plugin %s: %w", name, err)
	}
	
	delete(m.plugins, name)
	return nil
}

// GetPlugin retrieves a plugin by name
func (m *PluginManager) GetPlugin(name string) (Plugin, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	plugin, exists := m.plugins[name]
	return plugin, exists
}

// ListPlugins returns a list of registered plugins
func (m *PluginManager) ListPlugins() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	names := make([]string, 0, len(m.plugins))
	for name := range m.plugins {
		names = append(names, name)
	}
	return names
}

// Interceptor allows modifying data at extension points
type Interceptor interface {
	// Intercept processes and potentially modifies data
	Intercept(ctx context.Context, data interface{}) (interface{}, error)
}

// InterceptorChain chains multiple interceptors
type InterceptorChain struct {
	interceptors []Interceptor
}

// NewInterceptorChain creates a new interceptor chain
func NewInterceptorChain(interceptors ...Interceptor) *InterceptorChain {
	return &InterceptorChain{
		interceptors: interceptors,
	}
}

// Process runs data through all interceptors in the chain
func (c *InterceptorChain) Process(ctx context.Context, data interface{}) (interface{}, error) {
	var err error
	for _, interceptor := range c.interceptors {
		data, err = interceptor.Intercept(ctx, data)
		if err != nil {
			return nil, err
		}
	}
	return data, nil
}

// ExtensionRegistry manages all extension points
type ExtensionRegistry struct {
	hookManager   *HookManager
	pluginManager *PluginManager
	interceptors  map[string]*InterceptorChain
	mu            sync.RWMutex
}

// NewExtensionRegistry creates a new extension registry
func NewExtensionRegistry() *ExtensionRegistry {
	hookManager := NewHookManager()
	return &ExtensionRegistry{
		hookManager:   hookManager,
		pluginManager: NewPluginManager(hookManager),
		interceptors:  make(map[string]*InterceptorChain),
	}
}

// RegisterInterceptor registers an interceptor for a specific point
func (r *ExtensionRegistry) RegisterInterceptor(point string, interceptor Interceptor) {
	r.mu.Lock()
	defer r.mu.Unlock()
	
	if r.interceptors[point] == nil {
		r.interceptors[point] = NewInterceptorChain()
	}
	r.interceptors[point].interceptors = append(r.interceptors[point].interceptors, interceptor)
}

// ProcessInterceptors processes data through interceptors at a point
func (r *ExtensionRegistry) ProcessInterceptors(ctx context.Context, point string, data interface{}) (interface{}, error) {
	r.mu.RLock()
	chain, exists := r.interceptors[point]
	r.mu.RUnlock()
	
	if !exists {
		return data, nil
	}
	
	return chain.Process(ctx, data)
}

// GetHookManager returns the hook manager
func (r *ExtensionRegistry) GetHookManager() *HookManager {
	return r.hookManager
}

// GetPluginManager returns the plugin manager
func (r *ExtensionRegistry) GetPluginManager() *PluginManager {
	return r.pluginManager
}