# B2 Enhanced Organization Implementation Plan

## Overview

This plan details the implementation of hierarchical categories with AI-powered automatic categorization in B2's serverless architecture, adapting the best features from B2v1.

## Architecture Design

### Core Features to Implement
1. **Hierarchical Categories** - Multi-level category structure (General → Specific → More Specific)
2. **AI-Powered Categorization** - Automatic categorization using LLM providers
3. **Dynamic Category Creation** - Categories created and evolved based on content
4. **Smart Category Merging** - Detect and merge similar categories
5. **Category-based Navigation** - Filter and browse notes by category

## Phase 1: DynamoDB Schema Design

### 1.1 New Entity Types

```typescript
// Category Entity
{
  PK: "USER#{userId}",
  SK: "CAT#{categoryId}",
  Type: "Category",
  CategoryID: string,
  Name: string,
  Level: number, // 0 = top level, 1 = sub, 2 = sub-sub
  Description?: string,
  Color?: string,
  Icon?: string,
  AIGenerated: boolean,
  NoteCount: number,
  CreatedAt: string,
  UpdatedAt: string,
  // For GSI
  GSI1PK: "CAT_LEVEL#{level}",
  GSI1SK: "CAT#{categoryId}"
}

// Category Hierarchy
{
  PK: "USER#{userId}",
  SK: "HIERARCHY#PARENT#{parentId}#CHILD#{childId}",
  Type: "CategoryHierarchy",
  ParentID: string,
  ChildID: string,
  CreatedAt: string
}

// Node-Category Mapping
{
  PK: "USER#{userId}",
  SK: "NODE_CAT#NODE#{nodeId}#CAT#{categoryId}",
  Type: "NodeCategory",
  NodeID: string,
  CategoryID: string,
  Confidence: number, // AI confidence score
  Method: "ai" | "manual" | "rule-based",
  CreatedAt: string,
  // For GSI
  GSI1PK: "CAT#{categoryId}",
  GSI1SK: "NODE#{nodeId}"
}

// Category Similarity Cache (for performance)
{
  PK: "USER#{userId}",
  SK: "CAT_SIM#CAT1#{categoryId1}#CAT2#{categoryId2}",
  Type: "CategorySimilarity",
  Category1ID: string,
  Category2ID: string,
  Similarity: number,
  LastCalculated: string,
  TTL: number // Auto-expire after 30 days
}
```

### 1.2 GSI Design

```typescript
// GSI for finding nodes by category
GSI1: {
  PK: "CAT#{categoryId}",
  SK: "NODE#{nodeId}"
}

// GSI for category levels
GSI2: {
  PK: "CAT_LEVEL#{level}",
  SK: "CAT#{categoryId}"
}
```

## Phase 2: Backend Implementation

### 2.1 Domain Models

```go
// backend/internal/domain/category.go
package domain

type Category struct {
    ID          string   `json:"id"`
    UserID      string   `json:"userId"`
    Name        string   `json:"name"`
    Level       int      `json:"level"`
    Description string   `json:"description,omitempty"`
    Color       string   `json:"color,omitempty"`
    Icon        string   `json:"icon,omitempty"`
    AIGenerated bool     `json:"aiGenerated"`
    NoteCount   int      `json:"noteCount"`
    ParentID    string   `json:"parentId,omitempty"`
    ChildIDs    []string `json:"childIds,omitempty"`
    CreatedAt   string   `json:"createdAt"`
    UpdatedAt   string   `json:"updatedAt"`
}

type CategoryHierarchy struct {
    UserID    string `json:"userId"`
    ParentID  string `json:"parentId"`
    ChildID   string `json:"childId"`
    CreatedAt string `json:"createdAt"`
}

type NodeCategory struct {
    UserID     string  `json:"userId"`
    NodeID     string  `json:"nodeId"`
    CategoryID string  `json:"categoryId"`
    Confidence float64 `json:"confidence"`
    Method     string  `json:"method"` // "ai", "manual", "rule-based"
    CreatedAt  string  `json:"createdAt"`
}

type CategorySuggestion struct {
    Name       string  `json:"name"`
    Level      int     `json:"level"`
    Confidence float64 `json:"confidence"`
    Reason     string  `json:"reason"`
}
```

### 2.2 Repository Layer Updates

```go
// backend/internal/repository/repository.go
type Repository interface {
    // ... existing methods ...
    
    // Category operations
    CreateCategory(ctx context.Context, category domain.Category) error
    UpdateCategory(ctx context.Context, category domain.Category) error
    DeleteCategory(ctx context.Context, userID, categoryID string) error
    FindCategoryByID(ctx context.Context, userID, categoryID string) (*domain.Category, error)
    FindCategories(ctx context.Context, query CategoryQuery) ([]domain.Category, error)
    FindCategoriesByLevel(ctx context.Context, userID string, level int) ([]domain.Category, error)
    
    // Hierarchy operations
    CreateCategoryHierarchy(ctx context.Context, hierarchy domain.CategoryHierarchy) error
    DeleteCategoryHierarchy(ctx context.Context, userID, parentID, childID string) error
    FindChildCategories(ctx context.Context, userID, parentID string) ([]domain.Category, error)
    FindParentCategory(ctx context.Context, userID, childID string) (*domain.Category, error)
    GetCategoryTree(ctx context.Context, userID string) ([]domain.Category, error)
    
    // Node-Category operations
    AssignNodeToCategory(ctx context.Context, mapping domain.NodeCategory) error
    RemoveNodeFromCategory(ctx context.Context, userID, nodeID, categoryID string) error
    FindNodesByCategory(ctx context.Context, userID, categoryID string) ([]domain.Node, error)
    FindCategoriesForNode(ctx context.Context, userID, nodeID string) ([]domain.Category, error)
    
    // Batch operations for performance
    BatchAssignCategories(ctx context.Context, mappings []domain.NodeCategory) error
    UpdateCategoryNoteCounts(ctx context.Context, userID string, categoryCounts map[string]int) error
}
```

### 2.3 Category Service

```go
// backend/internal/service/category/service.go
package category

import (
    "context"
    "brain2-backend/internal/domain"
    "brain2-backend/internal/repository"
    "brain2-backend/internal/service/llm"
)

type Service struct {
    repo     repository.Repository
    llmSvc   *llm.Service
    keywords *keyword.Extractor
}

func (s *Service) CategorizeNode(ctx context.Context, node domain.Node) ([]domain.Category, error) {
    // 1. Extract keywords
    keywords := s.keywords.Extract(node.Content)
    
    // 2. Find existing categories that match
    existingCategories := s.findMatchingCategories(ctx, node.UserID, keywords)
    
    // 3. If LLM is available, get AI suggestions
    var aiCategories []domain.CategorySuggestion
    if s.llmSvc.IsAvailable() {
        aiCategories, _ = s.llmSvc.SuggestCategories(ctx, node.Content, existingCategories)
    }
    
    // 4. Merge and deduplicate suggestions
    finalCategories := s.mergeCategories(existingCategories, aiCategories)
    
    // 5. Create new categories if needed
    for _, cat := range finalCategories {
        if cat.ID == "" {
            // This is a new category
            newCat := s.createCategory(ctx, node.UserID, cat)
            cat.ID = newCat.ID
        }
    }
    
    // 6. Assign node to categories
    s.assignNodeToCategories(ctx, node, finalCategories)
    
    return finalCategories, nil
}

func (s *Service) BuildCategoryHierarchy(ctx context.Context, categories []domain.Category) error {
    // Analyze categories and create parent-child relationships
    // This can use LLM or rule-based logic
}

func (s *Service) MergeSimilarCategories(ctx context.Context, userID string, threshold float64) error {
    // Find and merge categories that are too similar
}
```

### 2.4 LLM Integration for Categories

```go
// backend/internal/service/llm/categorizer.go
package llm

type Categorizer struct {
    provider Provider
}

func (c *Categorizer) SuggestCategories(
    ctx context.Context, 
    content string, 
    existingCategories []domain.Category,
) ([]domain.CategorySuggestion, error) {
    prompt := c.buildCategorizationPrompt(content, existingCategories)
    
    response, err := c.provider.Complete(ctx, prompt, CompletionOptions{
        Temperature: 0.5, // Lower for more consistent categorization
        MaxTokens:   300,
        Format:      "json",
    })
    
    if err != nil {
        return nil, err
    }
    
    return c.parseCategorizationResponse(response)
}

func (c *Categorizer) buildCategorizationPrompt(content string, existing []domain.Category) string {
    return fmt.Sprintf(`
You are an expert content categorizer. Analyze the following text and suggest 1-3 hierarchical categories.

Existing categories in the system:
%s

Text to categorize:
%s

Return a JSON array with this structure:
[
  {"name": "General Category", "level": 0, "confidence": 0.9, "reason": "why this category"},
  {"name": "Specific Category", "level": 1, "confidence": 0.8, "reason": "why this category"},
  {"name": "More Specific", "level": 2, "confidence": 0.7, "reason": "why this category"}
]

Rules:
1. Prefer existing categories when appropriate
2. Suggest new categories only when content doesn't fit existing ones
3. Keep category names concise (2-3 words)
4. Ensure hierarchy makes sense (general → specific)
`, formatExistingCategories(existing), content)
}
```

## Phase 3: Lambda Functions

### 3.1 Category Lambda

```go
// backend/cmd/category/main.go
package main

// Handles category CRUD operations and categorization
func handler(ctx context.Context, request events.APIGatewayV2HTTPRequest) (events.APIGatewayV2HTTPResponse, error) {
    switch request.RouteKey {
    case "POST /api/categories":
        return handleCreateCategory(ctx, request)
    case "GET /api/categories":
        return handleListCategories(ctx, request)
    case "GET /api/categories/{categoryId}":
        return handleGetCategory(ctx, request)
    case "PUT /api/categories/{categoryId}":
        return handleUpdateCategory(ctx, request)
    case "DELETE /api/categories/{categoryId}":
        return handleDeleteCategory(ctx, request)
    case "GET /api/categories/{categoryId}/nodes":
        return handleGetCategoryNodes(ctx, request)
    case "POST /api/categories/rebuild":
        return handleRebuildCategories(ctx, request)
    case "GET /api/categories/hierarchy":
        return handleGetCategoryHierarchy(ctx, request)
    }
}
```

### 3.2 Auto-Categorization Lambda

```go
// backend/cmd/auto-categorize/main.go
package main

// Triggered by EventBridge when nodes are created/updated
func handler(ctx context.Context, event events.EventBridgeEvent) error {
    var nodeEvent NodeCreatedEvent
    json.Unmarshal(event.Detail, &nodeEvent)
    
    // Load node
    node, err := repo.FindNodeByID(ctx, nodeEvent.UserID, nodeEvent.NodeID)
    if err != nil {
        return err
    }
    
    // Categorize
    categories, err := categorySvc.CategorizeNode(ctx, *node)
    if err != nil {
        log.Printf("Categorization failed for node %s: %v", node.ID, err)
        // Don't fail the whole operation
        return nil
    }
    
    // Publish event for UI update
    publishCategoriesAssignedEvent(nodeEvent.UserID, nodeEvent.NodeID, categories)
    
    return nil
}
```

### 3.3 Category Insights Lambda

```go
// backend/cmd/category-insights/main.go
package main

// Scheduled lambda that runs daily to optimize categories
func handler(ctx context.Context) error {
    // 1. Find and merge similar categories
    // 2. Suggest new parent categories for orphaned ones
    // 3. Clean up unused categories
    // 4. Generate category statistics
}
```

## Phase 4: Frontend Implementation

### 4.1 Category Components

```typescript
// frontend/src/components/Categories/CategoryTree.tsx
import React, { useState, useEffect } from 'react';
import { Category, CategoryHierarchy } from '@/types';
import { api } from '@/services/api';
import { ChevronRight, ChevronDown, Folder, FolderOpen } from 'lucide-react';

interface CategoryTreeProps {
  onCategorySelect: (categoryId: string) => void;
  selectedCategoryId?: string;
}

export const CategoryTree: React.FC<CategoryTreeProps> = ({ 
  onCategorySelect, 
  selectedCategoryId 
}) => {
  const [categories, setCategories] = useState<Category[]>([]);
  const [hierarchy, setHierarchy] = useState<CategoryHierarchy>({});
  const [expanded, setExpanded] = useState<Set<string>>(new Set());

  useEffect(() => {
    loadCategories();
  }, []);

  const loadCategories = async () => {
    const data = await api.getCategoryHierarchy();
    setCategories(data.categories);
    setHierarchy(data.hierarchy);
  };

  const renderCategory = (category: Category, depth: number = 0) => {
    const hasChildren = hierarchy[category.id]?.length > 0;
    const isExpanded = expanded.has(category.id);
    const isSelected = category.id === selectedCategoryId;

    return (
      <div key={category.id}>
        <div 
          className={`category-item depth-${depth} ${isSelected ? 'selected' : ''}`}
          onClick={() => onCategorySelect(category.id)}
          style={{ paddingLeft: `${depth * 20 + 10}px` }}
        >
          <button
            className="expand-btn"
            onClick={(e) => {
              e.stopPropagation();
              toggleExpanded(category.id);
            }}
          >
            {hasChildren && (isExpanded ? <ChevronDown /> : <ChevronRight />)}
          </button>
          
          {isExpanded ? <FolderOpen /> : <Folder />}
          
          <span className="category-name">{category.name}</span>
          
          <span className="note-count">{category.noteCount}</span>
          
          {category.aiGenerated && <span className="ai-badge">AI</span>}
        </div>
        
        {hasChildren && isExpanded && (
          <div className="category-children">
            {hierarchy[category.id].map(childId => {
              const child = categories.find(c => c.id === childId);
              return child ? renderCategory(child, depth + 1) : null;
            })}
          </div>
        )}
      </div>
    );
  };

  return (
    <div className="category-tree">
      {categories
        .filter(cat => cat.level === 0)
        .map(cat => renderCategory(cat))}
    </div>
  );
};
```

### 4.2 Auto-Categorization UI

```typescript
// frontend/src/components/Node/NodeCategories.tsx
import React, { useState, useEffect } from 'react';
import { Category } from '@/types';
import { api } from '@/services/api';
import { Tag, Plus, X, Sparkles } from 'lucide-react';

interface NodeCategoriesProps {
  nodeId: string;
  editable?: boolean;
}

export const NodeCategories: React.FC<NodeCategoriesProps> = ({ 
  nodeId, 
  editable = false 
}) => {
  const [categories, setCategories] = useState<Category[]>([]);
  const [suggestions, setSuggestions] = useState<Category[]>([]);
  const [loading, setLoading] = useState(false);

  useEffect(() => {
    loadCategories();
  }, [nodeId]);

  const loadCategories = async () => {
    const cats = await api.getNodeCategories(nodeId);
    setCategories(cats);
  };

  const requestSuggestions = async () => {
    setLoading(true);
    try {
      const suggested = await api.suggestCategories(nodeId);
      setSuggestions(suggested);
    } finally {
      setLoading(false);
    }
  };

  const assignCategory = async (category: Category) => {
    await api.assignNodeToCategory(nodeId, category.id);
    setCategories([...categories, category]);
    setSuggestions(suggestions.filter(s => s.id !== category.id));
  };

  const removeCategory = async (categoryId: string) => {
    await api.removeNodeFromCategory(nodeId, categoryId);
    setCategories(categories.filter(c => c.id !== categoryId));
  };

  return (
    <div className="node-categories">
      <div className="categories-list">
        {categories.map(category => (
          <div key={category.id} className="category-tag">
            <Tag size={14} />
            <span>{category.name}</span>
            {editable && (
              <button onClick={() => removeCategory(category.id)}>
                <X size={12} />
              </button>
            )}
          </div>
        ))}
      </div>

      {editable && (
        <div className="category-actions">
          <button 
            className="suggest-btn"
            onClick={requestSuggestions}
            disabled={loading}
          >
            <Sparkles size={14} />
            {loading ? 'Analyzing...' : 'Suggest Categories'}
          </button>
        </div>
      )}

      {suggestions.length > 0 && (
        <div className="suggestions">
          <h4>Suggested Categories:</h4>
          {suggestions.map(suggestion => (
            <button
              key={suggestion.id}
              className="suggestion-item"
              onClick={() => assignCategory(suggestion)}
            >
              <Plus size={14} />
              {suggestion.name}
              {suggestion.confidence && (
                <span className="confidence">
                  {Math.round(suggestion.confidence * 100)}%
                </span>
              )}
            </button>
          ))}
        </div>
      )}
    </div>
  );
};
```

### 4.3 Category Management Page

```typescript
// frontend/src/pages/CategoriesPage.tsx
import React, { useState } from 'react';
import { CategoryTree } from '@/components/Categories/CategoryTree';
import { CategoryDetails } from '@/components/Categories/CategoryDetails';
import { CategoryInsights } from '@/components/Categories/CategoryInsights';
import { api } from '@/services/api';

export const CategoriesPage: React.FC = () => {
  const [selectedCategoryId, setSelectedCategoryId] = useState<string | null>(null);
  const [rebuildProgress, setRebuildProgress] = useState<number | null>(null);

  const handleRebuildCategories = async () => {
    const eventSource = new EventSource('/api/categories/rebuild-stream');
    
    eventSource.onmessage = (event) => {
      const data = JSON.parse(event.data);
      setRebuildProgress(data.progress);
    };

    eventSource.onerror = () => {
      eventSource.close();
      setRebuildProgress(null);
    };
  };

  return (
    <div className="categories-page">
      <div className="page-header">
        <h1>Categories</h1>
        <button 
          className="rebuild-btn"
          onClick={handleRebuildCategories}
          disabled={rebuildProgress !== null}
        >
          {rebuildProgress !== null 
            ? `Rebuilding... ${rebuildProgress}%` 
            : 'Rebuild All Categories'}
        </button>
      </div>

      <div className="categories-layout">
        <aside className="categories-sidebar">
          <CategoryTree 
            onCategorySelect={setSelectedCategoryId}
            selectedCategoryId={selectedCategoryId}
          />
        </aside>

        <main className="categories-content">
          {selectedCategoryId ? (
            <CategoryDetails categoryId={selectedCategoryId} />
          ) : (
            <CategoryInsights />
          )}
        </main>
      </div>
    </div>
  );
};
```

## Phase 5: Advanced Features

### 5.1 Smart Category Evolution

```go
// Detect when categories should be split or merged
func (s *Service) EvolveCategoryStructure(ctx context.Context, userID string) error {
    categories, _ := s.repo.FindCategories(ctx, CategoryQuery{UserID: userID})
    
    for _, category := range categories {
        if category.NoteCount > 50 && category.Level < 2 {
            // Consider splitting large categories
            s.suggestCategorySplit(ctx, category)
        }
    }
    
    // Find similar categories to merge
    s.findAndMergeSimilarCategories(ctx, categories)
    
    return nil
}
```

### 5.2 Category-based Insights

```go
func (s *Service) GenerateCategoryInsights(ctx context.Context, userID string) (*CategoryInsights, error) {
    return &CategoryInsights{
        MostActiveCategories: s.getMostActiveCategories(ctx, userID),
        CategoryGrowthTrends: s.analyzeCategoryGrowth(ctx, userID),
        SuggestedConnections: s.suggestCrossCategoryConnections(ctx, userID),
        KnowledgeGaps: s.identifyKnowledgeGaps(ctx, userID),
    }
}
```

### 5.3 Semantic Category Search

```go
// Find categories based on meaning, not just keywords
func (s *Service) SemanticCategorySearch(ctx context.Context, query string) ([]Category, error) {
    if s.embeddingService != nil {
        queryEmbedding := s.embeddingService.GetEmbedding(query)
        return s.repo.FindCategoriesBySimilarity(ctx, queryEmbedding, 0.7)
    }
    
    // Fallback to keyword search
    return s.keywordCategorySearch(ctx, query)
}
```

## Implementation Timeline

### Week 1-2: Core Infrastructure
- [ ] Update DynamoDB schema with category entities
- [ ] Implement repository methods for categories
- [ ] Create basic category service
- [ ] Add category API endpoints

### Week 3-4: AI Integration
- [ ] Integrate LLM for category suggestions
- [ ] Implement auto-categorization Lambda
- [ ] Add batch categorization for existing nodes
- [ ] Create category evolution algorithms

### Week 5-6: Frontend Implementation
- [ ] Build category tree component
- [ ] Add category filtering to node list
- [ ] Create category management page
- [ ] Implement real-time category updates

### Week 7-8: Advanced Features
- [ ] Add semantic category search
- [ ] Implement category insights dashboard
- [ ] Create category merge/split tools
- [ ] Add category-based recommendations

## Cost Optimization

### DynamoDB Optimization
```javascript
// Use batch operations
const batchAssignCategories = async (assignments) => {
  const chunks = chunk(assignments, 25); // DynamoDB batch limit
  
  for (const chunk of chunks) {
    await dynamodb.batchWriteItem({
      RequestItems: {
        [TABLE_NAME]: chunk.map(item => ({
          PutRequest: { Item: item }
        }))
      }
    });
  }
};
```

### LLM Cost Management
```go
// Cache category suggestions
type CategoryCache struct {
    cache *lru.Cache
    ttl   time.Duration
}

func (c *CategoryCache) GetOrCompute(content string, compute func() []Category) []Category {
    key := hash(content)
    if cached, ok := c.cache.Get(key); ok {
        return cached.([]Category)
    }
    
    categories := compute()
    c.cache.Add(key, categories)
    return categories
}
```

## Testing Strategy

### Unit Tests
```go
func TestCategorizeNode(t *testing.T) {
    // Test with AI enabled
    // Test with AI disabled (fallback to keywords)
    // Test with existing categories
    // Test with new category creation
}
```

### Integration Tests
```go
func TestCategoryHierarchyOperations(t *testing.T) {
    // Test creating parent-child relationships
    // Test circular dependency prevention
    // Test orphan category handling
}
```

### Performance Tests
```go
func BenchmarkBatchCategorization(b *testing.B) {
    // Test categorizing 1000 nodes
    // Measure DynamoDB read/write units
    // Monitor Lambda execution time
}
```

## Success Metrics

1. **Categorization Accuracy**: >80% user satisfaction with AI suggestions
2. **Performance**: <100ms for category assignment
3. **Cost**: <$0.001 per categorization operation
4. **User Engagement**: 50% increase in category usage
5. **Knowledge Discovery**: 30% more cross-category connections found

This implementation plan provides a comprehensive approach to adding enhanced organization features to B2, leveraging the serverless architecture while incorporating the best aspects of B2v1's category system.