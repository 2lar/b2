// Package context provides context utilities for the Brain2 API,
// including version management and request metadata.
package context

import (
	"context"
	"fmt"
	"strconv"
)

// versionKey is the context key for API version
type versionKey struct{}

// versionMetadataKey is the context key for version metadata
type versionMetadataKey struct{}

// VersionMetadata contains detailed information about the API version
type VersionMetadata struct {
	Version         string
	Major           int
	Minor           int
	Patch           int
	IsDeprecated    bool
	DetectionMethod string // How the version was detected
	Features        map[string]bool
}

// WithVersion adds version information to the context
func WithVersion(ctx context.Context, version string) context.Context {
	return context.WithValue(ctx, versionKey{}, version)
}

// GetVersion retrieves the API version from context
func GetVersion(ctx context.Context) string {
	if version, ok := ctx.Value(versionKey{}).(string); ok {
		return version
	}
	return "1" // Default version
}

// WithVersionMetadata adds version metadata to the context
func WithVersionMetadata(ctx context.Context, metadata *VersionMetadata) context.Context {
	return context.WithValue(ctx, versionMetadataKey{}, metadata)
}

// GetVersionMetadata retrieves version metadata from context
func GetVersionMetadata(ctx context.Context) (*VersionMetadata, bool) {
	if metadata, ok := ctx.Value(versionMetadataKey{}).(*VersionMetadata); ok {
		return metadata, true
	}
	return nil, false
}

// IsVersionAtLeast checks if the current version is at least the specified version
func IsVersionAtLeast(ctx context.Context, minVersion string) bool {
	currentVersion := GetVersion(ctx)
	return CompareVersions(currentVersion, minVersion) >= 0
}

// IsVersionExactly checks if the current version matches exactly
func IsVersionExactly(ctx context.Context, version string) bool {
	return GetVersion(ctx) == version
}

// IsVersionBetween checks if the current version is between min and max (inclusive)
func IsVersionBetween(ctx context.Context, minVersion, maxVersion string) bool {
	currentVersion := GetVersion(ctx)
	return CompareVersions(currentVersion, minVersion) >= 0 && 
	       CompareVersions(currentVersion, maxVersion) <= 0
}

// CompareVersions compares two version strings
// Returns: -1 if v1 < v2, 0 if v1 == v2, 1 if v1 > v2
func CompareVersions(v1, v2 string) int {
	// Simple numeric comparison for now
	// In production, use a proper semver library
	n1, err1 := strconv.Atoi(v1)
	n2, err2 := strconv.Atoi(v2)
	
	if err1 != nil || err2 != nil {
		// Fall back to string comparison if not numeric
		if v1 < v2 {
			return -1
		} else if v1 > v2 {
			return 1
		}
		return 0
	}
	
	if n1 < n2 {
		return -1
	} else if n1 > n2 {
		return 1
	}
	return 0
}

// HasFeature checks if a specific feature is available in the current version
func HasFeature(ctx context.Context, feature string) bool {
	if metadata, ok := GetVersionMetadata(ctx); ok {
		if metadata.Features != nil {
			return metadata.Features[feature]
		}
	}
	
	// Fall back to version-based feature detection
	version := GetVersion(ctx)
	return isFeatureInVersion(feature, version)
}

// isFeatureInVersion determines if a feature is available in a version
func isFeatureInVersion(feature, version string) bool {
	// Define feature availability by version
	// This is a simplified example - in production, load from configuration
	featureVersions := map[string]string{
		"batch_operations":    "1",
		"websocket":           "1",
		"categories":          "1",
		"field_filtering":     "2",
		"advanced_queries":    "2",
		"streaming_responses": "2",
	}
	
	requiredVersion, exists := featureVersions[feature]
	if !exists {
		return false
	}
	
	return CompareVersions(version, requiredVersion) >= 0
}

// VersionGuard provides version-based execution control
type VersionGuard struct {
	ctx context.Context
}

// NewVersionGuard creates a new version guard for the context
func NewVersionGuard(ctx context.Context) *VersionGuard {
	return &VersionGuard{ctx: ctx}
}

// IfVersion executes a function only if the version matches
func (vg *VersionGuard) IfVersion(version string, fn func()) *VersionGuard {
	if IsVersionExactly(vg.ctx, version) {
		fn()
	}
	return vg
}

// IfVersionAtLeast executes a function if version is at least the specified
func (vg *VersionGuard) IfVersionAtLeast(minVersion string, fn func()) *VersionGuard {
	if IsVersionAtLeast(vg.ctx, minVersion) {
		fn()
	}
	return vg
}

// IfFeature executes a function if a feature is available
func (vg *VersionGuard) IfFeature(feature string, fn func()) *VersionGuard {
	if HasFeature(vg.ctx, feature) {
		fn()
	}
	return vg
}

// Otherwise executes a function if no previous conditions matched
func (vg *VersionGuard) Otherwise(fn func()) {
	// This would need state tracking in production
	// For now, it's a placeholder
	fn()
}

// FormatVersionedError creates a version-aware error message
func FormatVersionedError(ctx context.Context, baseError error) error {
	version := GetVersion(ctx)
	return fmt.Errorf("API v%s error: %w", version, baseError)
}

// GetVersionedConfig returns configuration based on API version
func GetVersionedConfig(ctx context.Context, configKey string) interface{} {
	version := GetVersion(ctx)
	
	// Example configuration by version
	// In production, load from a configuration service
	configs := map[string]map[string]interface{}{
		"1": {
			"max_batch_size":   100,
			"max_page_size":    100,
			"default_page_size": 20,
		},
		"2": {
			"max_batch_size":   500,
			"max_page_size":    200,
			"default_page_size": 50,
		},
	}
	
	if versionConfig, exists := configs[version]; exists {
		if value, exists := versionConfig[configKey]; exists {
			return value
		}
	}
	
	return nil
}