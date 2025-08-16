// Package response provides consistent HTTP response builders following REST best practices.
// This package demonstrates patterns for building structured, consistent API responses
// with proper HTTP semantics, caching headers, and HATEOAS support.
//
// Key Concepts Illustrated:
//   - Consistent Response Format: All responses follow the same structure
//   - HTTP Semantics: Proper use of status codes and headers
//   - HATEOAS: Hypermedia as the Engine of Application State
//   - Caching: ETags and cache control headers
//   - Pagination: Standardized pagination metadata
//
// Design Principles:
//   - Builder Pattern: Fluent interface for response construction
//   - Type Safety: Strongly typed response structures
//   - Performance: Efficient JSON encoding
//   - Standards Compliance: Following REST and HTTP standards
//   - Extensibility: Easy to add new response types
package response

import (
	"crypto/md5"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"
)

// Response represents a standard API response structure
type Response struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Error   interface{} `json:"error,omitempty"`
	Meta    *Meta       `json:"meta,omitempty"`
	Links   *Links      `json:"links,omitempty"`
}

// Meta contains response metadata
type Meta struct {
	RequestID  string         `json:"request_id,omitempty"`
	Timestamp  string         `json:"timestamp"`
	Version    string         `json:"version,omitempty"`
	Pagination *Pagination    `json:"pagination,omitempty"`
	RateLimit  *RateLimit     `json:"rate_limit,omitempty"`
	Extra      map[string]any `json:"extra,omitempty"`
}

// Pagination contains pagination metadata
type Pagination struct {
	Page       int    `json:"page"`
	PerPage    int    `json:"per_page"`
	Total      int    `json:"total"`
	TotalPages int    `json:"total_pages"`
	HasNext    bool   `json:"has_next"`
	HasPrev    bool   `json:"has_prev"`
	NextToken  string `json:"next_token,omitempty"`
}

// RateLimit contains rate limiting information
type RateLimit struct {
	Limit     int   `json:"limit"`
	Remaining int   `json:"remaining"`
	Reset     int64 `json:"reset"` // Unix timestamp
}

// Links contains HATEOAS links
type Links struct {
	Self  string            `json:"self,omitempty"`
	Next  string            `json:"next,omitempty"`
	Prev  string            `json:"prev,omitempty"`
	First string            `json:"first,omitempty"`
	Last  string            `json:"last,omitempty"`
	Extra map[string]string `json:"extra,omitempty"`
}

// Builder provides a fluent interface for building responses
type Builder struct {
	response  *Response
	writer    http.ResponseWriter
	request   *http.Request
	status    int
	headers   map[string]string
	cookies   []*http.Cookie
	cacheTime time.Duration
}

// New creates a new response builder
func New(w http.ResponseWriter, r *http.Request) *Builder {
	return &Builder{
		writer:  w,
		request: r,
		response: &Response{
			Success: true,
			Meta: &Meta{
				Timestamp: time.Now().UTC().Format(time.RFC3339),
			},
		},
		status:  http.StatusOK,
		headers: make(map[string]string),
	}
}

// Status sets the HTTP status code
func (b *Builder) Status(code int) *Builder {
	b.status = code
	b.response.Success = code >= 200 && code < 300
	return b
}

// Data sets the response data
func (b *Builder) Data(data interface{}) *Builder {
	b.response.Data = data
	return b
}

// Error sets the error response
func (b *Builder) Error(err interface{}) *Builder {
	b.response.Success = false
	b.response.Error = err
	b.response.Data = nil // Clear data on error
	return b
}

// WithRequestID adds request ID to metadata
func (b *Builder) WithRequestID(id string) *Builder {
	if b.response.Meta == nil {
		b.response.Meta = &Meta{}
	}
	b.response.Meta.RequestID = id
	return b
}

// WithVersion adds API version to metadata
func (b *Builder) WithVersion(version string) *Builder {
	if b.response.Meta == nil {
		b.response.Meta = &Meta{}
	}
	b.response.Meta.Version = version
	return b
}

// WithPagination adds pagination metadata
func (b *Builder) WithPagination(page, perPage, total int) *Builder {
	if b.response.Meta == nil {
		b.response.Meta = &Meta{}
	}

	totalPages := (total + perPage - 1) / perPage
	
	b.response.Meta.Pagination = &Pagination{
		Page:       page,
		PerPage:    perPage,
		Total:      total,
		TotalPages: totalPages,
		HasNext:    page < totalPages,
		HasPrev:    page > 1,
	}

	// Add pagination links
	b.addPaginationLinks(page, totalPages)

	return b
}

// WithNextToken adds token-based pagination
func (b *Builder) WithNextToken(token string, hasMore bool) *Builder {
	if b.response.Meta == nil {
		b.response.Meta = &Meta{}
	}

	if b.response.Meta.Pagination == nil {
		b.response.Meta.Pagination = &Pagination{}
	}

	b.response.Meta.Pagination.NextToken = token
	b.response.Meta.Pagination.HasNext = hasMore

	return b
}

// WithRateLimit adds rate limiting information
func (b *Builder) WithRateLimit(limit, remaining int, reset time.Time) *Builder {
	if b.response.Meta == nil {
		b.response.Meta = &Meta{}
	}

	b.response.Meta.RateLimit = &RateLimit{
		Limit:     limit,
		Remaining: remaining,
		Reset:     reset.Unix(),
	}

	// Also set rate limit headers
	b.Header("X-RateLimit-Limit", strconv.Itoa(limit))
	b.Header("X-RateLimit-Remaining", strconv.Itoa(remaining))
	b.Header("X-RateLimit-Reset", strconv.FormatInt(reset.Unix(), 10))

	return b
}

// WithLinks adds HATEOAS links
func (b *Builder) WithLinks(links *Links) *Builder {
	b.response.Links = links
	return b
}

// WithSelfLink adds a self link
func (b *Builder) WithSelfLink(url string) *Builder {
	if b.response.Links == nil {
		b.response.Links = &Links{}
	}
	b.response.Links.Self = url
	return b
}

// Header adds a response header
func (b *Builder) Header(key, value string) *Builder {
	b.headers[key] = value
	return b
}

// Cookie adds a response cookie
func (b *Builder) Cookie(cookie *http.Cookie) *Builder {
	b.cookies = append(b.cookies, cookie)
	return b
}

// Cache sets cache control headers
func (b *Builder) Cache(duration time.Duration) *Builder {
	b.cacheTime = duration
	if duration > 0 {
		b.Header("Cache-Control", fmt.Sprintf("public, max-age=%d", int(duration.Seconds())))
	} else {
		b.Header("Cache-Control", "no-cache, no-store, must-revalidate")
	}
	return b
}

// NoCache sets headers to prevent caching
func (b *Builder) NoCache() *Builder {
	return b.Cache(0)
}

// ETag adds an ETag header
func (b *Builder) ETag(tag string) *Builder {
	b.Header("ETag", fmt.Sprintf(`"%s"`, tag))
	return b
}

// Send writes the response to the client
func (b *Builder) Send() error {
	// Set headers
	b.writer.Header().Set("Content-Type", "application/json")
	
	for key, value := range b.headers {
		b.writer.Header().Set(key, value)
	}

	// Set cookies
	for _, cookie := range b.cookies {
		http.SetCookie(b.writer, cookie)
	}

	// Generate ETag if caching is enabled and not already set
	if b.cacheTime > 0 && b.writer.Header().Get("ETag") == "" {
		if etag := b.generateETag(); etag != "" {
			b.writer.Header().Set("ETag", etag)
		}
	}

	// Check for conditional requests
	if b.handleConditional() {
		return nil // 304 Not Modified was sent
	}

	// Write status code
	b.writer.WriteHeader(b.status)

	// Encode and send response
	encoder := json.NewEncoder(b.writer)
	encoder.SetIndent("", "  ") // Pretty print in development
	return encoder.Encode(b.response)
}

// generateETag generates an ETag based on response data
func (b *Builder) generateETag() string {
	if b.response.Data == nil {
		return ""
	}

	data, err := json.Marshal(b.response.Data)
	if err != nil {
		return ""
	}

	hash := md5.Sum(data)
	return fmt.Sprintf(`"%x"`, hash)
}

// handleConditional handles conditional requests (If-None-Match, If-Modified-Since)
func (b *Builder) handleConditional() bool {
	etag := b.writer.Header().Get("ETag")
	if etag == "" {
		return false
	}

	// Check If-None-Match
	if match := b.request.Header.Get("If-None-Match"); match != "" {
		if match == etag {
			b.writer.WriteHeader(http.StatusNotModified)
			return true
		}
	}

	return false
}

// addPaginationLinks adds HATEOAS links for pagination
func (b *Builder) addPaginationLinks(page, totalPages int) {
	if b.response.Links == nil {
		b.response.Links = &Links{}
	}

	baseURL := b.request.URL.String()

	// Self link
	b.response.Links.Self = fmt.Sprintf("%s?page=%d", baseURL, page)

	// Navigation links
	if page > 1 {
		b.response.Links.First = fmt.Sprintf("%s?page=1", baseURL)
		b.response.Links.Prev = fmt.Sprintf("%s?page=%d", baseURL, page-1)
	}

	if page < totalPages {
		b.response.Links.Next = fmt.Sprintf("%s?page=%d", baseURL, page+1)
		b.response.Links.Last = fmt.Sprintf("%s?page=%d", baseURL, totalPages)
	}
}

// Helper functions for common responses

// OK sends a 200 OK response
func OK(w http.ResponseWriter, r *http.Request, data interface{}) error {
	return New(w, r).
		Status(http.StatusOK).
		Data(data).
		Send()
}

// Created sends a 201 Created response
func Created(w http.ResponseWriter, r *http.Request, data interface{}, location string) error {
	return New(w, r).
		Status(http.StatusCreated).
		Header("Location", location).
		Data(data).
		Send()
}

// NoContent sends a 204 No Content response
func NoContent(w http.ResponseWriter, r *http.Request) error {
	w.WriteHeader(http.StatusNoContent)
	return nil
}

// BadRequest sends a 400 Bad Request response
func BadRequest(w http.ResponseWriter, r *http.Request, err interface{}) error {
	return New(w, r).
		Status(http.StatusBadRequest).
		Error(err).
		Send()
}

// Unauthorized sends a 401 Unauthorized response
func Unauthorized(w http.ResponseWriter, r *http.Request, message string) error {
	return New(w, r).
		Status(http.StatusUnauthorized).
		Error(map[string]string{"message": message}).
		Send()
}

// Forbidden sends a 403 Forbidden response
func Forbidden(w http.ResponseWriter, r *http.Request, message string) error {
	return New(w, r).
		Status(http.StatusForbidden).
		Error(map[string]string{"message": message}).
		Send()
}

// NotFound sends a 404 Not Found response
func NotFound(w http.ResponseWriter, r *http.Request, resource string) error {
	return New(w, r).
		Status(http.StatusNotFound).
		Error(map[string]string{
			"message": fmt.Sprintf("%s not found", resource),
			"resource": resource,
		}).
		Send()
}

// InternalServerError sends a 500 Internal Server Error response
func InternalServerError(w http.ResponseWriter, r *http.Request) error {
	return New(w, r).
		Status(http.StatusInternalServerError).
		Error(map[string]string{"message": "An internal error occurred"}).
		Send()
}

// Paginated sends a paginated response
func Paginated(w http.ResponseWriter, r *http.Request, data interface{}, page, perPage, total int) error {
	return New(w, r).
		Status(http.StatusOK).
		Data(data).
		WithPagination(page, perPage, total).
		Send()
}

// File sends a file response
func File(w http.ResponseWriter, r *http.Request, contentType string, filename string, data []byte) {
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
	w.Header().Set("Content-Length", strconv.Itoa(len(data)))
	w.WriteHeader(http.StatusOK)
	w.Write(data)
}

// Stream sets up response for streaming
func Stream(w http.ResponseWriter, r *http.Request, contentType string) {
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Transfer-Encoding", "chunked")
}

// extractRequestID attempts to extract request ID from context
func extractRequestID(r *http.Request) string {
	if id := r.Context().Value("request_id"); id != nil {
		if str, ok := id.(string); ok {
			return str
		}
	}
	return r.Header.Get("X-Request-ID")
}