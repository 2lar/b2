package memory

import "context"

type contextKey string

const idempotencyKeyContext contextKey = "idempotency-key"

// WithIdempotencyKey adds an idempotency key to context
func WithIdempotencyKey(ctx context.Context, key string) context.Context {
	return context.WithValue(ctx, idempotencyKeyContext, key)
}

// GetIdempotencyKeyFromContext retrieves idempotency key from context
func GetIdempotencyKeyFromContext(ctx context.Context) string {
	if key, ok := ctx.Value(idempotencyKeyContext).(string); ok {
		return key
	}
	return ""
}