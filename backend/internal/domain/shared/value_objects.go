package shared

import (
	"regexp"
	"sort"
	"strings"

	"github.com/google/uuid"
)

// Pre-compiled regular expressions for better cold start performance
var (
	// alphanumericOnlyRegex removes all non-alphanumeric characters
	alphanumericOnlyRegex = regexp.MustCompile(`[^a-zA-Z0-9]`)
	// contentCleanupRegex removes non-alphanumeric characters except spaces
	contentCleanupRegex = regexp.MustCompile(`[^a-zA-Z0-9 ]+`)
	// tagSpecialCharsRegex removes special characters from tags
	tagSpecialCharsRegex = regexp.MustCompile(`[^a-zA-Z0-9\s-]`)
	// tagSpaceRegex replaces multiple spaces with single hyphens
	tagSpaceRegex = regexp.MustCompile(`\s+`)
)

// NodeID is a value object that ensures valid node identifiers
type NodeID struct {
	value string
}

// NewNodeID creates a new random NodeID
func NewNodeID() NodeID {
	return NodeID{value: uuid.New().String()}
}

// ParseNodeID creates a NodeID from a string, validating it's a proper UUID
func ParseNodeID(id string) (NodeID, error) {
	if _, err := uuid.Parse(id); err != nil {
		return NodeID{}, ErrInvalidNodeID
	}
	return NodeID{value: id}, nil
}

// String returns the string representation of the NodeID
func (id NodeID) String() string { 
	return id.value 
}

// Equals checks if two NodeIDs are equal
func (id NodeID) Equals(other NodeID) bool { 
	return id.value == other.value 
}

// IsEmpty checks if the NodeID is empty
func (id NodeID) IsEmpty() bool {
	return id.value == ""
}

// UserID is a value object that ensures valid user identifiers
type UserID struct {
	value string
}

// NewUserID creates a new UserID from a string with validation
func NewUserID(id string) (UserID, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return UserID{}, ErrEmptyUserID
	}
	if len(id) > MaxUserIDLength {
		return UserID{}, ErrUserIDTooLong
	}
	return UserID{value: id}, nil
}

// ParseUserID is an alias for NewUserID for consistency with other value objects
func ParseUserID(id string) (UserID, error) {
	return NewUserID(id)
}

// CategoryID represents the unique identifier for a Category.
type CategoryID string

// ParseCategoryID parses a string into a CategoryID
func ParseCategoryID(id string) (CategoryID, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return CategoryID(""), ErrInvalidCategoryID
	}
	return CategoryID(id), nil
}

// String returns the string representation of the UserID
func (id UserID) String() string { 
	return id.value 
}

// Equals checks if two UserIDs are equal
func (id UserID) Equals(other UserID) bool { 
	return id.value == other.value 
}

// IsEmpty checks if the UserID is empty
func (id UserID) IsEmpty() bool {
	return id.value == ""
}

// Content is a value object with business rules for node content
type Content struct {
	value string
}

// NewContent creates a new Content value object with validation
func NewContent(value string) (Content, error) {
	value = strings.TrimSpace(value)
	
	if len(value) == 0 {
		return Content{}, ErrEmptyContent
	}
	
	if len(value) > MaxContentLength {
		return Content{}, ErrContentTooLong
	}
	
	if containsProfanity(value) {
		return Content{}, ErrInappropriateContent
	}
	
	return Content{value: value}, nil
}

// String returns the string representation of the content
func (c Content) String() string { 
	return c.value 
}

// WordCount returns the number of words in the content
func (c Content) WordCount() int { 
	return len(strings.Fields(c.value))
}

// Equals checks if two Content objects have the same value
func (c Content) Equals(other Content) bool {
	return c.value == other.value
}

// Validate checks if the content is still valid (for updates)
func (c Content) Validate() error {
	if len(c.value) == 0 {
		return ErrEmptyContent
	}
	if len(c.value) > MaxContentLength {
		return ErrContentTooLong
	}
	if containsProfanity(c.value) {
		return ErrInappropriateContent
	}
	return nil
}

// ExtractKeywords extracts meaningful keywords from the content
func (c Content) ExtractKeywords() Keywords {
	// Business logic for keyword extraction
	content := strings.ToLower(c.value)
	content = contentCleanupRegex.ReplaceAllString(content, "")
	words := strings.Fields(content)

	uniqueWords := make(map[string]bool)
	for _, word := range words {
		word = cleanWord(word)
		if isSignificantWord(word) {
			uniqueWords[word] = true
		}
	}
	
	return Keywords{words: uniqueWords}
}

// Keywords value object encapsulates keyword logic
type Keywords struct {
	words map[string]bool
}

// NewKeywords creates a Keywords value object from a slice of strings
func NewKeywords(words []string) Keywords {
	uniqueWords := make(map[string]bool)
	for _, word := range words {
		word = cleanWord(strings.ToLower(word))
		if isSignificantWord(word) {
			uniqueWords[word] = true
		}
	}
	return Keywords{words: uniqueWords}
}

// Contains checks if a word exists in the keywords
func (k Keywords) Contains(word string) bool {
	return k.words[strings.ToLower(word)]
}

// Overlap calculates the percentage of overlapping keywords with another Keywords object
func (k Keywords) Overlap(other Keywords) float64 {
	if len(k.words) == 0 || len(other.words) == 0 {
		return 0
	}
	
	overlap := 0
	for word := range k.words {
		if other.words[word] {
			overlap++
		}
	}
	
	return float64(overlap) / float64(len(k.words))
}

// ToSlice returns the keywords as a sorted slice
func (k Keywords) ToSlice() []string {
	result := make([]string, 0, len(k.words))
	for word := range k.words {
		result = append(result, word)
	}
	sort.Strings(result)
	return result
}

// Count returns the number of unique keywords
func (k Keywords) Count() int {
	return len(k.words)
}

// IsEmpty checks if there are no keywords
func (k Keywords) IsEmpty() bool {
	return len(k.words) == 0
}

// Tags value object for managing node tags
type Tags struct {
	tags map[string]bool
}

// NewTags creates a Tags value object from a slice of strings
func NewTags(tags ...string) Tags {
	normalized := make(map[string]bool)
	for _, tag := range tags {
		tag = normalizeTag(tag)
		if isValidTag(tag) {
			normalized[tag] = true
		}
	}
	return Tags{tags: normalized}
}

// Contains checks if a tag exists
func (t Tags) Contains(tag string) bool {
	return t.tags[normalizeTag(tag)]
}

// ToSlice returns the tags as a sorted slice
func (t Tags) ToSlice() []string {
	result := make([]string, 0, len(t.tags))
	for tag := range t.tags {
		result = append(result, tag)
	}
	sort.Strings(result)
	return result
}

// Add adds a new tag
func (t Tags) Add(tag string) Tags {
	tag = normalizeTag(tag)
	if !isValidTag(tag) {
		return t
	}
	
	newTags := make(map[string]bool)
	for existingTag := range t.tags {
		newTags[existingTag] = true
	}
	newTags[tag] = true
	
	return Tags{tags: newTags}
}

// Remove removes a tag
func (t Tags) Remove(tag string) Tags {
	tag = normalizeTag(tag)
	
	newTags := make(map[string]bool)
	for existingTag := range t.tags {
		if existingTag != tag {
			newTags[existingTag] = true
		}
	}
	
	return Tags{tags: newTags}
}

// Count returns the number of tags
func (t Tags) Count() int {
	return len(t.tags)
}

// IsEmpty checks if there are no tags
func (t Tags) IsEmpty() bool {
	return len(t.tags) == 0
}

// Overlap calculates the percentage of overlapping tags with another Tags object
func (t Tags) Overlap(other Tags) float64 {
	if len(t.tags) == 0 || len(other.tags) == 0 {
		return 0
	}
	
	overlap := 0
	for tag := range t.tags {
		if other.tags[tag] {
			overlap++
		}
	}
	
	return float64(overlap) / float64(len(t.tags))
}

// Version value object for optimistic locking
type Version struct {
	value int
}

// NewVersion creates a new Version starting at 0
func NewVersion() Version {
	return Version{value: 0}
}

// ParseVersion creates a Version from an integer
func ParseVersion(value int) Version {
	if value < 0 {
		value = 0
	}
	return Version{value: value}
}

// Int returns the integer value of the version
func (v Version) Int() int {
	return v.value
}

// Next returns the next version number
func (v Version) Next() Version {
	return Version{value: v.value + 1}
}

// Equals checks if two versions are equal
func (v Version) Equals(other Version) bool {
	return v.value == other.value
}

// Helper functions for value object validation and processing

// stopWords contains common words filtered out during keyword extraction
var stopWords = map[string]bool{
	"the": true, "a": true, "an": true,
	"and": true, "or": true, "but": true,
	"in": true, "on": true, "at": true, "to": true, "for": true, "of": true,
	"with": true, "by": true, "from": true, "up": true, "about": true,
	"into": true, "through": true, "during": true, "before": true, "after": true,
	"above": true, "below": true, "between": true, "under": true,
	"again": true, "further": true, "then": true, "once": true,
	"is": true, "am": true, "are": true, "was": true, "were": true,
	"be": true, "been": true, "being": true,
	"have": true, "has": true, "had": true, "do": true, "does": true, "did": true,
	"will": true, "would": true, "should": true, "could": true, "ought": true,
	"i": true, "me": true, "my": true, "myself": true,
	"we": true, "our": true, "ours": true, "ourselves": true,
	"you": true, "your": true, "yours": true, "yourself": true, "yourselves": true,
	"he": true, "him": true, "his": true, "himself": true,
	"she": true, "her": true, "hers": true, "herself": true,
	"it": true, "its": true, "itself": true,
	"they": true, "them": true, "their": true, "theirs": true, "themselves": true,
	"what": true, "which": true, "who": true, "whom": true,
	"this": true, "that": true, "these": true, "those": true,
	"as": true, "if": true, "each": true, "how": true, "than": true,
	"too": true, "very": true, "can": true, "just": true, "also": true,
}

// cleanWord removes unwanted characters and normalizes the word
func cleanWord(word string) string {
	word = strings.TrimSpace(strings.ToLower(word))
	// Remove punctuation and keep only alphanumeric
	return alphanumericOnlyRegex.ReplaceAllString(word, "")
}

// isSignificantWord checks if a word should be considered significant for keywords
func isSignificantWord(word string) bool {
	return len(word) > 2 && !stopWords[word]
}

// normalizeTag normalizes a tag string
func normalizeTag(tag string) string {
	tag = strings.TrimSpace(strings.ToLower(tag))
	// Replace spaces with hyphens, remove special characters
	tag = tagSpecialCharsRegex.ReplaceAllString(tag, "")
	tag = tagSpaceRegex.ReplaceAllString(tag, "-")
	return tag
}

// isValidTag checks if a tag is valid
func isValidTag(tag string) bool {
	return len(tag) > 0 && len(tag) <= MaxTagLength
}

// containsProfanity checks if content contains inappropriate material
// This is a simplified implementation - in practice, you'd use a proper profanity filter
func containsProfanity(content string) bool {
	// Simple profanity check - in practice, use a proper library
	profanityWords := []string{"spam", "badword"} // Simplified list
	content = strings.ToLower(content)
	
	for _, word := range profanityWords {
		if strings.Contains(content, word) {
			return true
		}
	}
	return false
}

// Constants for validation
const (
	MaxTagLength     = 50
	MaxUserIDLength  = 100
)