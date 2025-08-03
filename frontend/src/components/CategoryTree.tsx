import React, { useState, useEffect } from 'react';
import { components } from '../types/generated/generated-types';

// Type aliases for easier usage
type Category = components['schemas']['Category'];
type CategoryHierarchy = { [key: string]: string[] };

interface CategoryTreeProps {
  onCategorySelect?: (categoryId: string) => void;
  selectedCategoryId?: string;
  showAIBadges?: boolean;
  showNoteCounts?: boolean;
  editable?: boolean;
}

interface CategoryTreeItem extends Category {
  children?: CategoryTreeItem[];
  isExpanded?: boolean;
}

export const CategoryTree: React.FC<CategoryTreeProps> = ({
  onCategorySelect,
  selectedCategoryId,
  showAIBadges = true,
  showNoteCounts = true,
  editable = false
}) => {
  const [categories, setCategories] = useState<Category[]>([]);
  const [hierarchy, setHierarchy] = useState<CategoryHierarchy>({});
  const [treeData, setTreeData] = useState<CategoryTreeItem[]>([]);
  const [expandedNodes, setExpandedNodes] = useState<Set<string>>(new Set());
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    loadCategoryHierarchy();
  }, []);

  useEffect(() => {
    if (categories.length > 0) {
      buildTreeStructure();
    }
  }, [categories, hierarchy, expandedNodes]);

  const loadCategoryHierarchy = async () => {
    setLoading(true);
    setError(null);
    try {
      const response = await fetch('/api/categories/hierarchy');
      if (!response.ok) {
        throw new Error('Failed to load categories');
      }
      
      const data = await response.json();
      setCategories(data.categories || []);
      setHierarchy(data.hierarchy || {});
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Unknown error');
      console.error('Error loading category hierarchy:', err);
    } finally {
      setLoading(false);
    }
  };

  const buildTreeStructure = () => {
    const categoryMap = new Map<string, CategoryTreeItem>();
    const rootCategories: CategoryTreeItem[] = [];

    // Create map of all categories
    categories.forEach(category => {
      categoryMap.set(category.id, {
        ...category,
        children: [],
        isExpanded: expandedNodes.has(category.id)
      });
    });

    // Build tree structure
    categories.forEach(category => {
      const categoryItem = categoryMap.get(category.id);
      if (!categoryItem) return;

      if (category.level === 0 || !category.parentId) {
        // Root level category
        rootCategories.push(categoryItem);
      } else {
        // Child category - add to parent
        const parent = categoryMap.get(category.parentId);
        if (parent) {
          parent.children = parent.children || [];
          parent.children.push(categoryItem);
        } else {
          // Parent not found, treat as root
          rootCategories.push(categoryItem);
        }
      }
    });

    // Sort categories by title at each level
    const sortCategories = (items: CategoryTreeItem[]) => {
      items.sort((a, b) => a.title.localeCompare(b.title));
      items.forEach(item => {
        if (item.children && item.children.length > 0) {
          sortCategories(item.children);
        }
      });
    };

    sortCategories(rootCategories);
    setTreeData(rootCategories);
  };

  const toggleExpanded = (categoryId: string) => {
    const newExpanded = new Set(expandedNodes);
    if (newExpanded.has(categoryId)) {
      newExpanded.delete(categoryId);
    } else {
      newExpanded.add(categoryId);
    }
    setExpandedNodes(newExpanded);
  };

  const handleCategoryClick = (categoryId: string) => {
    if (onCategorySelect) {
      onCategorySelect(categoryId);
    }
  };

  const renderCategoryIcon = (category: Category) => {
    if (category.icon) {
      return <span className="category-icon">{category.icon}</span>;
    }
    
    // Default icons based on level
    const defaultIcons = ['📁', '📂', '📄'];
    return <span className="category-icon">{defaultIcons[category.level] || '📄'}</span>;
  };

  const renderCategory = (category: CategoryTreeItem, depth: number = 0): React.ReactNode => {
    const hasChildren = category.children && category.children.length > 0;
    const isExpanded = category.isExpanded;
    const isSelected = category.id === selectedCategoryId;
    const indentWidth = depth * 20;

    return (
      <div key={category.id} className="category-tree-item">
        <div 
          className={`category-item-content ${isSelected ? 'selected' : ''} ${category.aiGenerated ? 'ai-generated' : ''}`}
          style={{ 
            paddingLeft: `${indentWidth + 12}px`,
            backgroundColor: isSelected ? (category.color || '#e3f2fd') : 'transparent'
          }}
          onClick={() => handleCategoryClick(category.id)}
        >
          {hasChildren && (
            <button
              className="expand-toggle"
              onClick={(e) => {
                e.stopPropagation();
                toggleExpanded(category.id);
              }}
              aria-label={isExpanded ? 'Collapse' : 'Expand'}
            >
              <span className={`arrow ${isExpanded ? 'expanded' : ''}`}>▶</span>
            </button>
          )}
          
          {!hasChildren && <span className="expand-spacer" />}
          
          {renderCategoryIcon(category)}
          
          <span 
            className="category-title"
            style={{ color: category.color || 'inherit' }}
          >
            {category.title}
          </span>
          
          {showNoteCounts && category.noteCount !== undefined && (
            <span className="note-count">
              {category.noteCount}
            </span>
          )}
          
          {showAIBadges && category.aiGenerated && (
            <span className="ai-badge" title="AI Generated">
              🤖
            </span>
          )}
          
          {editable && (
            <div className="category-actions">
              <button className="edit-btn" title="Edit category">
                ✏️
              </button>
              <button className="delete-btn" title="Delete category">
                🗑️
              </button>
            </div>
          )}
        </div>
        
        {hasChildren && isExpanded && (
          <div className="category-children">
            {category.children!.map(child => renderCategory(child, depth + 1))}
          </div>
        )}
      </div>
    );
  };

  if (loading) {
    return (
      <div className="category-tree loading">
        <div className="loading-spinner">Loading categories...</div>
      </div>
    );
  }

  if (error) {
    return (
      <div className="category-tree error">
        <div className="error-message">
          <span>Error loading categories: {error}</span>
          <button onClick={loadCategoryHierarchy} className="retry-btn">
            Retry
          </button>
        </div>
      </div>
    );
  }

  if (treeData.length === 0) {
    return (
      <div className="category-tree empty">
        <div className="empty-state">
          <p>No categories yet.</p>
          <p>Create your first category to start organizing your memories!</p>
        </div>
      </div>
    );
  }

  return (
    <div className="category-tree">
      <div className="category-tree-header">
        <h3>Categories</h3>
        <button 
          className="refresh-btn"
          onClick={loadCategoryHierarchy}
          title="Refresh categories"
        >
          🔄
        </button>
      </div>
      
      <div className="category-tree-content">
        {treeData.map(category => renderCategory(category))}
      </div>
    </div>
  );
};

export default CategoryTree;