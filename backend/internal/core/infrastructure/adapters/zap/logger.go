// Package zap provides a zap logger adapter for the ports.Logger interface
package zap

import (
	"context"

	"brain2-backend/internal/core/application/ports"
	
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// LoggerAdapter adapts zap.Logger to implement ports.Logger
type LoggerAdapter struct {
	logger *zap.Logger
}

// NewLoggerAdapter creates a new zap logger adapter
func NewLoggerAdapter(logger *zap.Logger) *LoggerAdapter {
	if logger == nil {
		// Create a default production logger if none provided
		logger, _ = zap.NewProduction()
	}
	return &LoggerAdapter{
		logger: logger,
	}
}

// Debug logs a debug message
func (l *LoggerAdapter) Debug(msg string, fields ...ports.Field) {
	l.logger.Debug(msg, l.convertFields(fields)...)
}

// Info logs an info message
func (l *LoggerAdapter) Info(msg string, fields ...ports.Field) {
	l.logger.Info(msg, l.convertFields(fields)...)
}

// Warn logs a warning message
func (l *LoggerAdapter) Warn(msg string, fields ...ports.Field) {
	l.logger.Warn(msg, l.convertFields(fields)...)
}

// Error logs an error message
func (l *LoggerAdapter) Error(msg string, err error, fields ...ports.Field) {
	zapFields := l.convertFields(fields)
	if err != nil {
		zapFields = append(zapFields, zap.Error(err))
	}
	l.logger.Error(msg, zapFields...)
}

// Fatal logs a fatal message and exits
func (l *LoggerAdapter) Fatal(msg string, err error, fields ...ports.Field) {
	zapFields := l.convertFields(fields)
	if err != nil {
		zapFields = append(zapFields, zap.Error(err))
	}
	l.logger.Fatal(msg, zapFields...)
}

// WithFields returns a logger with additional fields
func (l *LoggerAdapter) WithFields(fields ...ports.Field) ports.Logger {
	zapFields := l.convertFields(fields)
	return &LoggerAdapter{
		logger: l.logger.With(zapFields...),
	}
}

// WithContext returns a logger with context
func (l *LoggerAdapter) WithContext(ctx context.Context) ports.Logger {
	// Extract common context values if needed
	// For now, just return self as context handling can be added later
	return l
}

// convertFields converts ports.Field to zap.Field
func (l *LoggerAdapter) convertFields(fields []ports.Field) []zapcore.Field {
	zapFields := make([]zapcore.Field, 0, len(fields))
	for _, field := range fields {
		zapFields = append(zapFields, zap.Any(field.Key, field.Value))
	}
	return zapFields
}

// Sync flushes any buffered log entries
func (l *LoggerAdapter) Sync() error {
	return l.logger.Sync()
}