/**
 * FileSystemSidebar Component - Windows Explorer-Style Memory Browser
 * 
 * Purpose:
 * Provides a file system-like interface for browsing and organizing memories within categories.
 * Mimics Windows File Explorer with expandable folders (categories) containing files (memories)
 * and includes advanced features like drag-and-drop, search, and context menus.
 * 
 * Key Features:
 * - Windows Explorer-style hierarchical folder structure
 * - Expandable categories showing memories as files within folders
 * - Drag-and-drop memory movement between categories
 * - Real-time search across categories and memories
 * - Right-click context menus for memory and category management
 * - Uncategorized memories section for memories without categories
 * - Visual indicators for AI-generated categories
 * - Memory count badges on category folders
 * - Efficient caching system for performance
 * 
 * File System Structure:
 * - Categories displayed as expandable folders with icons
 * - Memories displayed as files within category folders
 * - "Uncategorized" special folder for memories without categories
 * - Visual hierarchy with proper indentation and tree lines
 * - Expand/collapse arrows with smooth rotation animations
 * 
 * Drag-and-Drop Features:
 * - Drag memories between categories with visual feedback
 * - Drop target highlighting during drag operations
 * - Automatic API calls to update memory categorization
 * - Visual loading states during move operations
 * - Error handling for failed operations
 * 
 * Search and Filtering:
 * - Real-time search across category names and memory content
 * - Instant filtering with highlighted results
 * - Search persistence during navigation
 * - Case-insensitive search matching
 * 
 * Context Menu Actions:
 * - Right-click on memories: View in Graph, Remove from Category, Delete
 * - Right-click on categories: View Category, Delete Category
 * - Confirmation dialogs for destructive operations
 * - Graceful error handling and user feedback
 * 
 * State Management:
 * - categories: List of categories with nested memory data
 * - uncategorizedMemories: Memories not assigned to any category
 * - expandedCategories: Set of currently expanded category IDs
 * - memoriesCache: Performance cache for category memories
 * - searchTerm: Current search filter text
 * - draggedMemory: Currently dragged memory information
 * - contextMenu: Right-click menu state and position
 * 
 * Integration:
 * - Positioned in left panel of Dashboard layout
 * - Integrates with GraphVisualization for memory selection
 * - Coordinates with Dashboard for refresh triggers
 * - Uses API client for all data operations
 */

import React, { useState, useEffect, useCallback } from 'react';
import { nodesApi } from '../api/nodes';
import { categoriesApi } from '../../categories/api/categories';
import { components } from '../../../types/generated/generated-types';

// Type aliases for easier usage
type Category = components['schemas']['Category'];
type Node = components['schemas']['Node'];

interface FileSystemSidebarProps {
    /** User ID for loading user-specific data */
    userId: string;
    /** Callback when a memory is selected for viewing */
    onMemorySelect: (nodeId: string) => void;
    /** Callback when a category is selected for viewing */
    onCategorySelect: (categoryId: string) => void;
    /** Optional trigger number that causes refresh when changed */
    refreshTrigger?: number;
}

interface CategoryWithMemories {
    id: string;
    title: string;
    description?: string;
    level: number;
    parentId?: string;
    color?: string;
    icon?: string;
    aiGenerated: boolean;
    noteCount: number;
    createdAt: string;
    updatedAt: string;
    memories?: Node[];
    isExpanded?: boolean;
    isLoading?: boolean;
}

const FileSystemSidebar: React.FC<FileSystemSidebarProps> = ({
    userId,
    onMemorySelect,
    onCategorySelect,
    refreshTrigger
}) => {
    const [categories, setCategories] = useState<CategoryWithMemories[]>([]);
    const [uncategorizedMemories, setUncategorizedMemories] = useState<Node[]>([]);
    const [expandedCategories, setExpandedCategories] = useState<Set<string>>(new Set());
    const [memoriesCache, setMemoriesCache] = useState<Record<string, Node[]>>({});
    const [isLoading, setIsLoading] = useState(false);
    const [isUncategorizedExpanded, setIsUncategorizedExpanded] = useState(false);
    const [isLoadingUncategorized, setIsLoadingUncategorized] = useState(false);
    const [searchTerm, setSearchTerm] = useState('');
    const [draggedMemory, setDraggedMemory] = useState<{ nodeId: string; fromCategoryId?: string } | null>(null);
    const [dropTarget, setDropTarget] = useState<string | null>(null);
    const [isMovingMemory, setIsMovingMemory] = useState(false);
    const [contextMenu, setContextMenu] = useState<{
        x: number;
        y: number;
        type: 'memory' | 'category';
        itemId: string;
        categoryId?: string;
    } | null>(null);

    // Load initial categories
    useEffect(() => {
        loadCategories();
    }, [userId]);

    // Refresh when refreshTrigger changes
    useEffect(() => {
        if (refreshTrigger !== undefined) {
            // Clear cache and reload everything
            setMemoriesCache({});
            setUncategorizedMemories([]);
            loadCategories();
            
            // If uncategorized is expanded, reload it
            if (isUncategorizedExpanded) {
                setTimeout(() => loadUncategorizedMemories(), 100);
            }
        }
    }, [refreshTrigger]);

    // Close context menu when clicking elsewhere
    useEffect(() => {
        const handleClickOutside = () => setContextMenu(null);
        if (contextMenu) {
            document.addEventListener('click', handleClickOutside);
            return () => document.removeEventListener('click', handleClickOutside);
        }
    }, [contextMenu]);

    const loadCategories = async () => {
        setIsLoading(true);
        try {
            const response = await categoriesApi.listCategories();
            const categoriesData = response.categories || [];
            
            // Sort categories alphabetically
            const sortedCategories = categoriesData.sort((a, b) => 
                a.title.localeCompare(b.title)
            );

            setCategories(sortedCategories.map(cat => ({
                ...cat,
                aiGenerated: cat.aiGenerated ?? false,
                noteCount: cat.noteCount ?? 0,
                isExpanded: expandedCategories.has(cat.id),
                isLoading: false
            })));
        } catch (error) {
            console.error('Error loading categories:', error);
        } finally {
            setIsLoading(false);
        }
    };

    const loadMemoriesInCategory = async (categoryId: string) => {
        // Check cache first
        if (memoriesCache[categoryId]) {
            return memoriesCache[categoryId];
        }

        try {
            // Mark category as loading
            setCategories(prev => prev.map(cat => 
                cat.id === categoryId ? { ...cat, isLoading: true } : cat
            ));

            const response = await categoriesApi.getNodesInCategory(categoryId);
            const memories = response.memories || [];

            // Sort memories by timestamp (newest first)
            const sortedMemories = memories.sort((a, b) => 
                new Date(b.timestamp || '').getTime() - new Date(a.timestamp || '').getTime()
            );

            // Update cache
            setMemoriesCache(prev => ({
                ...prev,
                [categoryId]: sortedMemories
            }));

            // Update category memories
            setCategories(prev => prev.map(cat => 
                cat.id === categoryId 
                    ? { ...cat, memories: sortedMemories, isLoading: false }
                    : cat
            ));

            return sortedMemories;
        } catch (error) {
            console.error('Error loading memories for category:', categoryId, error);
            
            // Mark category as not loading
            setCategories(prev => prev.map(cat => 
                cat.id === categoryId ? { ...cat, isLoading: false } : cat
            ));
            
            return [];
        }
    };

    const loadUncategorizedMemories = async () => {
        setIsLoadingUncategorized(true);
        try {
            // Get all memories
            const allMemoriesResponse = await nodesApi.listNodes();
            const allMemories = allMemoriesResponse.nodes || [];

            // Get all categorized memory IDs
            const categorizedMemoryIds = new Set<string>();
            for (const category of categories) {
                if (category.memories) {
                    category.memories.forEach(memory => categorizedMemoryIds.add(memory.nodeId));
                } else if (memoriesCache[category.id]) {
                    memoriesCache[category.id].forEach(memory => categorizedMemoryIds.add(memory.nodeId));
                }
            }

            // If we don't have complete category data, fetch it
            if (Object.keys(memoriesCache).length < categories.length) {
                for (const category of categories) {
                    if (!memoriesCache[category.id]) {
                        const categoryMemories = await loadMemoriesInCategory(category.id);
                        categoryMemories.forEach(memory => categorizedMemoryIds.add(memory.nodeId));
                    }
                }
            }

            // Filter out categorized memories
            const uncategorized = allMemories.filter(memory => 
                !categorizedMemoryIds.has(memory.nodeId)
            );

            // Sort by timestamp (newest first)
            const sortedUncategorized = uncategorized.sort((a, b) => 
                new Date(b.timestamp || '').getTime() - new Date(a.timestamp || '').getTime()
            );

            setUncategorizedMemories(sortedUncategorized);
        } catch (error) {
            console.error('Error loading uncategorized memories:', error);
        } finally {
            setIsLoadingUncategorized(false);
        }
    };

    const toggleCategoryExpansion = useCallback(async (categoryId: string) => {
        const isCurrentlyExpanded = expandedCategories.has(categoryId);
        const newExpandedCategories = new Set(expandedCategories);

        if (isCurrentlyExpanded) {
            newExpandedCategories.delete(categoryId);
        } else {
            newExpandedCategories.add(categoryId);
            // Load memories when expanding
            await loadMemoriesInCategory(categoryId);
        }

        setExpandedCategories(newExpandedCategories);
        
        // Update category expansion state
        setCategories(prev => prev.map(cat => 
            cat.id === categoryId 
                ? { ...cat, isExpanded: !isCurrentlyExpanded }
                : cat
        ));
    }, [expandedCategories]);

    const toggleUncategorizedExpansion = useCallback(async () => {
        const wasExpanded = isUncategorizedExpanded;
        setIsUncategorizedExpanded(!wasExpanded);
        
        if (!wasExpanded) {
            await loadUncategorizedMemories();
        }
    }, [isUncategorizedExpanded, categories, memoriesCache]);

    const handleMemoryClick = (nodeId: string) => {
        onMemorySelect(nodeId);
    };

    const handleCategoryClick = (categoryId: string) => {
        onCategorySelect(categoryId);
    };

    // Drag and Drop handlers
    const handleMemoryDragStart = (e: React.DragEvent, nodeId: string, fromCategoryId?: string) => {
        setDraggedMemory({ nodeId, fromCategoryId });
        e.dataTransfer.effectAllowed = 'move';
        e.dataTransfer.setData('text/plain', nodeId);
        
        // Add visual feedback
        e.currentTarget.classList.add('dragging');
    };

    const handleMemoryDragEnd = (e: React.DragEvent) => {
        setDraggedMemory(null);
        setDropTarget(null);
        e.currentTarget.classList.remove('dragging');
    };

    const handleCategoryDragOver = (e: React.DragEvent, categoryId: string) => {
        e.preventDefault();
        e.dataTransfer.dropEffect = 'move';
        setDropTarget(categoryId);
    };

    const handleCategoryDragLeave = (e: React.DragEvent) => {
        // Only clear drop target if we're leaving the category entirely
        const rect = e.currentTarget.getBoundingClientRect();
        const x = e.clientX;
        const y = e.clientY;
        
        if (x < rect.left || x > rect.right || y < rect.top || y > rect.bottom) {
            setDropTarget(null);
        }
    };

    const handleCategoryDrop = async (e: React.DragEvent, targetCategoryId: string) => {
        e.preventDefault();
        setDropTarget(null);

        if (!draggedMemory) return;

        const { nodeId, fromCategoryId } = draggedMemory;
        
        // Don't do anything if dropping on the same category
        if (fromCategoryId === targetCategoryId) {
            setDraggedMemory(null);
            return;
        }

        setIsMovingMemory(true);
        
        try {
            // Remove from old category if it exists
            if (fromCategoryId) {
                await categoriesApi.removeNodeFromCategory(fromCategoryId, nodeId);
            }

            // Add to new category
            await categoriesApi.assignNodeToCategory(targetCategoryId, nodeId);

            // Update local state by clearing cache and reloading
            setMemoriesCache(prev => {
                const newCache = { ...prev };
                
                // Remove from old category cache
                if (fromCategoryId && newCache[fromCategoryId]) {
                    newCache[fromCategoryId] = newCache[fromCategoryId].filter(
                        memory => memory.nodeId !== nodeId
                    );
                }
                
                // Clear target category cache to force reload
                delete newCache[targetCategoryId];
                
                return newCache;
            });

            // Update categories to reflect new counts
            setCategories(prev => prev.map(cat => {
                if (cat.id === fromCategoryId) {
                    return { ...cat, noteCount: (cat.noteCount || 1) - 1, memories: undefined };
                } else if (cat.id === targetCategoryId) {
                    return { ...cat, noteCount: (cat.noteCount || 0) + 1, memories: undefined };
                }
                return cat;
            }));

            // Update uncategorized memories if moving from there
            if (!fromCategoryId) {
                setUncategorizedMemories(prev => 
                    prev.filter(memory => memory.nodeId !== nodeId)
                );
            }

            // Reload the target category if it's expanded
            if (expandedCategories.has(targetCategoryId)) {
                await loadMemoriesInCategory(targetCategoryId);
            }

        } catch (error) {
            console.error('Error moving memory between categories:', error);
            // You might want to show a toast notification here
        } finally {
            setIsMovingMemory(false);
        }

        setDraggedMemory(null);
    };

    // Context Menu handlers
    const handleMemoryContextMenu = (e: React.MouseEvent, nodeId: string, categoryId?: string) => {
        e.preventDefault();
        e.stopPropagation();
        setContextMenu({
            x: e.clientX,
            y: e.clientY,
            type: 'memory',
            itemId: nodeId,
            categoryId
        });
    };

    const handleCategoryContextMenu = (e: React.MouseEvent, categoryId: string) => {
        e.preventDefault();
        e.stopPropagation();
        setContextMenu({
            x: e.clientX,
            y: e.clientY,
            type: 'category',
            itemId: categoryId
        });
    };

    const handleDeleteMemory = async (nodeId: string, categoryId?: string) => {
        if (!confirm('Are you sure you want to delete this memory?')) return;
        
        try {
            await nodesApi.deleteNode(nodeId);
            
            // Update local state
            if (categoryId) {
                // Remove from category cache
                setMemoriesCache(prev => ({
                    ...prev,
                    [categoryId]: (prev[categoryId] || []).filter(memory => memory.nodeId !== nodeId)
                }));
                
                // Update category count
                setCategories(prev => prev.map(cat => 
                    cat.id === categoryId 
                        ? { ...cat, noteCount: Math.max(0, (cat.noteCount || 1) - 1) }
                        : cat
                ));
            } else {
                // Remove from uncategorized
                setUncategorizedMemories(prev => 
                    prev.filter(memory => memory.nodeId !== nodeId)
                );
            }
            
        } catch (error) {
            console.error('Error deleting memory:', error);
            alert('Failed to delete memory. Please try again.');
        }
        
        setContextMenu(null);
    };

    const handleRemoveNodeFromCategory = async (nodeId: string, categoryId: string) => {
        try {
            await categoriesApi.removeNodeFromCategory(categoryId, nodeId);
            
            // Update local state
            setMemoriesCache(prev => ({
                ...prev,
                [categoryId]: (prev[categoryId] || []).filter(memory => memory.nodeId !== nodeId)
            }));
            
            // Update category count
            setCategories(prev => prev.map(cat => 
                cat.id === categoryId 
                    ? { ...cat, noteCount: Math.max(0, (cat.noteCount || 1) - 1) }
                    : cat
            ));
            
            // Add to uncategorized if it's expanded
            if (isUncategorizedExpanded) {
                await loadUncategorizedMemories();
            }
            
        } catch (error) {
            console.error('Error removing memory from category:', error);
            alert('Failed to remove memory from category. Please try again.');
        }
        
        setContextMenu(null);
    };

    const handleDeleteCategory = async (categoryId: string) => {
        if (!confirm('Are you sure you want to delete this category? Memories will become uncategorized.')) return;
        
        try {
            await categoriesApi.deleteCategory(categoryId);
            
            // Update local state
            setCategories(prev => prev.filter(cat => cat.id !== categoryId));
            
            // Clear cache for deleted category
            setMemoriesCache(prev => {
                const newCache = { ...prev };
                delete newCache[categoryId];
                return newCache;
            });
            
            // Remove from expanded categories
            setExpandedCategories(prev => {
                const newExpanded = new Set(prev);
                newExpanded.delete(categoryId);
                return newExpanded;
            });
            
            // Reload uncategorized if expanded (memories might have moved there)
            if (isUncategorizedExpanded) {
                await loadUncategorizedMemories();
            }
            
        } catch (error) {
            console.error('Error deleting category:', error);
            alert('Failed to delete category. Please try again.');
        }
        
        setContextMenu(null);
    };

    const truncateText = (text: string, maxLength: number = 40) => {
        return text.length > maxLength ? text.substring(0, maxLength) + '...' : text;
    };

    const formatTimestamp = (timestamp: string) => {
        const date = new Date(timestamp);
        const now = new Date();
        const diffInHours = (now.getTime() - date.getTime()) / (1000 * 60 * 60);

        if (diffInHours < 24) {
            return date.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' });
        } else if (diffInHours < 168) { // 7 days
            return date.toLocaleDateString([], { weekday: 'short' });
        } else {
            return date.toLocaleDateString([], { month: 'short', day: 'numeric' });
        }
    };

    // Filter categories and memories based on search term
    const filteredCategories = categories.filter(category =>
        searchTerm === '' || category.title.toLowerCase().includes(searchTerm.toLowerCase())
    );

    const filteredUncategorizedMemories = uncategorizedMemories.filter(memory =>
        searchTerm === '' || memory.content?.toLowerCase().includes(searchTerm.toLowerCase())
    );

    return (
        <div className="file-system-sidebar">
            <div className="sidebar-header">
                <h3>📁 Memory Explorer</h3>
                <div className="search-container">
                    <input
                        type="text"
                        placeholder="Search memories..."
                        value={searchTerm}
                        onChange={(e) => setSearchTerm(e.target.value)}
                        className="search-input"
                        autoComplete="off"
                    />
                </div>
            </div>

            <div className="sidebar-content">
                {isLoading ? (
                    <div className="loading-state">Loading categories...</div>
                ) : isMovingMemory ? (
                    <div className="loading-state">Moving memory...</div>
                ) : (
                    <>
                        {/* Categories */}
                        {filteredCategories.map(category => {
                            const isExpanded = expandedCategories.has(category.id);
                            const memories = category.memories || memoriesCache[category.id] || [];
                            
                            return (
                                <div key={category.id} className="file-system-item">
                                    <div 
                                        className={`folder-header ${dropTarget === category.id ? 'drop-target' : ''}`}
                                        onClick={() => toggleCategoryExpansion(category.id)}
                                        onContextMenu={(e) => handleCategoryContextMenu(e, category.id)}
                                        onDragOver={(e) => handleCategoryDragOver(e, category.id)}
                                        onDragLeave={handleCategoryDragLeave}
                                        onDrop={(e) => handleCategoryDrop(e, category.id)}
                                    >
                                        <span className="expand-icon">
                                            {isExpanded ? '📂' : '📁'}
                                        </span>
                                        <span className={`expand-arrow ${isExpanded ? 'expanded' : ''}`}>
                                            ▶
                                        </span>
                                        <span 
                                            className="folder-name"
                                            onClick={(e) => {
                                                e.stopPropagation();
                                                handleCategoryClick(category.id);
                                            }}
                                        >
                                            {category.title}
                                        </span>
                                        <span className="memory-count">
                                            ({category.noteCount || 0})
                                        </span>
                                        {category.aiGenerated && (
                                            <span className="ai-badge" title="AI Generated">🤖</span>
                                        )}
                                    </div>

                                    {isExpanded && (
                                        <div className="folder-contents">
                                            {category.isLoading ? (
                                                <div className="loading-memories">Loading memories...</div>
                                            ) : memories.length === 0 ? (
                                                <div className="empty-folder">No memories in this category</div>
                                            ) : (
                                                memories
                                                    .filter(memory => 
                                                        searchTerm === '' || 
                                                        memory.content?.toLowerCase().includes(searchTerm.toLowerCase())
                                                    )
                                                    .map(memory => (
                                                        <div 
                                                            key={memory.nodeId}
                                                            className="memory-file"
                                                            onClick={() => handleMemoryClick(memory.nodeId)}
                                                            onContextMenu={(e) => handleMemoryContextMenu(e, memory.nodeId, category.id)}
                                                            title={memory.content}
                                                            draggable={true}
                                                            onDragStart={(e) => handleMemoryDragStart(e, memory.nodeId, category.id)}
                                                            onDragEnd={handleMemoryDragEnd}
                                                        >
                                                            <span className="file-icon">📄</span>
                                                            <span className="file-name">
                                                                {truncateText(memory.content || 'Untitled')}
                                                            </span>
                                                            <span className="file-timestamp">
                                                                {memory.timestamp && formatTimestamp(memory.timestamp)}
                                                            </span>
                                                        </div>
                                                    ))
                                            )}
                                        </div>
                                    )}
                                </div>
                            );
                        })}

                        {/* Uncategorized Memories */}
                        <div className="file-system-item uncategorized-section">
                            <div 
                                className="folder-header"
                                onClick={toggleUncategorizedExpansion}
                            >
                                <span className="expand-icon">
                                    {isUncategorizedExpanded ? '📂' : '📁'}
                                </span>
                                <span className={`expand-arrow ${isUncategorizedExpanded ? 'expanded' : ''}`}>
                                    ▶
                                </span>
                                <span className="folder-name">Uncategorized</span>
                                <span className="memory-count">
                                    ({uncategorizedMemories.length})
                                </span>
                            </div>

                            {isUncategorizedExpanded && (
                                <div className="folder-contents">
                                    {isLoadingUncategorized ? (
                                        <div className="loading-memories">Loading memories...</div>
                                    ) : filteredUncategorizedMemories.length === 0 ? (
                                        <div className="empty-folder">
                                            {searchTerm ? 'No matching memories' : 'All memories are categorized'}
                                        </div>
                                    ) : (
                                        filteredUncategorizedMemories.map(memory => (
                                            <div 
                                                key={memory.nodeId}
                                                className="memory-file"
                                                onClick={() => handleMemoryClick(memory.nodeId)}
                                                onContextMenu={(e) => handleMemoryContextMenu(e, memory.nodeId)} // No categoryId for uncategorized
                                                title={memory.content}
                                                draggable={true}
                                                onDragStart={(e) => handleMemoryDragStart(e, memory.nodeId)} // No fromCategoryId for uncategorized
                                                onDragEnd={handleMemoryDragEnd}
                                            >
                                                <span className="file-icon">📄</span>
                                                <span className="file-name">
                                                    {truncateText(memory.content || 'Untitled')}
                                                </span>
                                                <span className="file-timestamp">
                                                    {memory.timestamp && formatTimestamp(memory.timestamp)}
                                                </span>
                                            </div>
                                        ))
                                    )}
                                </div>
                            )}
                        </div>

                        {/* Empty State */}
                        {filteredCategories.length === 0 && !isLoading && (
                            <div className="empty-state">
                                <p>No categories found</p>
                                {searchTerm && <p>Try adjusting your search term</p>}
                            </div>
                        )}
                    </>
                )}
            </div>

            {/* Context Menu */}
            {contextMenu && (
                <div 
                    className="context-menu"
                    style={{ 
                        position: 'fixed',
                        left: contextMenu.x,
                        top: contextMenu.y,
                        zIndex: 1000
                    }}
                    onClick={(e) => e.stopPropagation()}
                >
                    {contextMenu.type === 'memory' ? (
                        <>
                            <div className="context-menu-item" onClick={() => handleMemoryClick(contextMenu.itemId)}>
                                <span className="context-menu-icon">👁️</span>
                                View in Graph
                            </div>
                            {contextMenu.categoryId && (
                                <div 
                                    className="context-menu-item" 
                                    onClick={() => handleRemoveNodeFromCategory(contextMenu.itemId, contextMenu.categoryId!)}
                                >
                                    <span className="context-menu-icon">📤</span>
                                    Remove from Category
                                </div>
                            )}
                            <div className="context-menu-divider" />
                            <div 
                                className="context-menu-item danger" 
                                onClick={() => handleDeleteMemory(contextMenu.itemId, contextMenu.categoryId)}
                            >
                                <span className="context-menu-icon">🗑️</span>
                                Delete Memory
                            </div>
                        </>
                    ) : (
                        <>
                            <div className="context-menu-item" onClick={() => handleCategoryClick(contextMenu.itemId)}>
                                <span className="context-menu-icon">👁️</span>
                                View Category
                            </div>
                            <div className="context-menu-divider" />
                            <div 
                                className="context-menu-item danger" 
                                onClick={() => handleDeleteCategory(contextMenu.itemId)}
                            >
                                <span className="context-menu-icon">🗑️</span>
                                Delete Category
                            </div>
                        </>
                    )}
                </div>
            )}
        </div>
    );
};

export default FileSystemSidebar;