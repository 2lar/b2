// Package valueobjects contains content-related value objects
package valueobjects

import (
	"fmt"
	"regexp"
	"strings"
	"unicode"
)

// Content represents the main content of a node with validation rules
type Content struct {
	value string
}

const (
	MinContentLength = 1
	MaxContentLength = 10000
)

// NewContent creates a new Content value object
func NewContent(value string) Content {
	return Content{value: strings.TrimSpace(value)}
}

// String returns the string representation
func (c Content) String() string {
	return c.value
}

// Equals checks equality with another Content
func (c Content) Equals(other Content) bool {
	return c.value == other.value
}

// Validate checks if the content is valid
func (c Content) Validate() error {
	if len(c.value) < MinContentLength {
		return fmt.Errorf("content too short (minimum %d characters)", MinContentLength)
	}
	if len(c.value) > MaxContentLength {
		return fmt.Errorf("content too long (maximum %d characters)", MaxContentLength)
	}
	return nil
}

// WordCount returns the number of words in the content
func (c Content) WordCount() int {
	words := strings.Fields(c.value)
	return len(words)
}

// CharacterCount returns the number of characters
func (c Content) CharacterCount() int {
	return len(c.value)
}

// ExtractKeywords extracts important keywords from the content
func (c Content) ExtractKeywords() Keywords {
	// Simple keyword extraction - in production, use NLP
	words := strings.Fields(strings.ToLower(c.value))
	
	// Remove common stop words
	stopWords := map[string]bool{
		"the": true, "a": true, "an": true, "and": true, "or": true,
		"but": true, "in": true, "on": true, "at": true, "to": true,
		"for": true, "of": true, "with": true, "by": true, "from": true,
		"as": true, "is": true, "was": true, "are": true, "were": true,
		"be": true, "been": true, "being": true, "have": true, "has": true,
		"had": true, "do": true, "does": true, "did": true, "will": true,
		"would": true, "could": true, "should": true, "may": true, "might": true,
		"must": true, "can": true, "this": true, "that": true, "these": true,
		"those": true, "i": true, "you": true, "he": true, "she": true,
		"it": true, "we": true, "they": true, "what": true, "which": true,
		"who": true, "when": true, "where": true, "why": true, "how": true,
	}
	
	// Extract unique keywords
	keywordMap := make(map[string]bool)
	for _, word := range words {
		// Clean the word
		word = strings.Trim(word, ".,!?;:'\"")
		
		// Skip if stop word or too short
		if stopWords[word] || len(word) < 3 {
			continue
		}
		
		// Add to keywords
		keywordMap[word] = true
		
		// Limit keywords
		if len(keywordMap) >= 20 {
			break
		}
	}
	
	// Convert to slice
	keywords := make([]string, 0, len(keywordMap))
	for keyword := range keywordMap {
		keywords = append(keywords, keyword)
	}
	
	return NewKeywords(keywords)
}

// Title represents a node title with validation
type Title struct {
	value string
}

const (
	MaxTitleLength = 200
)

// NewTitle creates a new Title value object
func NewTitle(value string) Title {
	return Title{value: strings.TrimSpace(value)}
}

// String returns the string representation
func (t Title) String() string {
	return t.value
}

// Equals checks equality with another Title
func (t Title) Equals(other Title) bool {
	return t.value == other.value
}

// Validate checks if the title is valid
func (t Title) Validate() error {
	// Title is optional, so empty is valid
	if t.value == "" {
		return nil
	}
	if len(t.value) > MaxTitleLength {
		return fmt.Errorf("title too long (maximum %d characters)", MaxTitleLength)
	}
	return nil
}

// IsEmpty checks if the title is empty
func (t Title) IsEmpty() bool {
	return t.value == ""
}

// Keywords represents extracted keywords from content
type Keywords struct {
	values []string
}

// NewKeywords creates a new Keywords value object
func NewKeywords(values []string) Keywords {
	// Normalize and deduplicate
	seen := make(map[string]bool)
	unique := make([]string, 0, len(values))
	
	for _, v := range values {
		normalized := strings.ToLower(strings.TrimSpace(v))
		if normalized != "" && !seen[normalized] {
			seen[normalized] = true
			unique = append(unique, normalized)
		}
	}
	
	return Keywords{values: unique}
}

// ToSlice returns the keywords as a slice
func (k Keywords) ToSlice() []string {
	return k.values
}

// Values returns the keywords as a slice (alias for ToSlice)
func (k Keywords) Values() []string {
	return k.values
}

// Contains checks if a keyword exists
func (k Keywords) Contains(keyword string) bool {
	normalized := strings.ToLower(strings.TrimSpace(keyword))
	for _, v := range k.values {
		if v == normalized {
			return true
		}
	}
	return false
}

// Count returns the number of keywords
func (k Keywords) Count() int {
	return len(k.values)
}

// Tags represents user-defined tags for categorization
type Tags struct {
	values []string
}

const (
	MaxTagLength = 50
	MaxTags      = 20
)

// NewTags creates a new Tags value object
func NewTags(values []string) Tags {
	// Normalize and deduplicate
	seen := make(map[string]bool)
	unique := make([]string, 0, len(values))
	
	for _, v := range values {
		normalized := normalizeTag(v)
		if normalized != "" && !seen[normalized] {
			seen[normalized] = true
			unique = append(unique, normalized)
		}
	}
	
	return Tags{values: unique}
}

// normalizeTag normalizes a tag value
func normalizeTag(tag string) string {
	// Convert to lowercase and trim
	tag = strings.ToLower(strings.TrimSpace(tag))
	
	// Replace spaces with hyphens
	tag = strings.ReplaceAll(tag, " ", "-")
	
	// Remove special characters except hyphens and underscores
	var result strings.Builder
	for _, r := range tag {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '-' || r == '_' {
			result.WriteRune(r)
		}
	}
	
	return result.String()
}

// ToSlice returns the tags as a slice
func (t Tags) ToSlice() []string {
	return t.values
}

// Values returns the tags as a slice (alias for ToSlice)
func (t Tags) Values() []string {
	return t.values
}

// Contains checks if a tag exists
func (t Tags) Contains(tag string) bool {
	normalized := normalizeTag(tag)
	for _, v := range t.values {
		if v == normalized {
			return true
		}
	}
	return false
}

// Add adds new tags
func (t Tags) Add(tags ...string) Tags {
	combined := append(t.values, tags...)
	return NewTags(combined)
}

// Remove removes tags
func (t Tags) Remove(tags ...string) Tags {
	toRemove := make(map[string]bool)
	for _, tag := range tags {
		toRemove[normalizeTag(tag)] = true
	}
	
	result := make([]string, 0, len(t.values))
	for _, v := range t.values {
		if !toRemove[v] {
			result = append(result, v)
		}
	}
	
	return Tags{values: result}
}

// Count returns the number of tags
func (t Tags) Count() int {
	return len(t.values)
}

// Validate checks if all tags are valid
func (t Tags) Validate() error {
	if len(t.values) > MaxTags {
		return fmt.Errorf("too many tags (maximum %d)", MaxTags)
	}
	
	for _, tag := range t.values {
		if len(tag) > MaxTagLength {
			return fmt.Errorf("tag '%s' too long (maximum %d characters)", tag, MaxTagLength)
		}
		if len(tag) == 0 {
			return fmt.Errorf("empty tag not allowed")
		}
	}
	
	return nil
}

// URL represents a validated URL
type URL struct {
	value string
}

var urlRegex = regexp.MustCompile(`^(https?|ftp)://[^\s/$.?#].[^\s]*$`)

// NewURL creates a new URL value object
func NewURL(value string) (URL, error) {
	url := URL{value: strings.TrimSpace(value)}
	if err := url.Validate(); err != nil {
		return URL{}, err
	}
	return url, nil
}

// String returns the string representation
func (u URL) String() string {
	return u.value
}

// Equals checks equality with another URL
func (u URL) Equals(other URL) bool {
	return u.value == other.value
}

// Validate checks if the URL is valid
func (u URL) Validate() error {
	if u.value == "" {
		return fmt.Errorf("URL cannot be empty")
	}
	if !urlRegex.MatchString(u.value) {
		return fmt.Errorf("invalid URL format")
	}
	if len(u.value) > 2048 {
		return fmt.Errorf("URL too long")
	}
	return nil
}

// GetDomain extracts the domain from the URL
func (u URL) GetDomain() string {
	// Simple extraction - in production use net/url package
	parts := strings.Split(u.value, "/")
	if len(parts) >= 3 {
		return parts[2]
	}
	return ""
}