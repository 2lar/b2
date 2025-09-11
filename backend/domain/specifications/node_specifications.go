package specifications

import (
	"strings"

	"backend/domain/core/entities"
	"backend/domain/core/valueobjects"
)

// NodeSpecification is a specification for Node entities
type NodeSpecification interface {
	Specification[*entities.Node]
}

// NodeContentLengthSpec validates node content length
type NodeContentLengthSpec struct {
	BaseSpecification[*entities.Node]
	minLength int
	maxLength int
}

// NewNodeContentLengthSpec creates a specification for node content length
func NewNodeContentLengthSpec(minLength, maxLength int) *NodeContentLengthSpec {
	spec := &NodeContentLengthSpec{
		minLength: minLength,
		maxLength: maxLength,
	}
	spec.BaseSpecification = BaseSpecification[*entities.Node]{
		evaluator: spec.evaluate,
	}
	return spec
}

func (s *NodeContentLengthSpec) evaluate(node *entities.Node) bool {
	if node == nil {
		return false
	}
	content := node.Content()
	totalLength := len(content.Title()) + len(content.Body())
	return totalLength >= s.minLength && totalLength <= s.maxLength
}

// NodeHasTagsSpec validates that a node has specific tags
type NodeHasTagsSpec struct {
	BaseSpecification[*entities.Node]
	requiredTags []string
	matchAll     bool // If true, all tags must be present. If false, at least one.
}

// NewNodeHasTagsSpec creates a specification for node tags
func NewNodeHasTagsSpec(requiredTags []string, matchAll bool) *NodeHasTagsSpec {
	spec := &NodeHasTagsSpec{
		requiredTags: requiredTags,
		matchAll:     matchAll,
	}
	spec.BaseSpecification = BaseSpecification[*entities.Node]{
		evaluator: spec.evaluate,
	}
	return spec
}

func (s *NodeHasTagsSpec) evaluate(node *entities.Node) bool {
	if node == nil {
		return false
	}

	nodeTags := node.GetTags()
	tagSet := make(map[string]bool)
	for _, tag := range nodeTags {
		tagSet[strings.ToLower(tag)] = true
	}

	if s.matchAll {
		// All required tags must be present
		for _, required := range s.requiredTags {
			if !tagSet[strings.ToLower(required)] {
				return false
			}
		}
		return true
	} else {
		// At least one required tag must be present
		for _, required := range s.requiredTags {
			if tagSet[strings.ToLower(required)] {
				return true
			}
		}
		return false
	}
}

// NodeStatusSpec validates node status
type NodeStatusSpec struct {
	BaseSpecification[*entities.Node]
	allowedStatuses []entities.NodeStatus
}

// NewNodeStatusSpec creates a specification for node status
func NewNodeStatusSpec(allowedStatuses ...entities.NodeStatus) *NodeStatusSpec {
	spec := &NodeStatusSpec{
		allowedStatuses: allowedStatuses,
	}
	spec.BaseSpecification = BaseSpecification[*entities.Node]{
		evaluator: spec.evaluate,
	}
	return spec
}

func (s *NodeStatusSpec) evaluate(node *entities.Node) bool {
	if node == nil {
		return false
	}

	nodeStatus := node.Status()
	for _, allowed := range s.allowedStatuses {
		if nodeStatus == allowed {
			return true
		}
	}
	return false
}

// NodePositionBoundsSpec validates node position is within bounds
type NodePositionBoundsSpec struct {
	BaseSpecification[*entities.Node]
	minX, maxX float64
	minY, maxY float64
	minZ, maxZ float64
}

// NewNodePositionBoundsSpec creates a specification for node position bounds
func NewNodePositionBoundsSpec(minX, maxX, minY, maxY, minZ, maxZ float64) *NodePositionBoundsSpec {
	spec := &NodePositionBoundsSpec{
		minX: minX, maxX: maxX,
		minY: minY, maxY: maxY,
		minZ: minZ, maxZ: maxZ,
	}
	spec.BaseSpecification = BaseSpecification[*entities.Node]{
		evaluator: spec.evaluate,
	}
	return spec
}

func (s *NodePositionBoundsSpec) evaluate(node *entities.Node) bool {
	if node == nil {
		return false
	}

	pos := node.Position()
	return pos.X() >= s.minX && pos.X() <= s.maxX &&
		pos.Y() >= s.minY && pos.Y() <= s.maxY &&
		pos.Z() >= s.minZ && pos.Z() <= s.maxZ
}

// NodeContentFormatSpec validates node content format
type NodeContentFormatSpec struct {
	BaseSpecification[*entities.Node]
	allowedFormats []valueobjects.ContentFormat
}

// NewNodeContentFormatSpec creates a specification for node content format
func NewNodeContentFormatSpec(allowedFormats ...valueobjects.ContentFormat) *NodeContentFormatSpec {
	spec := &NodeContentFormatSpec{
		allowedFormats: allowedFormats,
	}
	spec.BaseSpecification = BaseSpecification[*entities.Node]{
		evaluator: spec.evaluate,
	}
	return spec
}

func (s *NodeContentFormatSpec) evaluate(node *entities.Node) bool {
	if node == nil {
		return false
	}

	content := node.Content()
	for _, allowed := range s.allowedFormats {
		if content.Format() == allowed {
			return true
		}
	}
	return false
}

// NodeHasContentSpec validates that a node has non-empty content
type NodeHasContentSpec struct {
	BaseSpecification[*entities.Node]
	requireTitle bool
	requireBody  bool
}

// NewNodeHasContentSpec creates a specification for node content presence
func NewNodeHasContentSpec(requireTitle, requireBody bool) *NodeHasContentSpec {
	spec := &NodeHasContentSpec{
		requireTitle: requireTitle,
		requireBody:  requireBody,
	}
	spec.BaseSpecification = BaseSpecification[*entities.Node]{
		evaluator: spec.evaluate,
	}
	return spec
}

func (s *NodeHasContentSpec) evaluate(node *entities.Node) bool {
	if node == nil {
		return false
	}

	content := node.Content()
	
	if s.requireTitle && strings.TrimSpace(content.Title()) == "" {
		return false
	}
	
	if s.requireBody && strings.TrimSpace(content.Body()) == "" {
		return false
	}
	
	return true
}

// NodeTagCountSpec validates the number of tags on a node
type NodeTagCountSpec struct {
	BaseSpecification[*entities.Node]
	minTags int
	maxTags int
}

// NewNodeTagCountSpec creates a specification for node tag count
func NewNodeTagCountSpec(minTags, maxTags int) *NodeTagCountSpec {
	spec := &NodeTagCountSpec{
		minTags: minTags,
		maxTags: maxTags,
	}
	spec.BaseSpecification = BaseSpecification[*entities.Node]{
		evaluator: spec.evaluate,
	}
	return spec
}

func (s *NodeTagCountSpec) evaluate(node *entities.Node) bool {
	if node == nil {
		return false
	}

	tagCount := len(node.GetTags())
	return tagCount >= s.minTags && tagCount <= s.maxTags
}

// NodeMetadataSpec validates node metadata
type NodeMetadataSpec struct {
	BaseSpecification[*entities.Node]
	requiredKeys []string
	validator    func(metadata map[string]interface{}) bool
}

// NewNodeMetadataSpec creates a specification for node metadata
func NewNodeMetadataSpec(requiredKeys []string, validator func(map[string]interface{}) bool) *NodeMetadataSpec {
	spec := &NodeMetadataSpec{
		requiredKeys: requiredKeys,
		validator:    validator,
	}
	spec.BaseSpecification = BaseSpecification[*entities.Node]{
		evaluator: spec.evaluate,
	}
	return spec
}

func (s *NodeMetadataSpec) evaluate(node *entities.Node) bool {
	if node == nil {
		return false
	}

	metadata := node.GetMetadata()
	
	// Check required keys
	for _, key := range s.requiredKeys {
		if _, exists := metadata[key]; !exists {
			return false
		}
	}
	
	// Run custom validator if provided
	if s.validator != nil {
		return s.validator(metadata)
	}
	
	return true
}

// Common pre-configured specifications

// NewValidNodeSpec creates a specification for a valid node
func NewValidNodeSpec() NodeSpecification {
	return NewNodeHasContentSpec(true, false).
		And(NewNodeContentLengthSpec(1, 50000)).
		And(NewNodePositionBoundsSpec(-10000, 10000, -10000, 10000, -10000, 10000)).
		And(NewNodeTagCountSpec(0, 20))
}

// NewActiveNodeSpec creates a specification for active nodes
func NewActiveNodeSpec() NodeSpecification {
	return NewNodeStatusSpec(entities.StatusPublished, entities.StatusDraft)
}

// NewArchivedNodeSpec creates a specification for archived nodes
func NewArchivedNodeSpec() NodeSpecification {
	return NewNodeStatusSpec(entities.StatusArchived)
}

// NewSearchableNodeSpec creates a specification for searchable nodes
func NewSearchableNodeSpec() NodeSpecification {
	// Nodes must be active and have content
	return NewActiveNodeSpec().
		And(NewNodeHasContentSpec(true, true))
}