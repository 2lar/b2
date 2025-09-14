package valueobjects

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewNodeContent(t *testing.T) {
	tests := []struct {
		name    string
		title   string
		body    string
		format  ContentFormat
		wantErr bool
		errMsg  string
	}{
		{
			name:    "valid content with title and body",
			title:   "Test Title",
			body:    "Test body content",
			format:  FormatMarkdown,
			wantErr: false,
		},
		{
			name:    "valid content with only title",
			title:   "Title Only",
			body:    "",
			format:  FormatPlainText,
			wantErr: false,
		},
		{
			name:    "empty title",
			title:   "",
			body:    "Body content",
			format:  FormatMarkdown,
			wantErr: true,
			errMsg:  "title cannot be empty",
		},
		{
			name:    "whitespace only title",
			title:   "   ",
			body:    "Body content",
			format:  FormatMarkdown,
			wantErr: true,
			errMsg:  "title cannot be empty",
		},
		{
			name:    "title too long",
			title:   strings.Repeat("a", 256),
			body:    "Body",
			format:  FormatMarkdown,
			wantErr: true,
			errMsg:  "title exceeds maximum length",
		},
		{
			name:    "title at max length",
			title:   strings.Repeat("a", 200),
			body:    "Body",
			format:  FormatMarkdown,
			wantErr: false,
		},
		{
			name:    "body too long",
			title:   "Title",
			body:    strings.Repeat("a", 50001),
			format:  FormatMarkdown,
			wantErr: true,
			errMsg:  "body exceeds maximum length",
		},
		{
			name:    "body at max length",
			title:   "Title",
			body:    strings.Repeat("a", 50000),
			format:  FormatMarkdown,
			wantErr: false,
		},
		{
			name:    "HTML format",
			title:   "HTML Title",
			body:    "<p>HTML content</p>",
			format:  FormatHTML,
			wantErr: false,
		},
		{
			name:    "invalid format",
			title:   "Title",
			body:    "Body",
			format:  ContentFormat("invalid"),
			wantErr: true,
			errMsg:  "invalid content format",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			content, err := NewNodeContent(tt.title, tt.body, tt.format)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
			} else {
				require.NoError(t, err)
				assert.NotNil(t, content)
				assert.Equal(t, strings.TrimSpace(tt.title), content.Title())
				assert.Equal(t, tt.body, content.Body())
				assert.Equal(t, tt.format, content.Format())
			}
		})
	}
}

func TestNodeContent_Getters(t *testing.T) {
	title := "Test Title"
	body := "Test body content"
	format := FormatMarkdown

	content, err := NewNodeContent(title, body, format)
	require.NoError(t, err)

	assert.Equal(t, title, content.Title())
	assert.Equal(t, body, content.Body())
	assert.Equal(t, format, content.Format())
}

func TestNodeContent_IsEmpty(t *testing.T) {
	tests := []struct {
		name     string
		title    string
		body     string
		expected bool
	}{
		{
			name:     "content with title and body is not empty",
			title:    "Title",
			body:     "Body",
			expected: false,
		},
		{
			name:     "content with only title is not empty",
			title:    "Title",
			body:     "",
			expected: false,
		},
		{
			name:     "zero value would be empty (but can't create)",
			title:    "Required", // Title is required
			body:     "",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			content, err := NewNodeContent(tt.title, tt.body, FormatMarkdown)
			require.NoError(t, err)

			result := content.IsEmpty()
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestNodeContent_WordCount(t *testing.T) {
	tests := []struct {
		name     string
		title    string
		body     string
		expected int
	}{
		{
			name:     "simple word count",
			title:    "Test Title",
			body:     "This is a test body",
			expected: 7, // 2 + 5
		},
		{
			name:     "empty body",
			title:    "Just Title",
			body:     "",
			expected: 2,
		},
		{
			name:     "with punctuation",
			title:    "Hello, World!",
			body:     "This is a test. Another sentence here!",
			expected: 9, // 2 + 7
		},
		{
			name:     "with newlines",
			title:    "Multi Line",
			body:     "Line one\nLine two\nLine three",
			expected: 8, // 2 + 6
		},
		{
			name:     "with multiple spaces",
			title:    "Spaced  Out",
			body:     "Multiple   spaces    between   words",
			expected: 6, // 2 + 4
		},
		{
			name:     "with markdown",
			title:    "Markdown Title",
			body:     "# Heading\n**Bold** text and *italic* text",
			expected: 9, // 2 + 7 (markdown symbols counted as words)
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			content, err := NewNodeContent(tt.title, tt.body, FormatMarkdown)
			require.NoError(t, err)

			count := content.WordCount()
			assert.Equal(t, tt.expected, count)
		})
	}
}

func TestNodeContent_Summary(t *testing.T) {
	tests := []struct {
		name      string
		title     string
		body      string
		maxLength int
		expected  string
	}{
		{
			name:      "short content no truncation",
			title:     "Title",
			body:      "Short body",
			maxLength: 50,
			expected:  "Title: Short body",
		},
		{
			name:      "long body gets truncated",
			title:     "Title",
			body:      strings.Repeat("a", 100),
			maxLength: 30,
			expected:  "Title: " + strings.Repeat("a", 20) + "...",
		},
		{
			name:      "empty body",
			title:     "Title Only",
			body:      "",
			maxLength: 50,
			expected:  "Title Only",
		},
		{
			name:      "very short max length",
			title:     "Long Title Here",
			body:      "Body content",
			maxLength: 10,
			expected:  "Long Ti...",
		},
		{
			name:      "multiline body takes first line",
			title:     "Title",
			body:      "First line\nSecond line\nThird line",
			maxLength: 50,
			expected:  "Title: First line",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			content, err := NewNodeContent(tt.title, tt.body, FormatMarkdown)
			require.NoError(t, err)

			summary := content.Summary(tt.maxLength)
			assert.Equal(t, tt.expected, summary)
			assert.LessOrEqual(t, len(summary), tt.maxLength)
		})
	}
}

// UpdateTitle test removed - value objects are immutable

// UpdateBody test removed - value objects are immutable

// ChangeFormat test removed - value objects are immutable

func TestContentFormat_Validation(t *testing.T) {
	tests := []struct {
		name     string
		format   ContentFormat
		expected bool
	}{
		{
			name:     "markdown is valid",
			format:   FormatMarkdown,
			expected: true,
		},
		{
			name:     "plain text is valid",
			format:   FormatPlainText,
			expected: true,
		},
		{
			name:     "HTML is valid",
			format:   FormatHTML,
			expected: true,
		},
		{
			name:     "JSON is valid",
			format:   FormatJSON,
			expected: true,
		},
		{
			name:     "invalid format",
			format:   ContentFormat("invalid"),
			expected: false,
		},
		{
			name:     "empty format",
			format:   ContentFormat(""),
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test by trying to create content with the format
			_, err := NewNodeContent("Title", "Body", tt.format)
			if tt.expected {
				assert.NoError(t, err)
			} else {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), "invalid content format")
			}
		})
	}
}

// Benchmarks
func BenchmarkNewNodeContent(b *testing.B) {
	title := "Benchmark Title"
	body := strings.Repeat("Content ", 100)

	for i := 0; i < b.N; i++ {
		_, _ = NewNodeContent(title, body, FormatMarkdown)
	}
}

func BenchmarkNodeContent_WordCount(b *testing.B) {
	content, _ := NewNodeContent(
		"Benchmark Title",
		strings.Repeat("word ", 1000),
		FormatMarkdown,
	)
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_ = content.WordCount()
	}
}

func BenchmarkNodeContent_Summary(b *testing.B) {
	content, _ := NewNodeContent(
		"Benchmark Title",
		strings.Repeat("Content ", 100),
		FormatMarkdown,
	)
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_ = content.Summary(100)
	}
}