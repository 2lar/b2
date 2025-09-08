package validators

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"

	"backend2/domain/core/valueobjects"
	"backend2/pkg/errors"
)

// NodeValidator validates node-related domain rules
type NodeValidator struct {
	titleMinLength   int
	titleMaxLength   int
	contentMaxLength int
	urlPattern       *regexp.Regexp
	tagMaxLength     int
	maxTags          int
	forbiddenWords   []string
}

// NewNodeValidator creates a new node validator with default rules
func NewNodeValidator() *NodeValidator {
	return &NodeValidator{
		titleMinLength:   1,
		titleMaxLength:   255,
		contentMaxLength: 50000,
		urlPattern:       regexp.MustCompile(`^https?://[^\s]+$`),
		tagMaxLength:     50,
		maxTags:          20,
		forbiddenWords:   []string{}, // Can be configured with inappropriate content filters
	}
}

// ValidateNodeContent validates the content value object
func (v *NodeValidator) ValidateNodeContent(content *valueobjects.NodeContent) error {
	validationErrors := errors.NewValidationErrors()

	// Validate title
	if err := v.validateTitle(content.Title()); err != nil {
		if domainErr, ok := err.(*errors.DomainError); ok {
			validationErrors.AddError(domainErr)
		} else {
			validationErrors.Add("title", err.Error())
		}
	}

	// Validate content body
	if err := v.validateContentBody(content.Body()); err != nil {
		if domainErr, ok := err.(*errors.DomainError); ok {
			validationErrors.AddError(domainErr)
		} else {
			validationErrors.Add("body", err.Error())
		}
	}

	// Validate URL if present (URLs are not part of NodeContent in this implementation)
	// URL validation can be done separately if needed

	// Validate format
	if err := v.validateFormat(content.Format()); err != nil {
		if domainErr, ok := err.(*errors.DomainError); ok {
			validationErrors.AddError(domainErr)
		} else {
			validationErrors.Add("format", err.Error())
		}
	}

	if validationErrors.HasErrors() {
		return validationErrors
	}

	return nil
}

// validateTitle validates the node title
func (v *NodeValidator) validateTitle(title string) error {
	title = strings.TrimSpace(title)

	if len(title) < v.titleMinLength {
		return errors.ErrNodeTitleRequired
	}

	if len(title) > v.titleMaxLength {
		return errors.ErrNodeTitleTooLong.
			WithDetail("actual_length", len(title)).
			WithDetail("max_length", v.titleMaxLength)
	}

	// Check for forbidden words
	if v.containsForbiddenWords(title) {
		return errors.NewDomainError(
			errors.DomainValidationError,
			"INAPPROPRIATE_CONTENT",
			"Title contains inappropriate content",
		).WithDetail("field", "title")
	}

	return nil
}

// validateContentBody validates the node content
func (v *NodeValidator) validateContentBody(content string) error {
	if len(content) > v.contentMaxLength {
		return errors.ErrNodeContentTooLong.
			WithDetail("actual_length", len(content)).
			WithDetail("max_length", v.contentMaxLength)
	}

	// Check for potentially malicious content
	if strings.Contains(content, "<script>") || strings.Contains(content, "javascript:") {
		return errors.NewDomainError(
			errors.DomainValidationError,
			"MALICIOUS_CONTENT",
			"Content contains potentially malicious code",
		).WithDetail("field", "content")
	}

	// Check for forbidden words
	if v.containsForbiddenWords(content) {
		return errors.NewDomainError(
			errors.DomainValidationError,
			"INAPPROPRIATE_CONTENT",
			"Content contains inappropriate material",
		).WithDetail("field", "content")
	}

	return nil
}

// validateURL validates a URL
func (v *NodeValidator) validateURL(urlStr string) error {
	if urlStr == "" {
		return nil // URL is optional
	}

	// Parse URL
	parsedURL, err := url.Parse(urlStr)
	if err != nil {
		return errors.NewDomainError(
			errors.DomainValidationError,
			"INVALID_URL_FORMAT",
			"Invalid URL format",
		).WithDetail("field", "url").WithCause(err)
	}

	// Check scheme
	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return errors.NewDomainError(
			errors.DomainValidationError,
			"INVALID_URL_SCHEME",
			"URL must use http or https scheme",
		).WithDetail("field", "url").WithDetail("scheme", parsedURL.Scheme)
	}

	// Check host
	if parsedURL.Host == "" {
		return errors.NewDomainError(
			errors.DomainValidationError,
			"INVALID_URL_HOST",
			"URL must have a valid host",
		).WithDetail("field", "url")
	}

	return nil
}

// validateFormat validates the content format
func (v *NodeValidator) validateFormat(format valueobjects.ContentFormat) error {
	validFormats := []valueobjects.ContentFormat{
		valueobjects.FormatPlainText,
		valueobjects.FormatMarkdown,
		valueobjects.FormatHTML,
		valueobjects.FormatJSON,
	}

	for _, valid := range validFormats {
		if format == valid {
			return nil
		}
	}

	return errors.NewDomainError(
		errors.DomainValidationError,
		"INVALID_CONTENT_FORMAT",
		"Invalid content format",
	).WithDetail("field", "format").WithDetail("value", string(format))
}

// ValidateTags validates a list of tags
func (v *NodeValidator) ValidateTags(tags []string) error {
	if len(tags) > v.maxTags {
		return errors.NewDomainError(
			errors.DomainValidationError,
			"TOO_MANY_TAGS",
			fmt.Sprintf("Cannot have more than %d tags", v.maxTags),
		).WithDetail("field", "tags").WithDetail("count", len(tags))
	}

	for _, tag := range tags {
		if err := v.validateTag(tag); err != nil {
			return err
		}
	}

	return nil
}

// validateTag validates a single tag
func (v *NodeValidator) validateTag(tag string) error {
	tag = strings.TrimSpace(tag)

	if tag == "" {
		return errors.NewDomainError(
			errors.DomainValidationError,
			"EMPTY_TAG",
			"Tag cannot be empty",
		).WithDetail("field", "tags")
	}

	if len(tag) > v.tagMaxLength {
		return errors.NewDomainError(
			errors.DomainValidationError,
			"TAG_TOO_LONG",
			fmt.Sprintf("Tag exceeds maximum length of %d characters", v.tagMaxLength),
		).WithDetail("field", "tags").WithDetail("tag", tag)
	}

	// Check for valid characters (alphanumeric, dash, underscore)
	validTagPattern := regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)
	if !validTagPattern.MatchString(tag) {
		return errors.NewDomainError(
			errors.DomainValidationError,
			"INVALID_TAG_FORMAT",
			"Tag contains invalid characters",
		).WithDetail("field", "tags").WithDetail("tag", tag)
	}

	return nil
}

// ValidatePosition validates node position coordinates
func (v *NodeValidator) ValidatePosition(x, y, z float64) error {
	const maxCoordinate = 10000.0
	const minCoordinate = -10000.0

	if x < minCoordinate || x > maxCoordinate ||
		y < minCoordinate || y > maxCoordinate ||
		z < minCoordinate || z > maxCoordinate {
		return errors.ErrInvalidNodePosition.
			WithDetail("x", x).
			WithDetail("y", y).
			WithDetail("z", z).
			WithDetail("min", minCoordinate).
			WithDetail("max", maxCoordinate)
	}

	return nil
}

// ValidateMetadata validates node metadata
func (v *NodeValidator) ValidateMetadata(metadata map[string]interface{}) error {
	const maxMetadataKeys = 50
	const maxKeyLength = 100
	const maxValueLength = 1000

	if len(metadata) > maxMetadataKeys {
		return errors.NewDomainError(
			errors.DomainValidationError,
			"TOO_MANY_METADATA_KEYS",
			fmt.Sprintf("Cannot have more than %d metadata keys", maxMetadataKeys),
		).WithDetail("field", "metadata").WithDetail("count", len(metadata))
	}

	for key, value := range metadata {
		// Validate key
		if len(key) > maxKeyLength {
			return errors.NewDomainError(
				errors.DomainValidationError,
				"METADATA_KEY_TOO_LONG",
				fmt.Sprintf("Metadata key '%s' exceeds maximum length of %d", key, maxKeyLength),
			).WithDetail("field", "metadata").WithDetail("key", key)
		}

		// Validate value based on type
		switch v := value.(type) {
		case string:
			if len(v) > maxValueLength {
				return errors.NewDomainError(
					errors.DomainValidationError,
					"METADATA_VALUE_TOO_LONG",
					fmt.Sprintf("Metadata value for '%s' exceeds maximum length of %d", key, maxValueLength),
				).WithDetail("field", "metadata").WithDetail("key", key)
			}
		case []interface{}:
			if len(v) > 100 {
				return errors.NewDomainError(
					errors.DomainValidationError,
					"METADATA_ARRAY_TOO_LARGE",
					fmt.Sprintf("Metadata array for '%s' exceeds maximum size of 100", key),
				).WithDetail("field", "metadata").WithDetail("key", key)
			}
		case map[string]interface{}:
			if len(v) > 50 {
				return errors.NewDomainError(
					errors.DomainValidationError,
					"METADATA_OBJECT_TOO_LARGE",
					fmt.Sprintf("Metadata object for '%s' exceeds maximum size of 50 properties", key),
				).WithDetail("field", "metadata").WithDetail("key", key)
			}
		}
	}

	return nil
}

// containsForbiddenWords checks if text contains forbidden words
func (v *NodeValidator) containsForbiddenWords(text string) bool {
	lowerText := strings.ToLower(text)
	for _, word := range v.forbiddenWords {
		if strings.Contains(lowerText, strings.ToLower(word)) {
			return true
		}
	}
	return false
}

// GraphValidator validates graph-related domain rules
type GraphValidator struct {
	nameMinLength     int
	nameMaxLength     int
	descMaxLength     int
	maxNodesPerGraph  int
	maxEdgesPerGraph  int
	maxGraphsPerUser  int
}

// NewGraphValidator creates a new graph validator
func NewGraphValidator() *GraphValidator {
	return &GraphValidator{
		nameMinLength:     1,
		nameMaxLength:     255,
		descMaxLength:     1000,
		maxNodesPerGraph:  10000,
		maxEdgesPerGraph:  50000,
		maxGraphsPerUser:  100,
	}
}

// ValidateGraphName validates the graph name
func (v *GraphValidator) ValidateGraphName(name string) error {
	name = strings.TrimSpace(name)

	if len(name) < v.nameMinLength {
		return errors.ErrGraphNameRequired
	}

	if len(name) > v.nameMaxLength {
		return errors.NewDomainError(
			errors.DomainValidationError,
			"GRAPH_NAME_TOO_LONG",
			"Graph name exceeds maximum length",
		).WithDetail("max_length", v.nameMaxLength)
	}

	return nil
}

// ValidateGraphDescription validates the graph description
func (v *GraphValidator) ValidateGraphDescription(desc string) error {
	if len(desc) > v.descMaxLength {
		return errors.NewDomainError(
			errors.DomainValidationError,
			"GRAPH_DESCRIPTION_TOO_LONG",
			"Graph description exceeds maximum length",
		).WithDetail("max_length", v.descMaxLength)
	}

	return nil
}

// ValidateNodeCount validates the number of nodes in a graph
func (v *GraphValidator) ValidateNodeCount(count int) error {
	if count > v.maxNodesPerGraph {
		return errors.ErrGraphLimitExceeded.
			WithDetail("current", count).
			WithDetail("limit", v.maxNodesPerGraph)
	}

	return nil
}

// ValidateEdgeCount validates the number of edges in a graph
func (v *GraphValidator) ValidateEdgeCount(count int) error {
	if count > v.maxEdgesPerGraph {
		return errors.NewDomainError(
			errors.DomainBusinessRuleError,
			"EDGE_LIMIT_EXCEEDED",
			"Maximum number of edges in graph exceeded",
		).WithDetail("current", count).WithDetail("limit", v.maxEdgesPerGraph)
	}

	return nil
}

// EdgeValidator validates edge-related domain rules
type EdgeValidator struct{}

// NewEdgeValidator creates a new edge validator
func NewEdgeValidator() *EdgeValidator {
	return &EdgeValidator{}
}

// ValidateEdge validates an edge creation
func (v *EdgeValidator) ValidateEdge(sourceID, targetID string) error {
	// Check for self-referential edge
	if sourceID == targetID {
		return errors.ErrSelfReferentialEdge.
			WithDetail("node_id", sourceID)
	}

	// Additional validation can be added here
	// - Check for cyclic dependencies
	// - Validate edge types
	// - Check edge weight ranges

	return nil
}

// ValidateEdgeWeight validates the edge weight
func (v *EdgeValidator) ValidateEdgeWeight(weight float64) error {
	if weight < 0 || weight > 1 {
		return errors.NewDomainError(
			errors.DomainValidationError,
			"INVALID_EDGE_WEIGHT",
			"Edge weight must be between 0 and 1",
		).WithDetail("weight", weight)
	}

	return nil
}