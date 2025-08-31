// Package middleware provides security middleware for HTTP handlers.
package middleware

import (
	"net/http"
	"strings"
	
	"brain2-backend/internal/config"
	"brain2-backend/pkg/api"
	"go.uber.org/zap"
)

// SecurityMiddleware provides comprehensive security features for HTTP handlers.
type SecurityMiddleware struct {
	config *config.Security
	logger *zap.Logger
}

// NewSecurityMiddleware creates a new security middleware instance.
func NewSecurityMiddleware(cfg *config.Security, logger *zap.Logger) *SecurityMiddleware {
	return &SecurityMiddleware{
		config: cfg,
		logger: logger,
	}
}

// SecurityHeaders adds security headers to responses.
func (s *SecurityMiddleware) SecurityHeaders(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if s.config.SecureHeaders {
			// Add security headers
			w.Header().Set("X-Content-Type-Options", "nosniff")
			w.Header().Set("X-Frame-Options", "DENY")
			w.Header().Set("X-XSS-Protection", "1; mode=block")
			w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
			w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self' 'unsafe-inline'; style-src 'self' 'unsafe-inline'")
			
			// Add HSTS for HTTPS connections
			if r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https" {
				w.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
			}
		}
		
		next(w, r)
	}
}

// RequestSizeLimit enforces maximum request body size.
func (s *SecurityMiddleware) RequestSizeLimit(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Check Content-Length header
		if r.ContentLength > 0 && r.ContentLength > s.config.MaxRequestBodySize {
			s.logger.Warn("Request body too large",
				zap.String("path", r.URL.Path),
				zap.Int64("size", r.ContentLength),
				zap.Int64("max", s.config.MaxRequestBodySize),
			)
			api.Error(w, http.StatusRequestEntityTooLarge, "Request body too large")
			return
		}
		
		// Limit the reader to prevent memory exhaustion
		r.Body = http.MaxBytesReader(w, r.Body, s.config.MaxRequestBodySize)
		
		next(w, r)
	}
}

// CSRFProtection provides CSRF protection for state-changing operations.
func (s *SecurityMiddleware) CSRFProtection(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !s.config.EnableCSRF {
			next(w, r)
			return
		}
		
		// Skip CSRF check for safe methods
		if r.Method == http.MethodGet || r.Method == http.MethodHead || r.Method == http.MethodOptions {
			next(w, r)
			return
		}
		
		// Check for CSRF token in header or form
		token := r.Header.Get("X-CSRF-Token")
		if token == "" {
			token = r.FormValue("csrf_token")
		}
		
		if token == "" {
			s.logger.Warn("Missing CSRF token",
				zap.String("path", r.URL.Path),
				zap.String("method", r.Method),
			)
			api.Error(w, http.StatusForbidden, "CSRF token required")
			return
		}
		
		// Validate CSRF token (simplified - in production use a proper CSRF library)
		// This is a placeholder - implement proper CSRF validation
		if !s.validateCSRFToken(token) {
			s.logger.Warn("Invalid CSRF token",
				zap.String("path", r.URL.Path),
				zap.String("method", r.Method),
			)
			api.Error(w, http.StatusForbidden, "Invalid CSRF token")
			return
		}
		
		next(w, r)
	}
}

// validateCSRFToken validates a CSRF token.
// This is a simplified implementation - use a proper CSRF library in production.
func (s *SecurityMiddleware) validateCSRFToken(token string) bool {
	// Placeholder validation - implement proper CSRF token validation
	// In production, use a library like gorilla/csrf or similar
	return len(token) >= s.config.CSRFTokenLength
}

// XSSProtection sanitizes user input to prevent XSS attacks.
func (s *SecurityMiddleware) XSSProtection(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Sanitize query parameters
		q := r.URL.Query()
		for key, values := range q {
			for i, value := range values {
				q[key][i] = sanitizeInput(value)
			}
		}
		r.URL.RawQuery = q.Encode()
		
		next(w, r)
	}
}

// sanitizeInput performs basic input sanitization.
// For production, use a proper HTML sanitization library.
func sanitizeInput(input string) string {
	// Basic HTML entity escaping
	replacer := strings.NewReplacer(
		"<", "&lt;",
		">", "&gt;",
		"\"", "&quot;",
		"'", "&#39;",
		"&", "&amp;",
	)
	return replacer.Replace(input)
}

// CORSMiddleware handles Cross-Origin Resource Sharing.
func (s *SecurityMiddleware) CORSMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		
		// Check if origin is allowed
		if s.isOriginAllowed(origin) {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Allow-Credentials", "true")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Accept, Content-Type, Content-Length, Accept-Encoding, Authorization, X-CSRF-Token")
			w.Header().Set("Access-Control-Max-Age", "86400") // 24 hours
		}
		
		// Handle preflight requests
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		
		next(w, r)
	}
}

// isOriginAllowed checks if an origin is in the allowed list.
func (s *SecurityMiddleware) isOriginAllowed(origin string) bool {
	if origin == "" {
		return false
	}
	
	for _, allowed := range s.config.AllowedOrigins {
		if allowed == "*" || allowed == origin {
			return true
		}
		
		// Support wildcard subdomains
		if strings.HasPrefix(allowed, "*.") {
			domain := strings.TrimPrefix(allowed, "*")
			if strings.HasSuffix(origin, domain) {
				return true
			}
		}
	}
	
	return false
}

// BuildSecurityPipeline creates a complete security middleware pipeline.
func BuildSecurityPipeline(cfg *config.Security, logger *zap.Logger) *Pipeline {
	sm := NewSecurityMiddleware(cfg, logger)
	pipeline := NewPipeline(logger)
	
	// Add security middleware in priority order
	pipeline.AddFunc("SecurityHeaders", 1, sm.SecurityHeaders)
	pipeline.AddFunc("CORS", 2, sm.CORSMiddleware)
	pipeline.AddFunc("RequestSizeLimit", 3, sm.RequestSizeLimit)
	
	if cfg.EnableCSRF {
		pipeline.AddFunc("CSRFProtection", 4, sm.CSRFProtection)
	}
	
	pipeline.AddFunc("XSSProtection", 5, sm.XSSProtection)
	
	return pipeline
}