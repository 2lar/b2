package valueobjects

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"backend2/domain/config"
	pkgerrors "backend2/pkg/errors"
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

// NewNodeContent creates content with validation using default configuration
func NewNodeContent(title, body string, format ContentFormat) (NodeContent, error) {
	return NewNodeContentWithConfig(title, body, format, config.DefaultDomainConfig())
}

// NewNodeContentWithConfig creates content with validation and configuration
func NewNodeContentWithConfig(title, body string, format ContentFormat, cfg *config.DomainConfig) (NodeContent, error) {
	if cfg == nil {
		cfg = config.DefaultDomainConfig()
	}

	title = strings.TrimSpace(title)
	body = strings.TrimSpace(body)

	if title == "" && !cfg.AllowEmptyContent {
		return NodeContent{}, pkgerrors.NewValidationError("title cannot be empty")
	}

	titleLength := utf8.RuneCountInString(title)
	if titleLength < cfg.MinTitleLength {
		return NodeContent{}, fmt.Errorf("title too short: minimum %d characters required", cfg.MinTitleLength)
	}

	if titleLength > cfg.MaxTitleLength {
		return NodeContent{}, fmt.Errorf("title exceeds maximum length of %d characters", cfg.MaxTitleLength)
	}

	if utf8.RuneCountInString(body) > cfg.MaxContentLength {
		return NodeContent{}, fmt.Errorf("content body exceeds maximum length of %d characters", cfg.MaxContentLength)
	}

	if !isValidFormat(format) {
		return NodeContent{}, pkgerrors.NewValidationError("invalid content format")
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
