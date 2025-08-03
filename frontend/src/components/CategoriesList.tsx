import React, { useState, useEffect } from 'react';
import { useNavigate } from 'react-router-dom';
import { api, type Category } from '../services';

interface CategoriesListProps {
    onCategorySelect?: (categoryId: string) => void;
}

const CategoriesList: React.FC<CategoriesListProps> = ({ onCategorySelect }) => {
    const navigate = useNavigate();
    const [categories, setCategories] = useState<Category[]>([]);
    const [isLoading, setIsLoading] = useState(false);
    const [showCreateForm, setShowCreateForm] = useState(false);
    const [newCategoryTitle, setNewCategoryTitle] = useState('');
    const [newCategoryDescription, setNewCategoryDescription] = useState('');
    const [isCreating, setIsCreating] = useState(false);

    useEffect(() => {
        loadCategories();
    }, []);

    const loadCategories = async () => {
        setIsLoading(true);
        try {
            const data = await api.listCategories();
            setCategories(data.categories || []);
        } catch (error) {
            console.error('Error loading categories:', error);
        } finally {
            setIsLoading(false);
        }
    };

    const handleCreateCategory = async () => {
        if (!newCategoryTitle.trim()) return;

        setIsCreating(true);
        try {
            await api.createCategory(newCategoryTitle.trim(), newCategoryDescription.trim() || undefined);
            setNewCategoryTitle('');
            setNewCategoryDescription('');
            setShowCreateForm(false);
            loadCategories();
        } catch (error) {
            console.error('Error creating category:', error);
        } finally {
            setIsCreating(false);
        }
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

    return (
        <div className="categories-container">
            <div className="categories-header">
                <h2>All Categories</h2>
                <button 
                    className="primary-btn create-category-btn"
                    onClick={() => setShowCreateForm(true)}
                >
                    + New Category
                </button>
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
                            disabled={!newCategoryTitle.trim() || isCreating}
                        >
                            {isCreating ? 'Creating...' : 'Create Category'}
                        </button>
                        <button 
                            className="secondary-btn"
                            onClick={handleCancelCreate}
                            disabled={isCreating}
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
                    categories.map(category => (
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