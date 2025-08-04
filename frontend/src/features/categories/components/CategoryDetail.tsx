/**
 * CategoryDetail Component - Individual Category Management Interface
 * 
 * Purpose:
 * Provides a detailed view of a single category with editing capabilities and memory management.
 * Displays category information, associated memories, and provides comprehensive management tools.
 * 
 * Key Features:
 * - Category information display with title and description
 * - Inline editing of category details with form validation
 * - Memory listing within the category with management options
 * - Navigation integration with URL parameters and programmatic routing
 * - Loading states for category and memory data
 * - Edit mode toggle with save/cancel functionality
 * - Memory actions including view in graph integration
 * 
 * Category Management:
 * - Display category title, description, and metadata
 * - Inline editing with click-to-edit functionality
 * - Form validation and error handling
 * - Save/cancel operations with loading states
 * - Automatic data refresh after updates
 * 
 * Memory Management:
 * - Display all memories within the category
 * - Memory count and pagination support
 * - Integration with graph visualization for memory viewing
 * - Memory metadata display (content preview, timestamps)
 * - Loading states during memory data fetching
 * 
 * Navigation Features:
 * - Support for URL-based navigation with category ID parameter
 * - Programmatic navigation with callback support
 * - Back navigation functionality
 * - Integration with React Router
 * 
 * State Management:
 * - category: Current category data object
 * - memories: Array of memories within the category
 * - isLoading: Loading state for category data
 * - isLoadingMemories: Loading state for memory data
 * - isEditing: Toggle for edit mode
 * - editTitle/editDescription: Form input values
 * - isSaving: Loading state during save operations
 * 
 * Integration:
 * - Can receive category ID from URL params or props
 * - Integrates with graph visualization for memory viewing
 * - Uses API client for all data operations
 * - Supports both standalone and embedded usage
 */

import React, { useState, useEffect } from 'react';
import { useParams, useNavigate } from 'react-router-dom';
import { categoriesApi } from '../api/categories';
import { nodesApi } from '../../memories/api/nodes';
import type { Category, Node } from '../../../services';

interface CategoryDetailProps {
    /** Optional category ID when used programmatically */
    categoryId?: string;
    /** Optional callback for back navigation */
    onBack?: () => void;
    /** Optional callback for viewing memory in graph */
    onMemoryViewInGraph?: (nodeId: string) => void;
}

const CategoryDetail: React.FC<CategoryDetailProps> = ({ 
    categoryId: propCategoryId, 
    onBack,
    onMemoryViewInGraph 
}) => {
    const { categoryId: urlCategoryId } = useParams<{ categoryId: string }>();
    const navigate = useNavigate();
    const categoryId = propCategoryId || urlCategoryId;
    const [category, setCategory] = useState<Category | null>(null);
    const [memories, setMemories] = useState<Node[]>([]);
    const [isLoading, setIsLoading] = useState(false);
    const [isLoadingMemories, setIsLoadingMemories] = useState(false);
    const [isEditing, setIsEditing] = useState(false);
    const [editTitle, setEditTitle] = useState('');
    const [editDescription, setEditDescription] = useState('');
    const [isSaving, setIsSaving] = useState(false);

    useEffect(() => {
        if (categoryId) {
            loadCategory();
            loadMemories();
        }
    }, [categoryId]);

    const loadCategory = async () => {
        if (!categoryId) return;
        setIsLoading(true);
        try {
            const categoryData = await categoriesApi.getCategory(categoryId);
            setCategory(categoryData);
            setEditTitle(categoryData.title);
            setEditDescription(categoryData.description || '');
        } catch (error) {
            console.error('Error loading category:', error);
        } finally {
            setIsLoading(false);
        }
    };

    const loadMemories = async () => {
        if (!categoryId) return;
        setIsLoadingMemories(true);
        try {
            const data = await categoriesApi.getMemoriesInCategory(categoryId);
            setMemories(data.memories || []);
        } catch (error) {
            console.error('Error loading memories:', error);
        } finally {
            setIsLoadingMemories(false);
        }
    };

    const handleEdit = () => {
        setIsEditing(true);
    };

    const handleSave = async () => {
        if (!editTitle.trim() || !categoryId) return;

        setIsSaving(true);
        try {
            await categoriesApi.updateCategory(categoryId, editTitle.trim(), editDescription.trim() || undefined);
            setIsEditing(false);
            loadCategory(); // Reload to get updated data
        } catch (error) {
            console.error('Error updating category:', error);
        } finally {
            setIsSaving(false);
        }
    };

    const handleCancel = () => {
        if (category) {
            setEditTitle(category.title);
            setEditDescription(category.description || '');
        }
        setIsEditing(false);
    };

    const handleDelete = async () => {
        if (!categoryId) return;
        if (!confirm('Are you sure you want to delete this category? All memory associations will be removed. This cannot be undone.')) {
            return;
        }

        try {
            await categoriesApi.deleteCategory(categoryId);
            if (onBack) {
                onBack();
            } else {
                navigate('/categories');
            }
        } catch (error) {
            console.error('Error deleting category:', error);
        }
    };

    const handleRemoveMemory = async (memoryId: string) => {
        if (!categoryId) return;
        if (!confirm('Remove this memory from the category?')) {
            return;
        }

        try {
            await categoriesApi.removeMemoryFromCategory(categoryId, memoryId);
            loadMemories(); // Reload memories
        } catch (error) {
            console.error('Error removing memory from category:', error);
        }
    };

    const formatDate = (dateString: string): string => {
        const date = new Date(dateString);
        const now = new Date();
        const diffMs = now.getTime() - date.getTime();
        const diffMins = Math.round(diffMs / 60000);

        if (diffMins < 1) return 'Just now';
        if (diffMins < 60) return `${diffMins} minute${diffMins > 1 ? 's' : ''} ago`;
        const diffHours = Math.round(diffMins / 60);
        if (diffHours < 24) return `${diffHours} hour${diffHours > 1 ? 's' : ''} ago`;
        const diffDays = Math.round(diffHours / 24);
        if (diffDays < 7) return `${diffDays} day${diffDays > 1 ? 's' : ''} ago`;
        
        return date.toLocaleDateString();
    };

    if (!categoryId) {
        return <div className="error-state">Category ID not provided</div>;
    }

    if (isLoading) {
        return <div className="loading-state">Loading category...</div>;
    }

    if (!category) {
        return <div className="error-state">Category not found</div>;
    }

    return (
        <div className="category-detail-container">
            <div className="category-detail-header">
                <div className="header-top">
                    <button 
                        className="back-btn" 
                        onClick={onBack || (() => navigate('/categories'))}
                    >
                        ← Back to Categories
                    </button>
                    <div className="header-actions">
                        {isEditing ? (
                            <>
                                <button 
                                    className="primary-btn"
                                    onClick={handleSave}
                                    disabled={!editTitle.trim() || isSaving}
                                >
                                    {isSaving ? 'Saving...' : 'Save'}
                                </button>
                                <button 
                                    className="secondary-btn"
                                    onClick={handleCancel}
                                    disabled={isSaving}
                                >
                                    Cancel
                                </button>
                            </>
                        ) : (
                            <>
                                <button className="secondary-btn" onClick={handleEdit}>
                                    Edit
                                </button>
                                <button className="danger-btn" onClick={handleDelete}>
                                    Delete
                                </button>
                            </>
                        )}
                    </div>
                </div>

                <div className="category-info">
                    {isEditing ? (
                        <div className="edit-form">
                            <input
                                type="text"
                                value={editTitle}
                                onChange={(e) => setEditTitle(e.target.value)}
                                className="edit-title"
                                placeholder="Category title"
                            />
                            <textarea
                                value={editDescription}
                                onChange={(e) => setEditDescription(e.target.value)}
                                className="edit-description"
                                placeholder="Category description (optional)"
                                rows={3}
                            />
                        </div>
                    ) : (
                        <>
                            <h1 className="category-title">{category.title}</h1>
                            {category.description && (
                                <p className="category-description">{category.description}</p>
                            )}
                        </>
                    )}
                    <div className="category-meta">
                        <span>Created {formatDate(category.createdAt)}</span>
                        <span>{memories.length} {memories.length === 1 ? 'memory' : 'memories'}</span>
                    </div>
                </div>
            </div>

            <div className="category-memories">
                <h2>Memories in this Category</h2>
                {isLoadingMemories ? (
                    <div className="loading-state">Loading memories...</div>
                ) : memories.length === 0 ? (
                    <div className="empty-state">
                        <p>No memories in this category yet.</p>
                        <p>Add memories to this category to organize your thoughts!</p>
                    </div>
                ) : (
                    <div className="memory-list">
                        {memories.map(memory => (
                            <div 
                                key={memory.nodeId} 
                                className="memory-item"
                                onClick={onMemoryViewInGraph ? () => onMemoryViewInGraph(memory.nodeId || '') : undefined}
                                style={onMemoryViewInGraph ? { cursor: 'pointer' } : undefined}
                            >
                                <div className="memory-content">
                                    {memory.content}
                                </div>
                                {memory.tags && memory.tags.length > 0 && (
                                    <div className="memory-tags">
                                        {memory.tags.map((tag: string, index: number) => (
                                            <span key={index} className="memory-tag">
                                                {tag}
                                            </span>
                                        ))}
                                    </div>
                                )}
                                <div className="memory-meta">
                                    <span className="memory-date">
                                        {formatDate(memory.timestamp || '')}
                                    </span>
                                    <button 
                                        className="remove-btn"
                                        onClick={(e) => {
                                            e.stopPropagation();
                                            handleRemoveMemory(memory.nodeId || '');
                                        }}
                                        title="Remove from category"
                                    >
                                        Remove
                                    </button>
                                </div>
                            </div>
                        ))}
                    </div>
                )}
            </div>
        </div>
    );
};

export default CategoryDetail;