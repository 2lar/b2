/**
 * MemoryList Component - Paginated Memory Management Interface
 * 
 * Purpose:
 * Provides a comprehensive list view for browsing, editing, and managing memories.
 * Supports both full panel mode and compact slide-in panel mode.
 * Offers bulk operations, individual memory actions, and seamless integration with the graph view.
 * 
 * Key Features:
 * - Paginated display of memories with configurable page size
 * - Inline editing with click-to-edit functionality
 * - Bulk selection and deletion operations
 * - Individual memory actions (edit, delete, view in graph)
 * - Search and filtering capabilities
 * - Timestamp display with relative time formatting
 * - Loading states and error handling
 * - Responsive design for different screen sizes
 * - Panel mode for slide-in interface
 * 
 * Display Modes:
 * - Full mode: Traditional dashboard container layout
 * - Panel mode: Compact slide-in panel from right side
 * 
 * Memory Management:
 * - Click to edit memory content inline
 * - Bulk select with checkboxes for multiple operations
 * - Delete confirmation dialogs for safety
 * - Real-time updates after edit/delete operations
 * - Integration with graph visualization for memory viewing
 * 
 * Pagination:
 * - Configurable page size (default 50 memories per page)
 * - Navigation controls with page numbers
 * - Total count display
 * - Efficient loading of large memory collections
 * 
 * State Management:
 * - editingId: Currently editing memory ID
 * - editContent: Content being edited
 * - selectedMemories: Set of selected memory IDs for bulk operations
 * - isDeleting: Loading state during delete operations
 * 
 * Integration:
 * - Receives paginated memory data from Dashboard
 * - Calls callbacks for data refresh after operations
 * - Integrates with GraphVisualization for "View in Graph" functionality
 * - Can be positioned in panel or slide-in mode
 */

import React, { useState, memo } from 'react';
import { nodesApi } from '../api/nodes';
import type { Node } from '../../../services';
import VirtualMemoryList from './VirtualMemoryList';
import { useDeleteMemory, useBulkDeleteMemories } from '../hooks/useDeleteMemory';
import { useUpdateMemory } from '../hooks/useUpdateMemory';

interface MemoryListProps {
    /** Array of memory objects to display */
    memories: Node[];
    /** Total number of memories across all pages */
    totalMemories: number;
    /** Current page number (1-based) */
    currentPage: number;
    /** Total number of pages available */
    totalPages: number;
    /** Loading state indicator */
    isLoading: boolean;
    /** Callback for page navigation */
    onPageChange: (page: number) => void;
    /** Callback after memory deletion */
    onMemoryDeleted: () => void;
    /** Callback after memory update */
    onMemoryUpdated: () => void;
    /** Optional callback to view memory in graph visualization */
    onMemoryViewInGraph?: (nodeId: string) => void;
    /** Whether component is rendered in a slide-in panel */
    isInPanel?: boolean;
    /** Whether to use virtual scrolling for better performance */
    useVirtualScrolling?: boolean;
}

const MemoryList: React.FC<MemoryListProps> = ({
    memories,
    totalMemories,
    currentPage,
    totalPages,
    isLoading,
    onPageChange,
    onMemoryDeleted,
    onMemoryUpdated,
    onMemoryViewInGraph,
    isInPanel = false,
    useVirtualScrolling = false
}) => {
    // Use virtual scrolling for better performance with large lists
    if (useVirtualScrolling) {
        return (
            <VirtualMemoryList
                memories={memories}
                totalMemories={totalMemories}
                currentPage={currentPage}
                totalPages={totalPages}
                isLoading={isLoading}
                onPageChange={onPageChange}
                onMemoryDeleted={onMemoryDeleted}
                onMemoryUpdated={onMemoryUpdated}
                onMemoryViewInGraph={onMemoryViewInGraph}
                isInPanel={isInPanel}
            />
        );
    }
    const [editingId, setEditingId] = useState<string | null>(null);
    const [editContent, setEditContent] = useState('');
    const [editTitle, setEditTitle] = useState('');
    const [selectedMemories, setSelectedMemories] = useState<Set<string>>(new Set());
    
    // Optimistic mutation hooks
    const updateMemoryMutation = useUpdateMemory();
    const deleteMemoryMutation = useDeleteMemory();
    const bulkDeleteMutation = useBulkDeleteMemories();

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

    const handleEdit = (memory: Node) => {
        setEditingId(memory.nodeId || null);
        setEditContent(memory.content || '');
        setEditTitle(memory.title || '');
    };

    const handleSave = async (nodeId: string) => {
        if (!editContent.trim()) return;

        updateMemoryMutation.mutate(
            { 
                nodeId, 
                content: editContent.trim(),
                title: editTitle.trim() || undefined
            },
            {
                onSuccess: () => {
                    setEditingId(null);
                    setEditContent('');
                    setEditTitle('');
                    onMemoryUpdated();
                },
                onError: (error) => {
                    console.error('Failed to update memory:', error);
                    // Keep editing mode active on error
                }
            }
        );
    };

    const handleCancel = () => {
        setEditingId(null);
        setEditContent('');
        setEditTitle('');
    };

    const handleDelete = async (nodeId: string) => {
        if (!confirm('Are you sure you want to delete this memory? This cannot be undone.')) {
            return;
        }

        deleteMemoryMutation.mutate(
            { nodeId },
            {
                onSuccess: () => {
                    onMemoryDeleted();
                },
                onError: (error) => {
                    console.error('Failed to delete memory:', error);
                }
            }
        );
    };

    const handleSelectAll = () => {
        if (selectedMemories.size === memories.length) {
            setSelectedMemories(new Set());
        } else {
            setSelectedMemories(new Set(memories.map(m => m.nodeId || '')));
        }
    };

    const handleSelectMemory = (nodeId: string) => {
        const newSelected = new Set(selectedMemories);
        if (newSelected.has(nodeId)) {
            newSelected.delete(nodeId);
        } else {
            newSelected.add(nodeId);
        }
        setSelectedMemories(newSelected);
    };

    const handleBulkDelete = async () => {
        const selectedIds = Array.from(selectedMemories).filter(id => id);
        if (selectedIds.length === 0) return;

        const message = selectedIds.length === 1 
            ? 'Are you sure you want to delete this memory? This cannot be undone.'
            : `Are you sure you want to delete ${selectedIds.length} memories? This cannot be undone.`;

        if (!confirm(message)) return;

        bulkDeleteMutation.mutate(selectedIds, {
            onSuccess: () => {
                setSelectedMemories(new Set());
                onMemoryDeleted();
            },
            onError: (error) => {
                console.error('Failed to bulk delete memories:', error);
            }
        });
    };

    if (isInPanel) {
        return (
            <div className="memory-list-panel-content">
                <div className="memory-list">
                    {memories.length > 0 && (
                        <div className="memory-list-controls">
                            <div className="controls-main">
                                <label className="checkbox-container">
                                    <input 
                                        type="checkbox" 
                                        checked={selectedMemories.size === memories.length && memories.length > 0}
                                        onChange={handleSelectAll}
                                        className="select-all-checkbox"
                                    />
                                    <span className="checkmark"></span>
                                    Select All (Page)
                                </label>
                                <span className="total-count">
                                    {totalMemories} {totalMemories === 1 ? 'memory' : 'memories'}
                                </span>
                            </div>
                            {selectedMemories.size > 0 && (
                                <div className="controls-footer">
                                    <button 
                                        className="danger-btn bulk-delete-btn" 
                                        onClick={handleBulkDelete}
                                        disabled={bulkDeleteMutation.isPending}
                                    >
                                        {bulkDeleteMutation.isPending ? 'Deleting...' : `Delete ${selectedMemories.size} Selected`}
                                    </button>
                                </div>
                            )}
                        </div>
                    )}

                    {isLoading ? (
                        <div className="empty-state">Loading memories...</div>
                    ) : memories.length === 0 ? (
                        <div className="empty-state">No memories yet. Create your first memory above!</div>
                    ) : (
                        memories.map(memory => (
                            <div 
                                key={memory.nodeId} 
                                className={`memory-item ${onMemoryViewInGraph ? 'memory-item-clickable' : ''}`}
                                data-node-id={memory.nodeId}
                                onClick={onMemoryViewInGraph && editingId !== memory.nodeId ? () => onMemoryViewInGraph(memory.nodeId || '') : undefined}
                                style={onMemoryViewInGraph && editingId !== memory.nodeId ? { cursor: 'pointer' } : undefined}
                            >
                                <div className="memory-item-header">
                                    <label className="checkbox-container" onClick={(e) => e.stopPropagation()}>
                                        <input 
                                            type="checkbox" 
                                            checked={selectedMemories.has(memory.nodeId || '')}
                                            onChange={(e) => {
                                                e.stopPropagation();
                                                handleSelectMemory(memory.nodeId || '');
                                            }}
                                            className="memory-checkbox" 
                                        />
                                        <span className="checkmark"></span>
                                    </label>
                                    <div className="memory-item-content">
                                        {editingId === memory.nodeId ? (
                                            <>
                                                <input 
                                                    type="text"
                                                    value={editTitle}
                                                    onChange={(e) => setEditTitle(e.target.value)}
                                                    onClick={(e) => e.stopPropagation()}
                                                    placeholder="Title (optional)"
                                                    className="edit-title-input"
                                                    autoFocus
                                                />
                                                <textarea 
                                                    value={editContent}
                                                    onChange={(e) => setEditContent(e.target.value)}
                                                    onClick={(e) => e.stopPropagation()}
                                                    className="edit-textarea"
                                                />
                                            </>
                                        ) : (
                                            <>
                                                {memory.title && (
                                                    <div className="memory-title">
                                                        {memory.title}
                                                    </div>
                                                )}
                                                <div className="memory-content">
                                                    {memory.title 
                                                        ? (memory.content?.length > 200 ? memory.content.substring(0, 200) + '...' : memory.content)
                                                        : memory.content || ''
                                                    }
                                                </div>
                                            </>
                                        )}
                                    </div>
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
                                <div className="memory-item-meta">
                                    {formatDate(memory.timestamp || '')}
                                </div>
                                <div className="memory-item-actions">
                                    {editingId === memory.nodeId ? (
                                        <>
                                            <button 
                                                className="primary-btn save-btn"
                                                onClick={(e) => {
                                                    e.stopPropagation();
                                                    handleSave(memory.nodeId || '');
                                                }}
                                            >
                                                Save
                                            </button>
                                            <button 
                                                className="secondary-btn cancel-btn"
                                                onClick={(e) => {
                                                    e.stopPropagation();
                                                    handleCancel();
                                                }}
                                            >
                                                Cancel
                                            </button>
                                        </>
                                    ) : (
                                        <>
                                            <button 
                                                className="secondary-btn edit-btn"
                                                onClick={(e) => {
                                                    e.stopPropagation();
                                                    handleEdit(memory);
                                                }}
                                            >
                                                Edit
                                            </button>
                                            <button 
                                                className="danger-btn delete-btn"
                                                onClick={(e) => {
                                                    e.stopPropagation();
                                                    handleDelete(memory.nodeId || '');
                                                }}
                                            >
                                                Delete
                                            </button>
                                        </>
                                    )}
                                </div>
                            </div>
                        ))
                    )}
                </div>

                {totalPages > 1 && (
                    <div className="pagination-controls">
                        <button 
                            className="pagination-btn" 
                            onClick={() => onPageChange(currentPage - 1)}
                            disabled={currentPage <= 1}
                        >
                            ← Previous
                        </button>
                        <span id="page-info">
                            Page {currentPage} of {totalPages}
                        </span>
                        <button 
                            className="pagination-btn" 
                            onClick={() => onPageChange(currentPage + 1)}
                            disabled={currentPage >= totalPages}
                        >
                            Next →
                        </button>
                    </div>
                )}
            </div>
        );
    }

    return (
        <div className="dashboard-container" id="list-container" data-container="list">
            <div className="container-header" data-drag-handle>
                <span className="container-title">All Memories</span>
                <div className="container-controls">
                    <span id="memory-count">
                        {totalMemories === 1 ? '1 memory' : `${totalMemories} memories`}
                    </span>
                    <span className="drag-handle">⋮⋮</span>
                </div>
            </div>
            <div className="container-content">
                <div className="memory-list">
                    {memories.length > 0 && (
                        <div className="memory-list-controls">
                            <div className="controls-main">
                                <label className="checkbox-container">
                                    <input 
                                        type="checkbox" 
                                        checked={selectedMemories.size === memories.length && memories.length > 0}
                                        onChange={handleSelectAll}
                                        className="select-all-checkbox"
                                    />
                                    <span className="checkmark"></span>
                                    Select All (Page)
                                </label>
                                <span className="total-count">
                                    {totalMemories} {totalMemories === 1 ? 'memory' : 'memories'}
                                </span>
                            </div>
                            {selectedMemories.size > 0 && (
                                <div className="controls-footer">
                                    <button 
                                        className="danger-btn bulk-delete-btn" 
                                        onClick={handleBulkDelete}
                                        disabled={bulkDeleteMutation.isPending}
                                    >
                                        {bulkDeleteMutation.isPending ? 'Deleting...' : `Delete ${selectedMemories.size} Selected`}
                                    </button>
                                </div>
                            )}
                        </div>
                    )}

                    {isLoading ? (
                        <div className="empty-state">Loading memories...</div>
                    ) : memories.length === 0 ? (
                        <div className="empty-state">No memories yet. Create your first memory above!</div>
                    ) : (
                        memories.map(memory => (
                            <div 
                                key={memory.nodeId} 
                                className={`memory-item ${onMemoryViewInGraph ? 'memory-item-clickable' : ''}`}
                                data-node-id={memory.nodeId}
                                onClick={onMemoryViewInGraph && editingId !== memory.nodeId ? () => onMemoryViewInGraph(memory.nodeId || '') : undefined}
                                style={onMemoryViewInGraph && editingId !== memory.nodeId ? { cursor: 'pointer' } : undefined}
                            >
                                <div className="memory-item-header">
                                    <label className="checkbox-container" onClick={(e) => e.stopPropagation()}>
                                        <input 
                                            type="checkbox" 
                                            checked={selectedMemories.has(memory.nodeId || '')}
                                            onChange={(e) => {
                                                e.stopPropagation();
                                                handleSelectMemory(memory.nodeId || '');
                                            }}
                                            className="memory-checkbox" 
                                        />
                                        <span className="checkmark"></span>
                                    </label>
                                    <div className="memory-item-content">
                                        {editingId === memory.nodeId ? (
                                            <>
                                                <input 
                                                    type="text"
                                                    value={editTitle}
                                                    onChange={(e) => setEditTitle(e.target.value)}
                                                    onClick={(e) => e.stopPropagation()}
                                                    placeholder="Title (optional)"
                                                    className="edit-title-input"
                                                    autoFocus
                                                />
                                                <textarea 
                                                    value={editContent}
                                                    onChange={(e) => setEditContent(e.target.value)}
                                                    onClick={(e) => e.stopPropagation()}
                                                    className="edit-textarea"
                                                />
                                            </>
                                        ) : (
                                            <>
                                                {memory.title && (
                                                    <div className="memory-title">
                                                        {memory.title}
                                                    </div>
                                                )}
                                                <div className="memory-content">
                                                    {memory.title 
                                                        ? (memory.content?.length > 200 ? memory.content.substring(0, 200) + '...' : memory.content)
                                                        : memory.content || ''
                                                    }
                                                </div>
                                            </>
                                        )}
                                    </div>
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
                                <div className="memory-item-meta">
                                    {formatDate(memory.timestamp || '')}
                                </div>
                                <div className="memory-item-actions">
                                    {editingId === memory.nodeId ? (
                                        <>
                                            <button 
                                                className="primary-btn save-btn"
                                                onClick={(e) => {
                                                    e.stopPropagation();
                                                    handleSave(memory.nodeId || '');
                                                }}
                                            >
                                                Save
                                            </button>
                                            <button 
                                                className="secondary-btn cancel-btn"
                                                onClick={(e) => {
                                                    e.stopPropagation();
                                                    handleCancel();
                                                }}
                                            >
                                                Cancel
                                            </button>
                                        </>
                                    ) : (
                                        <>
                                            <button 
                                                className="secondary-btn edit-btn"
                                                onClick={(e) => {
                                                    e.stopPropagation();
                                                    handleEdit(memory);
                                                }}
                                            >
                                                Edit
                                            </button>
                                            <button 
                                                className="danger-btn delete-btn"
                                                onClick={(e) => {
                                                    e.stopPropagation();
                                                    handleDelete(memory.nodeId || '');
                                                }}
                                            >
                                                Delete
                                            </button>
                                        </>
                                    )}
                                </div>
                            </div>
                        ))
                    )}
                </div>

                {totalPages > 1 && (
                    <div className="pagination-controls">
                        <button 
                            className="pagination-btn" 
                            onClick={() => onPageChange(currentPage - 1)}
                            disabled={currentPage <= 1}
                        >
                            ← Previous
                        </button>
                        <span id="page-info">
                            Page {currentPage} of {totalPages}
                        </span>
                        <button 
                            className="pagination-btn" 
                            onClick={() => onPageChange(currentPage + 1)}
                            disabled={currentPage >= totalPages}
                        >
                            Next →
                        </button>
                    </div>
                )}
            </div>
        </div>
    );
};

// Optimize with React.memo to prevent unnecessary re-renders
export default memo(MemoryList);