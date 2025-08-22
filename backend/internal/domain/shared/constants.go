// Package shared contains domain constants and shared values
package shared

// Document editing constants for frontend-backend consistency
const (
	// MaxContentLength is the maximum allowed content length in characters
	MaxContentLength = 20000 // 20KB limit, approximately 4 pages
	
	// DocumentSuggestionThreshold is when to show document mode suggestion
	DocumentSuggestionThreshold = 800
	
	// DocumentAutoOpenThreshold is when to automatically open document editor
	DocumentAutoOpenThreshold = 1200
	
	// MaxInlineLength is the absolute maximum for inline input mode
	MaxInlineLength = 1500
)

// Legacy constants for backward compatibility
const (
	// DefaultMaxContentLength is the old content limit
	DefaultMaxContentLength = 10000
)