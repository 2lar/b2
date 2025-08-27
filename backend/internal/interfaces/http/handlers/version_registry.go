// Package handlers provides version-aware handler registration and routing.
package handlers

import (
	"fmt"
	"net/http"
	"sync"
	
	"brain2-backend/internal/interfaces/http/middleware"
)

// HandlerFunc represents a versioned handler function
type HandlerFunc func(version string, w http.ResponseWriter, r *http.Request)

// VersionedHandler interface for handlers that support multiple versions
type VersionedHandler interface {
	// SupportedVersions returns the versions this handler supports
	SupportedVersions() []string
	
	// HandleRequest handles the request for a specific version
	HandleRequest(version string, w http.ResponseWriter, r *http.Request)
}

// VersionRegistry manages version-specific handlers
type VersionRegistry struct {
	mu            sync.RWMutex
	handlers      map[string]map[string]http.HandlerFunc // version -> route -> handler
	defaultVersion string
	routes        map[string][]string // route -> supported versions
}

// NewVersionRegistry creates a new version registry
func NewVersionRegistry(defaultVersion string) *VersionRegistry {
	return &VersionRegistry{
		handlers:       make(map[string]map[string]http.HandlerFunc),
		defaultVersion: defaultVersion,
		routes:         make(map[string][]string),
	}
}

// RegisterHandler registers a handler for a specific version and route
func (vr *VersionRegistry) RegisterHandler(version, route string, handler http.HandlerFunc) error {
	vr.mu.Lock()
	defer vr.mu.Unlock()
	
	// Initialize version map if not exists
	if vr.handlers[version] == nil {
		vr.handlers[version] = make(map[string]http.HandlerFunc)
	}
	
	// Check for duplicate registration
	if _, exists := vr.handlers[version][route]; exists {
		return fmt.Errorf("handler already registered for version %s, route %s", version, route)
	}
	
	// Register the handler
	vr.handlers[version][route] = handler
	
	// Track route versions
	if vr.routes[route] == nil {
		vr.routes[route] = []string{}
	}
	vr.routes[route] = append(vr.routes[route], version)
	
	return nil
}

// RegisterVersionedHandler registers a versioned handler for multiple versions
func (vr *VersionRegistry) RegisterVersionedHandler(route string, handler VersionedHandler) error {
	for _, version := range handler.SupportedVersions() {
		handlerFunc := func(w http.ResponseWriter, r *http.Request) {
			// Get version from context
			version := middleware.GetVersion(r.Context())
			handler.HandleRequest(version, w, r)
		}
		
		if err := vr.RegisterHandler(version, route, handlerFunc); err != nil {
			return err
		}
	}
	return nil
}

// GetHandler retrieves a handler for a specific version and route
func (vr *VersionRegistry) GetHandler(version, route string) (http.HandlerFunc, bool) {
	vr.mu.RLock()
	defer vr.mu.RUnlock()
	
	if handlers, versionExists := vr.handlers[version]; versionExists {
		if handler, routeExists := handlers[route]; routeExists {
			return handler, true
		}
	}
	
	// Try to fall back to default version
	if version != vr.defaultVersion {
		return vr.GetHandler(vr.defaultVersion, route)
	}
	
	return nil, false
}

// GetSupportedVersions returns all versions that support a specific route
func (vr *VersionRegistry) GetSupportedVersions(route string) []string {
	vr.mu.RLock()
	defer vr.mu.RUnlock()
	
	if versions, exists := vr.routes[route]; exists {
		// Return a copy to prevent external modification
		result := make([]string, len(versions))
		copy(result, versions)
		return result
	}
	
	return nil
}

// CreateVersionAwareHandler creates a handler that automatically routes based on version
func (vr *VersionRegistry) CreateVersionAwareHandler(route string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Get version from context
		version := middleware.GetVersion(r.Context())
		
		// Get appropriate handler
		handler, exists := vr.GetHandler(version, route)
		if !exists {
			// No handler found for this version/route combination
			http.Error(w, fmt.Sprintf("Route %s not available in API version %s", route, version), http.StatusNotFound)
			return
		}
		
		// Execute the handler
		handler(w, r)
	}
}

// RouteBuilder provides a fluent interface for registering versioned routes
type RouteBuilder struct {
	registry *VersionRegistry
	route    string
	versions []string
}

// NewRouteBuilder creates a new route builder
func (vr *VersionRegistry) NewRouteBuilder(route string) *RouteBuilder {
	return &RouteBuilder{
		registry: vr,
		route:    route,
		versions: []string{},
	}
}

// ForVersion adds a version to the route builder
func (rb *RouteBuilder) ForVersion(version string) *RouteBuilder {
	rb.versions = append(rb.versions, version)
	return rb
}

// ForVersions adds multiple versions to the route builder
func (rb *RouteBuilder) ForVersions(versions ...string) *RouteBuilder {
	rb.versions = append(rb.versions, versions...)
	return rb
}

// Handle registers the handler for all specified versions
func (rb *RouteBuilder) Handle(handler http.HandlerFunc) error {
	for _, version := range rb.versions {
		if err := rb.registry.RegisterHandler(version, rb.route, handler); err != nil {
			return err
		}
	}
	return nil
}

// HandleVersioned registers a versioned handler
func (rb *RouteBuilder) HandleVersioned(handler VersionedHandler) error {
	return rb.registry.RegisterVersionedHandler(rb.route, handler)
}

// VersionAdapter adapts existing handlers to be version-aware
type VersionAdapter struct {
	v1Handler http.HandlerFunc
	v2Handler http.HandlerFunc // For future use
}

// NewVersionAdapter creates a new version adapter
func NewVersionAdapter(v1Handler http.HandlerFunc) *VersionAdapter {
	return &VersionAdapter{
		v1Handler: v1Handler,
	}
}

// WithV2Handler sets the v2 handler (for future use)
func (va *VersionAdapter) WithV2Handler(handler http.HandlerFunc) *VersionAdapter {
	va.v2Handler = handler
	return va
}

// SupportedVersions returns the supported versions
func (va *VersionAdapter) SupportedVersions() []string {
	versions := []string{"1"}
	if va.v2Handler != nil {
		versions = append(versions, "2")
	}
	return versions
}

// HandleRequest handles the request based on version
func (va *VersionAdapter) HandleRequest(version string, w http.ResponseWriter, r *http.Request) {
	switch version {
	case "1":
		if va.v1Handler != nil {
			va.v1Handler(w, r)
		} else {
			http.Error(w, "Handler not implemented for version 1", http.StatusNotImplemented)
		}
	case "2":
		if va.v2Handler != nil {
			va.v2Handler(w, r)
		} else {
			// Fall back to v1 handler if v2 not implemented
			if va.v1Handler != nil {
				va.v1Handler(w, r)
			} else {
				http.Error(w, "Handler not implemented for version 2", http.StatusNotImplemented)
			}
		}
	default:
		http.Error(w, fmt.Sprintf("Unsupported version: %s", version), http.StatusBadRequest)
	}
}

// GlobalRegistry is the global version registry instance
var GlobalRegistry = NewVersionRegistry("1")

// RegisterV1Handler is a convenience function to register a v1 handler
func RegisterV1Handler(route string, handler http.HandlerFunc) error {
	return GlobalRegistry.RegisterHandler("1", route, handler)
}

// RegisterV2Handler is a convenience function to register a v2 handler (for future use)
func RegisterV2Handler(route string, handler http.HandlerFunc) error {
	return GlobalRegistry.RegisterHandler("2", route, handler)
}

// GetVersionedHandler is a convenience function to get a version-aware handler
func GetVersionedHandler(route string) http.HandlerFunc {
	return GlobalRegistry.CreateVersionAwareHandler(route)
}