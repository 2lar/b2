// Package assertions provides custom test assertions for domain objects
package assertions

import (
	"testing"

	"brain2-backend/internal/core/domain/aggregates/node"
	"brain2-backend/internal/core/domain/valueobjects"
	
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// DomainAssertions provides assertions for domain objects
type DomainAssertions struct {
	t *testing.T
}

// NewDomainAssertions creates new domain assertions
func NewDomainAssertions(t *testing.T) *DomainAssertions {
	return &DomainAssertions{t: t}
}

// AssertNodeEqual asserts that two nodes are equal
func (a *DomainAssertions) AssertNodeEqual(expected, actual *node.Aggregate, msgAndArgs ...interface{}) {
	require.NotNil(a.t, expected, "Expected node should not be nil")
	require.NotNil(a.t, actual, "Actual node should not be nil")
	
	assert.Equal(a.t, expected.GetID(), actual.GetID(), append([]interface{}{"Node IDs should match"}, msgAndArgs...)...)
	assert.Equal(a.t, expected.GetVersion(), actual.GetVersion(), append([]interface{}{"Node versions should match"}, msgAndArgs...)...)
	
	// Compare content and title
	assert.Equal(a.t, expected.GetContent(), actual.GetContent(), append([]interface{}{"Node content should match"}, msgAndArgs...)...)
	assert.Equal(a.t, expected.GetTitle(), actual.GetTitle(), append([]interface{}{"Node title should match"}, msgAndArgs...)...)
}

// AssertNodeHasContent asserts that a node has specific content
func (a *DomainAssertions) AssertNodeHasContent(node *node.Aggregate, content string, msgAndArgs ...interface{}) {
	require.NotNil(a.t, node, "Node should not be nil")
	
	// Check the actual content from the node
	nodeContent := node.GetContent()
	assert.Equal(a.t, content, nodeContent, append([]interface{}{"Node should have content: " + content}, msgAndArgs...)...)
}

// AssertNodeHasTags asserts that a node has specific tags
func (a *DomainAssertions) AssertNodeHasTags(node *node.Aggregate, tags []string, msgAndArgs ...interface{}) {
	require.NotNil(a.t, node, "Node should not be nil")
	
	// Check tags directly from the node
	nodeTags := node.GetTags()
	found := equalStringSlices(nodeTags, tags)
	
	assert.True(a.t, found, append([]interface{}{"Node should have tags"}, msgAndArgs...)...)
}

// AssertNodeIsArchived asserts that a node is archived
func (a *DomainAssertions) AssertNodeIsArchived(node *node.Aggregate, msgAndArgs ...interface{}) {
	require.NotNil(a.t, node, "Node should not be nil")
	
	// Check if node is archived directly
	archived := node.IsArchived()
	
	assert.True(a.t, archived, append([]interface{}{"Node should be archived"}, msgAndArgs...)...)
}

// AssertNodeIsActive asserts that a node is active (not archived)
func (a *DomainAssertions) AssertNodeIsActive(node *node.Aggregate, msgAndArgs ...interface{}) {
	require.NotNil(a.t, node, "Node should not be nil")
	
	// Check if node is active (not archived) directly
	archived := node.IsArchived()
	
	assert.False(a.t, archived, append([]interface{}{"Node should be active"}, msgAndArgs...)...)
}

// AssertNodeVersion asserts that a node has a specific version
func (a *DomainAssertions) AssertNodeVersion(node *node.Aggregate, version int64, msgAndArgs ...interface{}) {
	require.NotNil(a.t, node, "Node should not be nil")
	assert.Equal(a.t, version, node.GetVersion(), append([]interface{}{"Node version should match"}, msgAndArgs...)...)
}

// AssertValueObjectEqual asserts that two value objects are equal
func (a *DomainAssertions) AssertValueObjectEqual(expected, actual interface{}, msgAndArgs ...interface{}) {
	assert.Equal(a.t, expected, actual, msgAndArgs...)
}

// AssertNodeIDValid asserts that a node ID is valid
func (a *DomainAssertions) AssertNodeIDValid(id *valueobjects.NodeID, msgAndArgs ...interface{}) {
	require.NotNil(a.t, id, "NodeID should not be nil")
	assert.NotEmpty(a.t, id.String(), append([]interface{}{"NodeID should not be empty"}, msgAndArgs...)...)
}

// AssertUserIDValid asserts that a user ID is valid
func (a *DomainAssertions) AssertUserIDValid(id *valueobjects.UserID, msgAndArgs ...interface{}) {
	require.NotNil(a.t, id, "UserID should not be nil")
	assert.NotEmpty(a.t, id.String(), append([]interface{}{"UserID should not be empty"}, msgAndArgs...)...)
}

// AssertContentValid asserts that content is valid
func (a *DomainAssertions) AssertContentValid(content *valueobjects.Content, msgAndArgs ...interface{}) {
	require.NotNil(a.t, content, "Content should not be nil")
	assert.NotEmpty(a.t, content.String(), append([]interface{}{"Content should not be empty"}, msgAndArgs...)...)
	assert.LessOrEqual(a.t, len(content.String()), 10000, append([]interface{}{"Content should not exceed max length"}, msgAndArgs...)...)
}

// AssertTitleValid asserts that a title is valid
func (a *DomainAssertions) AssertTitleValid(title *valueobjects.Title, msgAndArgs ...interface{}) {
	require.NotNil(a.t, title, "Title should not be nil")
	assert.LessOrEqual(a.t, len(title.String()), 200, append([]interface{}{"Title should not exceed max length"}, msgAndArgs...)...)
}

// AssertTagsValid asserts that tags are valid
func (a *DomainAssertions) AssertTagsValid(tags *valueobjects.Tags, msgAndArgs ...interface{}) {
	require.NotNil(a.t, tags, "Tags should not be nil")
	assert.LessOrEqual(a.t, len(tags.Values()), 20, append([]interface{}{"Should not exceed max number of tags"}, msgAndArgs...)...)
	
	for _, tag := range tags.Values() {
		assert.NotEmpty(a.t, tag, append([]interface{}{"Tag should not be empty"}, msgAndArgs...)...)
		assert.LessOrEqual(a.t, len(tag), 50, append([]interface{}{"Tag should not exceed max length"}, msgAndArgs...)...)
	}
}

// Helper functions

func equalStringSlices(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}