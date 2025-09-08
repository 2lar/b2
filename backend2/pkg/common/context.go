package common

import (
	"context"
	"time"
)

// ContextKey represents a context key type
type ContextKey string

// Context keys
const (
	ContextKeyUserID    ContextKey = "user_id"
	ContextKeyRequestID ContextKey = "request_id"
	ContextKeyTraceID   ContextKey = "trace_id"
	ContextKeyStartTime ContextKey = "start_time"
	ContextKeyUserRoles ContextKey = "user_roles"
	ContextKeyTenantID  ContextKey = "tenant_id"
)

// WithUserID adds user ID to context
func WithUserID(ctx context.Context, userID string) context.Context {
	return context.WithValue(ctx, ContextKeyUserID, userID)
}

// GetUserID extracts user ID from context
func GetUserID(ctx context.Context) (string, bool) {
	userID, ok := ctx.Value(ContextKeyUserID).(string)
	return userID, ok
}

// WithRequestID adds request ID to context
func WithRequestID(ctx context.Context, requestID string) context.Context {
	return context.WithValue(ctx, ContextKeyRequestID, requestID)
}

// GetRequestID extracts request ID from context
func GetRequestID(ctx context.Context) (string, bool) {
	requestID, ok := ctx.Value(ContextKeyRequestID).(string)
	return requestID, ok
}

// WithTraceID adds trace ID to context
func WithTraceID(ctx context.Context, traceID string) context.Context {
	return context.WithValue(ctx, ContextKeyTraceID, traceID)
}

// GetTraceID extracts trace ID from context
func GetTraceID(ctx context.Context) (string, bool) {
	traceID, ok := ctx.Value(ContextKeyTraceID).(string)
	return traceID, ok
}

// WithStartTime adds start time to context
func WithStartTime(ctx context.Context, startTime time.Time) context.Context {
	return context.WithValue(ctx, ContextKeyStartTime, startTime)
}

// GetStartTime extracts start time from context
func GetStartTime(ctx context.Context) (time.Time, bool) {
	startTime, ok := ctx.Value(ContextKeyStartTime).(time.Time)
	return startTime, ok
}

// GetElapsedTime calculates elapsed time from start time in context
func GetElapsedTime(ctx context.Context) time.Duration {
	if startTime, ok := GetStartTime(ctx); ok {
		return time.Since(startTime)
	}
	return 0
}

// WithUserRoles adds user roles to context
func WithUserRoles(ctx context.Context, roles []string) context.Context {
	return context.WithValue(ctx, ContextKeyUserRoles, roles)
}

// GetUserRoles extracts user roles from context
func GetUserRoles(ctx context.Context) ([]string, bool) {
	roles, ok := ctx.Value(ContextKeyUserRoles).([]string)
	return roles, ok
}

// HasRole checks if user has a specific role
func HasRole(ctx context.Context, role string) bool {
	roles, ok := GetUserRoles(ctx)
	if !ok {
		return false
	}
	
	for _, r := range roles {
		if r == role {
			return true
		}
	}
	return false
}

// WithTenantID adds tenant ID to context
func WithTenantID(ctx context.Context, tenantID string) context.Context {
	return context.WithValue(ctx, ContextKeyTenantID, tenantID)
}

// GetTenantID extracts tenant ID from context
func GetTenantID(ctx context.Context) (string, bool) {
	tenantID, ok := ctx.Value(ContextKeyTenantID).(string)
	return tenantID, ok
}

// EnrichContext adds common metadata to context
func EnrichContext(ctx context.Context, userID, requestID string) context.Context {
	ctx = WithUserID(ctx, userID)
	ctx = WithRequestID(ctx, requestID)
	ctx = WithStartTime(ctx, time.Now())
	return ctx
}

// ContextMetadata contains all context metadata
type ContextMetadata struct {
	UserID    string        `json:"user_id,omitempty"`
	RequestID string        `json:"request_id,omitempty"`
	TraceID   string        `json:"trace_id,omitempty"`
	TenantID  string        `json:"tenant_id,omitempty"`
	Roles     []string      `json:"roles,omitempty"`
	Duration  time.Duration `json:"duration,omitempty"`
}

// ExtractMetadata extracts all metadata from context
func ExtractMetadata(ctx context.Context) ContextMetadata {
	meta := ContextMetadata{}
	
	if userID, ok := GetUserID(ctx); ok {
		meta.UserID = userID
	}
	if requestID, ok := GetRequestID(ctx); ok {
		meta.RequestID = requestID
	}
	if traceID, ok := GetTraceID(ctx); ok {
		meta.TraceID = traceID
	}
	if tenantID, ok := GetTenantID(ctx); ok {
		meta.TenantID = tenantID
	}
	if roles, ok := GetUserRoles(ctx); ok {
		meta.Roles = roles
	}
	meta.Duration = GetElapsedTime(ctx)
	
	return meta
}