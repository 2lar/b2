package auth

import (
	"context"
	"crypto/rsa"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

var (
	ErrInvalidToken     = errors.New("invalid token")
	ErrExpiredToken     = errors.New("token has expired")
	ErrInvalidSignature = errors.New("invalid token signature")
	ErrMissingToken     = errors.New("missing authentication token")
	ErrInvalidClaims    = errors.New("invalid token claims")
)

// Claims represents the JWT claims
type Claims struct {
	UserID   string   `json:"sub"`
	Email    string   `json:"email"`
	Roles    []string `json:"roles"`
	Scope    string   `json:"scope"`
	ClientID string   `json:"client_id,omitempty"`
	jwt.RegisteredClaims
}

// JWTValidator handles JWT validation
type JWTValidator struct {
	publicKey     *rsa.PublicKey
	secretKey     []byte
	signingMethod jwt.SigningMethod
	issuer        string
	audience      []string
}

// NewJWTValidator creates a new JWT validator
func NewJWTValidator(config JWTConfig) (*JWTValidator, error) {
	validator := &JWTValidator{
		issuer:   config.Issuer,
		audience: config.Audience,
	}

	switch config.SigningMethod {
	case "RS256":
		validator.signingMethod = jwt.SigningMethodRS256
		if config.PublicKey == "" {
			return nil, errors.New("public key required for RS256")
		}
		key, err := jwt.ParseRSAPublicKeyFromPEM([]byte(config.PublicKey))
		if err != nil {
			return nil, fmt.Errorf("failed to parse public key: %w", err)
		}
		validator.publicKey = key
	case "HS256":
		validator.signingMethod = jwt.SigningMethodHS256
		if config.SecretKey == "" {
			return nil, errors.New("secret key required for HS256")
		}
		validator.secretKey = []byte(config.SecretKey)
	default:
		return nil, fmt.Errorf("unsupported signing method: %s", config.SigningMethod)
	}

	return validator, nil
}

// ValidateToken validates a JWT token and returns the claims
func (v *JWTValidator) ValidateToken(tokenString string) (*Claims, error) {
	// Remove "Bearer " prefix if present
	tokenString = strings.TrimPrefix(tokenString, "Bearer ")
	tokenString = strings.TrimSpace(tokenString)

	if tokenString == "" {
		return nil, ErrMissingToken
	}

	// Parse token with claims
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		// Verify signing method
		if token.Method != v.signingMethod {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Method)
		}

		// Return appropriate key based on method
		switch v.signingMethod {
		case jwt.SigningMethodRS256:
			return v.publicKey, nil
		case jwt.SigningMethodHS256:
			return v.secretKey, nil
		default:
			return nil, errors.New("unknown signing method")
		}
	})

	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return nil, ErrExpiredToken
		}
		if errors.Is(err, jwt.ErrSignatureInvalid) {
			return nil, ErrInvalidSignature
		}
		return nil, fmt.Errorf("%w: %v", ErrInvalidToken, err)
	}

	// Extract and validate claims
	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, ErrInvalidClaims
	}

	// Validate issuer
	if v.issuer != "" && claims.Issuer != v.issuer {
		return nil, fmt.Errorf("%w: invalid issuer", ErrInvalidClaims)
	}

	// Validate audience
	if len(v.audience) > 0 {
		validAudience := false
		for _, aud := range v.audience {
			if claims.Audience != nil && contains(claims.Audience, aud) {
				validAudience = true
				break
			}
		}
		if !validAudience {
			return nil, fmt.Errorf("%w: invalid audience", ErrInvalidClaims)
		}
	}

	// Additional validation
	if claims.UserID == "" {
		return nil, fmt.Errorf("%w: missing user ID", ErrInvalidClaims)
	}

	return claims, nil
}

// JWTConfig holds JWT configuration
type JWTConfig struct {
	SigningMethod string   // RS256 or HS256
	PublicKey     string   // For RS256
	SecretKey     string   // For HS256
	Issuer        string   // Expected issuer
	Audience      []string // Expected audience
}

// JWTGenerator generates JWT tokens
type JWTGenerator struct {
	privateKey    *rsa.PrivateKey
	secretKey     []byte
	signingMethod jwt.SigningMethod
	issuer        string
	audience      []string
	expiryTime    time.Duration
}

// NewJWTGenerator creates a new JWT generator
func NewJWTGenerator(config JWTGeneratorConfig) (*JWTGenerator, error) {
	generator := &JWTGenerator{
		issuer:     config.Issuer,
		audience:   config.Audience,
		expiryTime: config.ExpiryTime,
	}

	switch config.SigningMethod {
	case "RS256":
		generator.signingMethod = jwt.SigningMethodRS256
		if config.PrivateKey == "" {
			return nil, errors.New("private key required for RS256")
		}
		key, err := jwt.ParseRSAPrivateKeyFromPEM([]byte(config.PrivateKey))
		if err != nil {
			return nil, fmt.Errorf("failed to parse private key: %w", err)
		}
		generator.privateKey = key
	case "HS256":
		generator.signingMethod = jwt.SigningMethodHS256
		if config.SecretKey == "" {
			return nil, errors.New("secret key required for HS256")
		}
		generator.secretKey = []byte(config.SecretKey)
	default:
		return nil, fmt.Errorf("unsupported signing method: %s", config.SigningMethod)
	}

	return generator, nil
}

// GenerateToken generates a new JWT token
func (g *JWTGenerator) GenerateToken(userID, email string, roles []string) (string, error) {
	now := time.Now()
	claims := &Claims{
		UserID: userID,
		Email:  email,
		Roles:  roles,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    g.issuer,
			Subject:   userID,
			Audience:  g.audience,
			ExpiresAt: jwt.NewNumericDate(now.Add(g.expiryTime)),
			NotBefore: jwt.NewNumericDate(now),
			IssuedAt:  jwt.NewNumericDate(now),
			ID:        generateJTI(),
		},
	}

	token := jwt.NewWithClaims(g.signingMethod, claims)

	// Sign token with appropriate key
	var key interface{}
	switch g.signingMethod {
	case jwt.SigningMethodRS256:
		key = g.privateKey
	case jwt.SigningMethodHS256:
		key = g.secretKey
	default:
		return "", errors.New("unknown signing method")
	}

	return token.SignedString(key)
}

// JWTGeneratorConfig holds JWT generator configuration
type JWTGeneratorConfig struct {
	SigningMethod string        // RS256 or HS256
	PrivateKey    string        // For RS256
	SecretKey     string        // For HS256
	Issuer        string        // Token issuer
	Audience      []string      // Token audience
	ExpiryTime    time.Duration // Token expiry duration
}

// UserContext represents user information from JWT
type UserContext struct {
	UserID   string
	Email    string
	Roles    []string
	ClientID string
}

// ContextKey for storing user context
type contextKey string

const UserContextKey contextKey = "user"

// GetUserFromContext extracts user from context
func GetUserFromContext(ctx context.Context) (*UserContext, error) {
	user, ok := ctx.Value(UserContextKey).(*UserContext)
	if !ok || user == nil {
		return nil, errors.New("user not found in context")
	}
	return user, nil
}

// SetUserInContext adds user to context
func SetUserInContext(ctx context.Context, user *UserContext) context.Context {
	return context.WithValue(ctx, UserContextKey, user)
}

// Helper functions

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

func generateJTI() string {
	// Generate a unique JWT ID
	return fmt.Sprintf("%d-%s", time.Now().UnixNano(), randomString(8))
}

func randomString(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[time.Now().UnixNano()%int64(len(charset))]
	}
	return string(b)
}

// JWTService provides JWT token generation and validation
type JWTService struct {
	validator *JWTValidator
	secretKey []byte
	issuer    string
	audience  []string
	ttl       time.Duration
}

// NewJWTService creates a new JWT service
func NewJWTService(secret string, issuer string, audience []string, ttl time.Duration) *JWTService {
	config := JWTConfig{
		SigningMethod: "HS256",
		SecretKey:     secret,
		Issuer:        issuer,
		Audience:      audience,
	}

	validator, _ := NewJWTValidator(config)

	return &JWTService{
		validator: validator,
		secretKey: []byte(secret),
		issuer:    issuer,
		audience:  audience,
		ttl:       ttl,
	}
}

// GenerateToken generates a new JWT token for a user
func (s *JWTService) GenerateToken(userID string, email string, roles []string) (string, error) {
	now := time.Now()
	claims := Claims{
		UserID: userID,
		Email:  email,
		Roles:  roles,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    s.issuer,
			Subject:   userID,
			Audience:  s.audience,
			ExpiresAt: jwt.NewNumericDate(now.Add(s.ttl)),
			NotBefore: jwt.NewNumericDate(now),
			IssuedAt:  jwt.NewNumericDate(now),
			ID:        randomString(16),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(s.secretKey)
}

// ValidateToken validates a JWT token and returns the claims
func (s *JWTService) ValidateToken(tokenString string) (jwt.MapClaims, error) {
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return s.secretKey, nil
	})

	if err != nil {
		return nil, err
	}

	if claims, ok := token.Claims.(jwt.MapClaims); ok && token.Valid {
		return claims, nil
	}

	return nil, ErrInvalidToken
}
