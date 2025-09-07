package valueobjects

import (
	"errors"
	"strings"
	"unicode/utf8"
)

const (
	MaxTitleLength   = 200
	MaxContentLength = 50000
)

// ContentFormat represents the format of the content
type ContentFormat string

const (
	FormatPlainText ContentFormat = "text"
	FormatMarkdown  ContentFormat = "markdown"
	FormatHTML      ContentFormat = "html"
	FormatJSON      ContentFormat = "json"
)

// NodeContent is a value object for node content
type NodeContent struct {
	title  string
	body   string
	format ContentFormat
}

// NewNodeContent creates content with validation
func NewNodeContent(title, body string, format ContentFormat) (NodeContent, error) {
	title = strings.TrimSpace(title)
	body = strings.TrimSpace(body)

	if title == "" {
		return NodeContent{}, errors.New("title cannot be empty")
	}
	
	if utf8.RuneCountInString(title) > MaxTitleLength {
		return NodeContent{}, errors.New("title exceeds maximum length")
	}
	
	if utf8.RuneCountInString(body) > MaxContentLength {
		return NodeContent{}, errors.New("content body exceeds maximum length")
	}
	
	if !isValidFormat(format) {
		return NodeContent{}, errors.New("invalid content format")
	}
	
	return NodeContent{
		title:  title,
		body:   body,
		format: format,
	}, nil
}

// Title returns the content title
func (c NodeContent) Title() string {
	return c.title
}

// Body returns the content body
func (c NodeContent) Body() string {
	return c.body
}

// Format returns the content format
func (c NodeContent) Format() ContentFormat {
	return c.format
}

// IsEmpty checks if content is empty
func (c NodeContent) IsEmpty() bool {
	return c.title == "" && c.body == ""
}

// Equals checks if two contents are equal
func (c NodeContent) Equals(other NodeContent) bool {
	return c.title == other.title && 
		c.body == other.body && 
		c.format == other.format
}

// WordCount returns the approximate word count
func (c NodeContent) WordCount() int {
	combined := c.title + " " + c.body
	return len(strings.Fields(combined))
}

// Summary returns a truncated summary of the content
func (c NodeContent) Summary(maxLength int) string {
	if maxLength <= 0 {
		return ""
	}
	
	combined := c.title
	if c.body != "" {
		combined += ": " + c.body
	}
	
	if utf8.RuneCountInString(combined) <= maxLength {
		return combined
	}
	
	runes := []rune(combined)
	return string(runes[:maxLength-3]) + "..."
}

func isValidFormat(format ContentFormat) bool {
	switch format {
	case FormatPlainText, FormatMarkdown, FormatHTML, FormatJSON:
		return true
	default:
		return false
	}
}