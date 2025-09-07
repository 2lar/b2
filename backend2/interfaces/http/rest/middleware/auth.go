package middleware

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"backend2/pkg/auth"
	"go.uber.org/zap"
)

var (
	// Default JWT configuration for development
	// In production, these should come from secure configuration
	defaultJWTSecret = "development-secret-change-in-production"
	defaultIssuer    = "brain2-backend"
	defaultAudience  = []string{"brain2-api"}
)

// Authenticate creates an authentication middleware with proper JWT validation
func Authenticate() func(next http.Handler) http.Handler {
	// Create a default JWT validator for backwards compatibility
	jwtConfig := auth.JWTConfig{
		SigningMethod: "HS256",
		SecretKey:     defaultJWTSecret,
		Issuer:        defaultIssuer,
		Audience:      defaultAudience,
	}

	validator, err := auth.NewJWTValidator(jwtConfig)
	if err != nil {
		// Log error and return a middleware that always fails
		return func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				respondUnauthorized(w, "Authentication system error")
			})
		}
	}

	// Create rate limiters
	ipLimiter := auth.NewIPRateLimiter(100)     // 100 requests per minute per IP
	userLimiter := auth.NewUserRateLimiter(200) // 200 requests per minute per user

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Extract client IP for rate limiting
			clientIP := getClientIP(r)
			
			// Apply IP rate limiting
			allowed, _ := ipLimiter.Allow(r.Context(), clientIP)
			if !allowed {
				respondWithError(w, http.StatusTooManyRequests, "Rate limit exceeded")
				return
			}

			// Get Authorization header
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				// Check lowercase too
				authHeader = r.Header.Get("authorization")
			}
			if authHeader == "" {
				respondUnauthorized(w, "Missing authorization header")
				return
			}
			
			// Check for Bearer token
			parts := strings.Split(authHeader, " ")
			if len(parts) != 2 || (parts[0] != "Bearer" && parts[0] != "bearer") {
				respondUnauthorized(w, "Invalid authorization header format")
				return
			}
			
			token := parts[1]
			
			// Check for API Gateway pre-authorized request
			var claims *auth.Claims
			if token == "api-gateway-validated" && r.Header.Get("X-API-Gateway-Authorized") == "true" {
				// This request was already validated by API Gateway JWT authorizer
				// Create a default authenticated user context
				// In production, you might want to extract user info from other headers
				// or use a different authorization flow
				claims = &auth.Claims{
					UserID: "125deabf-b32e-4313-b893-4a3ddb416cc2", // TODO: Extract from API Gateway context
					Email:  "admin@test.com",
					Roles:  []string{"authenticated"},
				}
			} else if strings.HasPrefix(token, "lambda-authorized:") {
				// Extract user ID from Lambda-authorized token
				userID := strings.TrimPrefix(token, "lambda-authorized:")
				if userID == "" {
					respondUnauthorized(w, "Invalid Lambda authorization")
					return
				}
				
				// Create claims from Lambda context
				// The JWT authorizer has already validated, so we trust these headers
				claims = &auth.Claims{
					UserID: userID,
					Email:  r.Header.Get("X-User-Email"),
					Roles:  []string{r.Header.Get("X-User-Role")},
				}
				if claims.Roles[0] == "" {
					claims.Roles = []string{"authenticated"}
				}
			} else {
				// Validate JWT token normally
				var err error
				claims, err = validator.ValidateToken(token)
				if err != nil {
					switch err {
					case auth.ErrExpiredToken:
						respondUnauthorized(w, "Token has expired")
					case auth.ErrInvalidSignature:
						respondUnauthorized(w, "Invalid token signature")
					default:
						respondUnauthorized(w, "Invalid token")
					}
					return
				}
			}

			// Apply user rate limiting
			allowed, _ = userLimiter.Allow(r.Context(), claims.UserID)
			if !allowed {
				respondWithError(w, http.StatusTooManyRequests, "User rate limit exceeded")
				return
			}

			// Create user context
			userCtx := &auth.UserContext{
				UserID: claims.UserID,
				Email:  claims.Email,
				Roles:  claims.Roles,
			}
			
			// Add user context to request
			ctx := auth.SetUserInContext(r.Context(), userCtx)
			
			// Also add userID for backwards compatibility
			ctx = context.WithValue(ctx, "userID", claims.UserID)
			
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// AuthenticateWithConfig creates an authentication middleware with custom configuration
func AuthenticateWithConfig(validator *auth.JWTValidator, logger *zap.Logger) func(next http.Handler) http.Handler {
	// Create rate limiters
	ipLimiter := auth.NewIPRateLimiter(100)     // 100 requests per minute per IP
	userLimiter := auth.NewUserRateLimiter(200) // 200 requests per minute per user

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Extract client IP for rate limiting
			clientIP := getClientIP(r)
			
			// Apply IP rate limiting
			allowed, err := ipLimiter.Allow(r.Context(), clientIP)
			if err != nil {
				logger.Error("Rate limiter error", zap.Error(err))
				respondWithError(w, http.StatusInternalServerError, "Internal server error")
				return
			}
			if !allowed {
				respondWithError(w, http.StatusTooManyRequests, "Rate limit exceeded")
				return
			}

			// Extract token from multiple sources
			token := extractToken(r)
			if token == "" {
				respondUnauthorized(w, "Missing authentication token")
				return
			}

			// Validate JWT token
			claims, err := validator.ValidateToken(token)
			if err != nil {
				logger.Warn("Invalid token", 
					zap.Error(err),
					zap.String("ip", clientIP),
					zap.String("path", r.URL.Path),
				)
				
				switch err {
				case auth.ErrExpiredToken:
					respondUnauthorized(w, "Token has expired")
				case auth.ErrInvalidSignature:
					respondUnauthorized(w, "Invalid token signature")
				default:
					respondUnauthorized(w, "Invalid token")
				}
				return
			}

			// Apply user rate limiting
			allowed, err = userLimiter.Allow(r.Context(), claims.UserID)
			if err != nil {
				logger.Error("User rate limiter error", zap.Error(err))
				respondWithError(w, http.StatusInternalServerError, "Internal server error")
				return
			}
			if !allowed {
				respondWithError(w, http.StatusTooManyRequests, "User rate limit exceeded")
				return
			}

			// Create user context
			userCtx := &auth.UserContext{
				UserID: claims.UserID,
				Email:  claims.Email,
				Roles:  claims.Roles,
			}

			// Add user context to request
			ctx := auth.SetUserInContext(r.Context(), userCtx)
			
			// Also add userID for backwards compatibility
			ctx = context.WithValue(ctx, "userID", claims.UserID)
			
			// Log successful authentication
			logger.Debug("Request authenticated",
				zap.String("user_id", claims.UserID),
				zap.String("path", r.URL.Path),
				zap.String("method", r.Method),
			)

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// extractToken extracts the JWT token from multiple sources
func extractToken(r *http.Request) string {
	// Check Authorization header
	authHeader := r.Header.Get("Authorization")
	if authHeader != "" {
		// Bearer token
		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) == 2 && strings.ToLower(parts[0]) == "bearer" {
			return parts[1]
		}
		// Token without Bearer prefix
		return authHeader
	}

	// Check cookie
	if cookie, err := r.Cookie("auth_token"); err == nil {
		return cookie.Value
	}

	// Check query parameter (not recommended for production)
	return r.URL.Query().Get("token")
}

// getClientIP extracts the client IP address
func getClientIP(r *http.Request) string {
	// Check X-Forwarded-For header
	xff := r.Header.Get("X-Forwarded-For")
	if xff != "" {
		parts := strings.Split(xff, ",")
		return strings.TrimSpace(parts[0])
	}

	// Check X-Real-IP header
	xri := r.Header.Get("X-Real-IP")
	if xri != "" {
		return xri
	}

	// Fall back to RemoteAddr
	addr := r.RemoteAddr
	if idx := strings.LastIndex(addr, ":"); idx != -1 {
		return addr[:idx]
	}
	return addr
}

// respondUnauthorized sends an unauthorized response
func respondUnauthorized(w http.ResponseWriter, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusUnauthorized)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"error":   true,
		"message": message,
		"code":    http.StatusUnauthorized,
	})
}

// respondWithError sends an error response with a specific status code
func respondWithError(w http.ResponseWriter, code int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"error":   true,
		"message": message,
		"code":    code,
	})
}

// RequireRole creates middleware that requires specific roles
func RequireRole(roles ...string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			user, err := auth.GetUserFromContext(r.Context())
			if err != nil {
				respondUnauthorized(w, "Unauthorized")
				return
			}

			// Check if user has any of the required roles
			hasRole := false
			for _, requiredRole := range roles {
				for _, userRole := range user.Roles {
					if userRole == requiredRole {
						hasRole = true
						break
					}
				}
				if hasRole {
					break
				}
			}

			if !hasRole {
				respondWithError(w, http.StatusForbidden, "Insufficient permissions")
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// TokenRefreshMiddleware handles token refresh
type TokenRefreshMiddleware struct {
	generator *auth.JWTGenerator
	validator *auth.JWTValidator
}

// NewTokenRefreshMiddleware creates a new token refresh middleware
func NewTokenRefreshMiddleware(secretKey string) (*TokenRefreshMiddleware, error) {
	genConfig := auth.JWTGeneratorConfig{
		SigningMethod: "HS256",
		SecretKey:     secretKey,
		Issuer:        defaultIssuer,
		Audience:      defaultAudience,
		ExpiryTime:    24 * time.Hour,
	}

	generator, err := auth.NewJWTGenerator(genConfig)
	if err != nil {
		return nil, err
	}

	valConfig := auth.JWTConfig{
		SigningMethod: "HS256",
		SecretKey:     secretKey,
		Issuer:        defaultIssuer,
		Audience:      defaultAudience,
	}

	validator, err := auth.NewJWTValidator(valConfig)
	if err != nil {
		return nil, err
	}

	return &TokenRefreshMiddleware{
		generator: generator,
		validator: validator,
	}, nil
}

// RefreshToken handles token refresh requests
func (m *TokenRefreshMiddleware) RefreshToken(w http.ResponseWriter, r *http.Request) {
	// Extract current token
	token := extractToken(r)
	if token == "" {
		respondUnauthorized(w, "Missing token")
		return
	}

	// Validate current token (even if expired, we check other claims)
	claims, err := m.validator.ValidateToken(token)
	if err != nil && err != auth.ErrExpiredToken {
		respondUnauthorized(w, "Invalid token")
		return
	}

	// Generate new token
	newToken, err := m.generator.GenerateToken(claims.UserID, claims.Email, claims.Roles)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to generate token")
		return
	}

	// Send new token
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"token":      newToken,
		"expires_in": 86400, // 24 hours in seconds
	})
}