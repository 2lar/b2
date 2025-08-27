// Package middleware provides HTTP middleware components for the Brain2 API.
// This package implements cross-cutting concerns like versioning, authentication,
// rate limiting, and request/response processing.
package middleware

import (
	"context"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Version constants define the supported API versions
const (
	// VersionV1 is the first stable API version
	VersionV1 = "1"
	
	// VersionV2 is the second API version (future)
	VersionV2 = "2"
	
	// DefaultVersion is the default API version when none is specified
	DefaultVersion = VersionV1
	
	// LatestVersion is the latest stable API version
	LatestVersion = VersionV1
	
	// VersionHeader is the custom header for API version
	VersionHeader = "X-API-Version"
	
	// SupportedVersionsHeader lists all supported versions
	SupportedVersionsHeader = "X-API-Supported-Versions"
	
	// DeprecatedHeader indicates if the version is deprecated
	DeprecatedHeader = "X-API-Deprecated"
	
	// SunsetHeader indicates when the version will be retired
	SunsetHeader = "X-API-Sunset"
)

// contextKey is a type for context keys
type contextKey string

const (
	// VersionContextKey is the context key for API version
	VersionContextKey contextKey = "api-version"
	
	// VersionMetadataKey stores version metadata in context
	VersionMetadataKey contextKey = "version-metadata"
)

// VersionMetadata contains information about the API version being used
type VersionMetadata struct {
	Version           string
	IsDeprecated      bool
	DeprecationDate   *time.Time
	SunsetDate        *time.Time
	DetectionMethod   string // How the version was detected (url, header, query, default)
	RequestedVersion  string // The original requested version
	SupportedVersions []string
}

// VersionConfig configures the versioning middleware
type VersionConfig struct {
	// SupportedVersions lists all supported API versions
	SupportedVersions []string
	
	// DefaultVersion is used when no version is specified
	DefaultVersion string
	
	// DeprecatedVersions maps versions to their deprecation info
	DeprecatedVersions map[string]DeprecationInfo
	
	// EnableVersionHeader enables version detection from X-API-Version header
	EnableVersionHeader bool
	
	// EnableAcceptHeader enables version detection from Accept header
	EnableAcceptHeader bool
	
	// EnableQueryParam enables version detection from ?version= query param
	EnableQueryParam bool
	
	// EnableURLPath enables version detection from URL path /api/v{version}/
	EnableURLPath bool
	
	// StrictMode rejects requests with invalid versions (vs falling back to default)
	StrictMode bool
	
	// MetricsEnabled enables version metrics collection
	MetricsEnabled bool
}

// DeprecationInfo contains deprecation details for a version
type DeprecationInfo struct {
	DeprecatedAt time.Time
	SunsetAt     time.Time
	Message      string
	MigrationURL string
}

// DefaultVersionConfig returns a sensible default configuration
func DefaultVersionConfig() VersionConfig {
	return VersionConfig{
		SupportedVersions:   []string{VersionV1},
		DefaultVersion:      DefaultVersion,
		DeprecatedVersions:  make(map[string]DeprecationInfo),
		EnableVersionHeader: true,
		EnableAcceptHeader:  true,
		EnableQueryParam:    true,
		EnableURLPath:       true,
		StrictMode:          false,
		MetricsEnabled:      true,
	}
}

// versionRegex matches version in URL path
var versionRegex = regexp.MustCompile(`/api/v(\d+)(?:/|$)`)

// acceptVersionRegex matches version in Accept header
var acceptVersionRegex = regexp.MustCompile(`application/vnd\.[\w\-]+\.v(\d+)\+json`)

// Versioning creates a middleware that handles API versioning
func Versioning(config VersionConfig) func(next http.Handler) http.Handler {
	// Validate configuration
	if len(config.SupportedVersions) == 0 {
		config.SupportedVersions = []string{VersionV1}
	}
	if config.DefaultVersion == "" {
		config.DefaultVersion = DefaultVersion
	}
	
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Detect version from request
			version, method := detectVersion(r, config)
			
			// Validate version
			if !isVersionSupported(version, config.SupportedVersions) {
				if config.StrictMode {
					writeVersionError(w, version, config.SupportedVersions)
					return
				}
				// Fall back to default version
				version = config.DefaultVersion
				method = "default-fallback"
			}
			
			// Create version metadata
			metadata := VersionMetadata{
				Version:           version,
				DetectionMethod:   method,
				RequestedVersion:  version,
				SupportedVersions: config.SupportedVersions,
			}
			
			// Check if version is deprecated
			if deprecation, exists := config.DeprecatedVersions[version]; exists {
				metadata.IsDeprecated = true
				metadata.DeprecationDate = &deprecation.DeprecatedAt
				metadata.SunsetDate = &deprecation.SunsetAt
				
				// Add deprecation headers
				w.Header().Set(DeprecatedHeader, "true")
				w.Header().Set(SunsetHeader, deprecation.SunsetAt.Format(time.RFC3339))
				if deprecation.Message != "" {
					w.Header().Set("X-API-Deprecation-Message", deprecation.Message)
				}
				if deprecation.MigrationURL != "" {
					w.Header().Set("X-API-Migration-URL", deprecation.MigrationURL)
				}
			}
			
			// Add version headers to response
			w.Header().Set(VersionHeader, version)
			w.Header().Set(SupportedVersionsHeader, strings.Join(config.SupportedVersions, ", "))
			
			// Add version to context
			ctx := r.Context()
			ctx = context.WithValue(ctx, VersionContextKey, version)
			ctx = context.WithValue(ctx, VersionMetadataKey, metadata)
			
			// Collect metrics if enabled
			if config.MetricsEnabled {
				collectVersionMetrics(version, method, metadata.IsDeprecated)
			}
			
			// Proceed with request
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// detectVersion detects the API version from the request
func detectVersion(r *http.Request, config VersionConfig) (string, string) {
	// 1. Check URL path (highest priority)
	if config.EnableURLPath {
		if matches := versionRegex.FindStringSubmatch(r.URL.Path); len(matches) > 1 {
			return matches[1], "url-path"
		}
	}
	
	// 2. Check Accept header
	if config.EnableAcceptHeader {
		accept := r.Header.Get("Accept")
		if accept != "" {
			if matches := acceptVersionRegex.FindStringSubmatch(accept); len(matches) > 1 {
				return matches[1], "accept-header"
			}
		}
	}
	
	// 3. Check custom version header
	if config.EnableVersionHeader {
		if version := r.Header.Get(VersionHeader); version != "" {
			return version, "version-header"
		}
	}
	
	// 4. Check query parameter (lowest priority)
	if config.EnableQueryParam {
		if version := r.URL.Query().Get("version"); version != "" {
			return version, "query-param"
		}
	}
	
	// 5. Use default version
	return config.DefaultVersion, "default"
}

// isVersionSupported checks if a version is in the supported list
func isVersionSupported(version string, supported []string) bool {
	for _, v := range supported {
		if v == version {
			return true
		}
	}
	return false
}

// writeVersionError writes an error response for unsupported version
func writeVersionError(w http.ResponseWriter, requested string, supported []string) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set(SupportedVersionsHeader, strings.Join(supported, ", "))
	w.WriteHeader(http.StatusBadRequest)
	
	message := fmt.Sprintf(`{"error":"Unsupported API version '%s'. Supported versions: %s"}`,
		requested, strings.Join(supported, ", "))
	w.Write([]byte(message))
}

// collectVersionMetrics collects metrics about API version usage
func collectVersionMetrics(version, method string, deprecated bool) {
	// This is a placeholder for metrics collection
	// In production, this would integrate with your metrics system
	// Example metrics to collect:
	// - api_requests_total{version="1", method="url-path"}
	// - api_deprecated_usage_total{version="1"}
	// - api_version_detection_method{method="accept-header"}
}

// GetVersion retrieves the API version from the request context
func GetVersion(ctx context.Context) string {
	if version, ok := ctx.Value(VersionContextKey).(string); ok {
		return version
	}
	return DefaultVersion
}

// GetVersionMetadata retrieves version metadata from the request context
func GetVersionMetadata(ctx context.Context) (*VersionMetadata, bool) {
	if metadata, ok := ctx.Value(VersionMetadataKey).(VersionMetadata); ok {
		return &metadata, true
	}
	return nil, false
}

// IsVersionAtLeast checks if the current version is at least the specified version
func IsVersionAtLeast(ctx context.Context, minVersion string) bool {
	currentVersion := GetVersion(ctx)
	current, err1 := strconv.Atoi(currentVersion)
	min, err2 := strconv.Atoi(minVersion)
	
	if err1 != nil || err2 != nil {
		return false
	}
	
	return current >= min
}

// IsVersionDeprecated checks if the current version is deprecated
func IsVersionDeprecated(ctx context.Context) bool {
	if metadata, ok := GetVersionMetadata(ctx); ok {
		return metadata.IsDeprecated
	}
	return false
}