package common

import (
	"encoding/json"
	"net/http"
)

// APIResponse represents a standard API response
type APIResponse struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Error   *ErrorInfo  `json:"error,omitempty"`
	Meta    *MetaInfo   `json:"meta,omitempty"`
}

// ErrorInfo contains error details
type ErrorInfo struct {
	Code    string                 `json:"code"`
	Message string                 `json:"message"`
	Details map[string]interface{} `json:"details,omitempty"`
}

// MetaInfo contains metadata about the response
type MetaInfo struct {
	RequestID  string `json:"request_id,omitempty"`
	Timestamp  string `json:"timestamp,omitempty"`
	Version    string `json:"version,omitempty"`
	Pagination *PaginationInfo `json:"pagination,omitempty"`
}

// PaginationInfo contains pagination details
type PaginationInfo struct {
	Page       int  `json:"page"`
	PageSize   int  `json:"page_size"`
	Total      int  `json:"total"`
	TotalPages int  `json:"total_pages"`
	HasNext    bool `json:"has_next"`
	HasPrev    bool `json:"has_prev"`
}

// RespondJSON sends a JSON response
func RespondJSON(w http.ResponseWriter, status int, data interface{}) {
	response := APIResponse{
		Success: status >= 200 && status < 300,
		Data:    data,
	}
	
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(response)
}

// RespondError sends an error response
func RespondError(w http.ResponseWriter, status int, code, message string) {
	response := APIResponse{
		Success: false,
		Error: &ErrorInfo{
			Code:    code,
			Message: message,
		},
	}
	
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(response)
}

// RespondErrorWithDetails sends an error response with additional details
func RespondErrorWithDetails(w http.ResponseWriter, status int, code, message string, details map[string]interface{}) {
	response := APIResponse{
		Success: false,
		Error: &ErrorInfo{
			Code:    code,
			Message: message,
			Details: details,
		},
	}
	
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(response)
}

// RespondWithMeta sends a response with metadata
func RespondWithMeta(w http.ResponseWriter, status int, data interface{}, meta *MetaInfo) {
	response := APIResponse{
		Success: status >= 200 && status < 300,
		Data:    data,
		Meta:    meta,
	}
	
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(response)
}

// ExtractRequestID extracts the request ID from the request context
func ExtractRequestID(r *http.Request) string {
	// Try various headers
	if id := r.Header.Get("X-Request-ID"); id != "" {
		return id
	}
	if id := r.Header.Get("X-Request-Id"); id != "" {
		return id
	}
	if id := r.Header.Get("X-Amzn-Trace-Id"); id != "" {
		return id
	}
	
	// Try context value
	if id, ok := r.Context().Value("request_id").(string); ok {
		return id
	}
	
	return ""
}

// StandardErrorCodes defines common error codes
var StandardErrorCodes = struct {
	ValidationError   string
	NotFound          string
	Unauthorized      string
	Forbidden         string
	Conflict          string
	InternalError     string
	BadRequest        string
	TooManyRequests   string
	ServiceUnavailable string
}{
	ValidationError:   "VALIDATION_ERROR",
	NotFound:          "NOT_FOUND",
	Unauthorized:      "UNAUTHORIZED",
	Forbidden:         "FORBIDDEN",
	Conflict:          "CONFLICT",
	InternalError:     "INTERNAL_ERROR",
	BadRequest:        "BAD_REQUEST",
	TooManyRequests:   "TOO_MANY_REQUESTS",
	ServiceUnavailable: "SERVICE_UNAVAILABLE",
}

// ParseJSONBody parses JSON request body with size limit
func ParseJSONBody(r *http.Request, v interface{}, maxBytes int64) error {
	r.Body = http.MaxBytesReader(nil, r.Body, maxBytes)
	
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	
	if err := decoder.Decode(v); err != nil {
		return err
	}
	
	return nil
}