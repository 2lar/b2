/**
 * CategoriesList Component - Category Management Grid View
 *
 * Purpose:
 * Provides a comprehensive grid-based interface for viewing, creating, and managing categories.
 * Displays all user categories in an organized card layout with creation and navigation functionality.
 *
 * Key Features:
 * - Grid layout displaying category cards with metadata
 * - Inline category creation form with title and description
 * - Navigation to individual category detail views
 * - Category card display with creation dates and descriptions
 * - Loading states and empty state handling
 * - Responsive grid layout that adapts to screen size
 * - Integration with routing for category navigation
 *
 * Category Creation:
 * - Toggle-able creation form with title and description fields
 * - Form validation and submission handling
 * - Loading states during category creation
 * - Automatic refresh after successful creation
 * - Error handling for failed operations
 *
 * Category Display:
 * - Card-based layout with hover effects
 * - Category title and description display
 * - Creation date formatting and display
 * - Click-to-navigate to category detail view
 * - Empty state messaging for new users
 *
 * State Management:
 * - categories: Array of category objects
 * - isLoading: Loading state for initial data fetch
 * - showCreateForm: Toggle for creation form visibility
 * - newCategoryTitle/Description: Form input values
 * - isCreating: Loading state during category creation
 *
 * Integration:
 * - Can be used standalone or with callback for category selection
 * - Integrates with React Router for navigation
 * - Uses API client for category operations
 * - Accessible from main navigation and dashboard
 */

import React, { useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { categoriesApi } from '../api/categories';
import type { Category } from '../../../services';
import { useGraphStore } from '../../../stores/graphStore';

interface CategoriesListProps {
    /** Optional callback when category is selected instead of navigation */
    onCategorySelect?: (categoryId: string) => void;
}

const CategoriesList: React.FC<CategoriesListProps> = ({ onCategorySelect }) => {
    const navigate = useNavigate();
    const queryClient = useQueryClient();
    const { isSidebarOpen, toggleSidebar } = useGraphStore();

    const [showCreateForm, setShowCreateForm] = useState(false);
    const [newCategoryTitle, setNewCategoryTitle] = useState('');
    const [newCategoryDescription, setNewCategoryDescription] = useState('');

    const { data: categories = [], isLoading, isError, error } = useQuery({
        queryKey: ['categories'],
        queryFn: async () => {
            const data = await categoriesApi.listCategories();
            return data.categories || [];
        }
    });

    const createCategoryMutation = useMutation({
        mutationFn: () => categoriesApi.createCategory(newCategoryTitle.trim(), newCategoryDescription.trim() || undefined),
        onSuccess: () => {
            queryClient.invalidateQueries({ queryKey: ['categories'] });
            setNewCategoryTitle('');
            setNewCategoryDescription('');
            setShowCreateForm(false);
        },
    });

    const handleCreateCategory = () => {
        if (!newCategoryTitle.trim()) return;
        createCategoryMutation.mutate();
    };

    const handleCancelCreate = () => {
        setNewCategoryTitle('');
        setNewCategoryDescription('');
        setShowCreateForm(false);
    };

    const formatDate = (dateString: string): string => {
        const date = new Date(dateString);
        return date.toLocaleDateString();
    };

    if (isError) {
        return <div>Error loading categories: {error.message}</div>;
    }

    return (
        <div className="categories-container">
            <div className="categories-header">
                <h2>All Categories</h2>
                <div>
                    <button
                        className="secondary-btn"
                        onClick={toggleSidebar}
                    >
                        {isSidebarOpen ? 'Close' : 'Open'} Sidebar (Zustand)
                    </button>
                    <button
                        className="primary-btn create-category-btn"
                        onClick={() => setShowCreateForm(true)}
                    >
                        + New Category
                    </button>
                </div>
            </div>

            {showCreateForm && (
                <div className="create-category-form">
                    <div className="form-group">
                        <label htmlFor="category-title">Title</label>
                        <input
                            id="category-title"
                            type="text"
                            value={newCategoryTitle}
                            onChange={(e) => setNewCategoryTitle(e.target.value)}
                            placeholder="Enter category title"
                            autoFocus
                        />
                    </div>
                    <div className="form-group">
                        <label htmlFor="category-description">Description (optional)</label>
                        <textarea
                            id="category-description"
                            value={newCategoryDescription}
                            onChange={(e) => setNewCategoryDescription(e.target.value)}
                            placeholder="Enter category description"
                            rows={3}
                        />
                    </div>
                    <div className="form-actions">
                        <button
                            className="primary-btn"
                            onClick={handleCreateCategory}
                            disabled={!newCategoryTitle.trim() || createCategoryMutation.isPending}
                        >
                            {createCategoryMutation.isPending ? 'Creating...' : 'Create Category'}
                        </button>
                        <button
                            className="secondary-btn"
                            onClick={handleCancelCreate}
                            disabled={createCategoryMutation.isPending}
                        >
                            Cancel
                        </button>
                    </div>
                </div>
            )}

            <div className="categories-grid">
                {isLoading ? (
                    <div className="loading-state">Loading categories...</div>
                ) : categories.length === 0 ? (
                    <div className="empty-state">
                        <p>No categories yet.</p>
                        <p>Create your first category to start organizing your memories!</p>
                    </div>
                ) : (
                    categories.map((category: Category) => (
                        <div
                            key={category.id}
                            className="category-card"
                            onClick={() => {
                                if (onCategorySelect) {
                                    onCategorySelect(category.id);
                                } else {
                                    navigate(`/categories/${category.id}`);
                                }
                            }}
                        >
                            <div className="category-card-header">
                                <h3 className="category-title">{category.title}</h3>
                            </div>
                            {category.description && (
                                <div className="category-description">
                                    {category.description}
                                </div>
                            )}
                            <div className="category-card-meta">
                                <span className="category-date">
                                    Created {formatDate(category.createdAt)}
                                </span>
                            </div>
                        </div>
                    ))
                )}
            </div>
        </div>
    );
};

export default CategoriesList;
