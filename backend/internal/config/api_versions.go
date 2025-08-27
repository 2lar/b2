// Package config provides API version configuration for the Brain2 backend.
package config

import (
	"time"
)

// APIVersion represents a single API version configuration
type APIVersion struct {
	Version      string
	ReleaseDate  time.Time
	Deprecated   bool
	DeprecatedAt *time.Time
	SunsetDate   *time.Time
	Features     []string
	Changes      []string
}

// APIVersionConfig holds the complete API versioning configuration
type APIVersionConfig struct {
	// CurrentVersion is the current stable API version
	CurrentVersion string
	
	// DefaultVersion is the version used when none is specified
	DefaultVersion string
	
	// Versions contains configuration for all API versions
	Versions map[string]APIVersion
	
	// VersionFeatures maps features to the minimum version required
	VersionFeatures map[string]string
}

// GetAPIVersionConfig returns the API version configuration
func GetAPIVersionConfig() APIVersionConfig {
	return APIVersionConfig{
		CurrentVersion: "1",
		DefaultVersion: "1",
		
		Versions: map[string]APIVersion{
			"1": {
				Version:      "1",
				ReleaseDate:  time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
				Deprecated:   false,
				DeprecatedAt: nil,
				SunsetDate:   nil,
				Features: []string{
					"node-management",
					"edge-management",
					"category-management",
					"graph-visualization",
					"bulk-operations",
					"idempotency",
					"websocket-support",
				},
				Changes: []string{
					"Initial API release",
					"Full node and edge management",
					"Category hierarchy support",
					"Real-time updates via WebSocket",
				},
			},
			// Future version placeholder
			"2": {
				Version:      "2",
				ReleaseDate:  time.Time{}, // Not released yet
				Deprecated:   false,
				DeprecatedAt: nil,
				SunsetDate:   nil,
				Features: []string{
					// Future v2 features (when implemented)
					"advanced-filtering",
					"field-selection",
					"batch-operations-enhanced",
					"graphql-style-queries",
					"streaming-responses",
					"partial-updates",
					"json-patch-support",
				},
				Changes: []string{
					// Planned v2 changes (for documentation)
					"Enhanced filtering with complex queries",
					"Field selection for optimized responses",
					"Improved batch operation performance",
					"Support for partial resource updates",
					"JSON Patch support for precise updates",
				},
			},
		},
		
		VersionFeatures: map[string]string{
			// Map features to minimum required version
			"basic-operations":         "1",
			"node-management":          "1",
			"edge-management":          "1",
			"category-management":      "1",
			"bulk-operations":          "1",
			"websocket":                "1",
			"idempotency":              "1",
			
			// Future v2 features
			"advanced-filtering":       "2",
			"field-selection":          "2",
			"json-patch":               "2",
			"streaming-responses":      "2",
			"graphql-style-queries":    "2",
		},
	}
}

// IsVersionSupported checks if a version is supported
func (c APIVersionConfig) IsVersionSupported(version string) bool {
	_, exists := c.Versions[version]
	return exists
}

// GetSupportedVersions returns a list of all supported versions
func (c APIVersionConfig) GetSupportedVersions() []string {
	versions := make([]string, 0, len(c.Versions))
	for v := range c.Versions {
		versions = append(versions, v)
	}
	return versions
}

// IsFeatureAvailable checks if a feature is available in a given version
func (c APIVersionConfig) IsFeatureAvailable(feature, version string) bool {
	requiredVersion, exists := c.VersionFeatures[feature]
	if !exists {
		return false // Unknown feature
	}
	
	// Simple numeric comparison (works for single digit versions)
	// For more complex versioning, use a proper version comparison library
	return version >= requiredVersion
}

// GetVersionFeatures returns all features available in a specific version
func (c APIVersionConfig) GetVersionFeatures(version string) []string {
	v, exists := c.Versions[version]
	if !exists {
		return nil
	}
	return v.Features
}

// GetVersionChanges returns the changes introduced in a specific version
func (c APIVersionConfig) GetVersionChanges(version string) []string {
	v, exists := c.Versions[version]
	if !exists {
		return nil
	}
	return v.Changes
}

// IsVersionDeprecated checks if a version is deprecated
func (c APIVersionConfig) IsVersionDeprecated(version string) bool {
	v, exists := c.Versions[version]
	if !exists {
		return false
	}
	return v.Deprecated
}

// GetDeprecationInfo returns deprecation information for a version
func (c APIVersionConfig) GetDeprecationInfo(version string) (deprecated bool, deprecatedAt *time.Time, sunsetDate *time.Time) {
	v, exists := c.Versions[version]
	if !exists {
		return false, nil, nil
	}
	return v.Deprecated, v.DeprecatedAt, v.SunsetDate
}

// VersionFeatureFlags provides feature flags based on API version
type VersionFeatureFlags struct {
	// EnableBatchOperations enables batch operations for multiple resources
	EnableBatchOperations bool
	
	// EnableFieldFiltering allows clients to specify which fields to return
	EnableFieldFiltering bool
	
	// EnableAdvancedQuerying enables complex query parameters
	EnableAdvancedQuerying bool
	
	// EnablePartialUpdates allows partial resource updates (PATCH)
	EnablePartialUpdates bool
	
	// EnableStreamingResponses enables streaming for large responses
	EnableStreamingResponses bool
	
	// MaxBatchSize defines the maximum batch size for operations
	MaxBatchSize int
	
	// MaxPageSize defines the maximum page size for list operations
	MaxPageSize int
	
	// DefaultPageSize defines the default page size
	DefaultPageSize int
}

// GetFeatureFlags returns feature flags for a specific API version
func GetFeatureFlags(version string) VersionFeatureFlags {
	switch version {
	case "1":
		return VersionFeatureFlags{
			EnableBatchOperations:    true,
			EnableFieldFiltering:     false,
			EnableAdvancedQuerying:   false,
			EnablePartialUpdates:     false,
			EnableStreamingResponses: false,
			MaxBatchSize:             100,
			MaxPageSize:              100,
			DefaultPageSize:          20,
		}
	case "2":
		// Future v2 feature flags
		return VersionFeatureFlags{
			EnableBatchOperations:    true,
			EnableFieldFiltering:     true,
			EnableAdvancedQuerying:   true,
			EnablePartialUpdates:     true,
			EnableStreamingResponses: true,
			MaxBatchSize:             500,
			MaxPageSize:              200,
			DefaultPageSize:          50,
		}
	default:
		// Default to v1 feature flags
		return GetFeatureFlags("1")
	}
}