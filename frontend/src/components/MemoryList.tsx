import React, { useState } from 'react';
import { api } from '../ts/apiClient';
import { components } from '../ts/generated-types';

type Node = components['schemas']['Node'];

interface MemoryListProps {
    memories: Node[];
    totalMemories: number;
    currentPage: number;
    totalPages: number;
    isLoading: boolean;
    onPageChange: (page: number) => void;
    onMemoryDeleted: () => void;
    onMemoryUpdated: () => void;
    onMemoryViewInGraph?: (nodeId: string) => void;
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
    onMemoryViewInGraph
}) => {
    const [editingId, setEditingId] = useState<string | null>(null);
    const [editContent, setEditContent] = useState('');
    const [selectedMemories, setSelectedMemories] = useState<Set<string>>(new Set());
    const [isDeleting, setIsDeleting] = useState(false);

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
    };

    const handleSave = async (nodeId: string) => {
        if (!editContent.trim()) return;

        try {
            await api.updateNode(nodeId, editContent.trim());
            setEditingId(null);
            setEditContent('');
            onMemoryUpdated();
        } catch (error) {
            console.error('Failed to update memory:', error);
        }
    };

    const handleCancel = () => {
        setEditingId(null);
        setEditContent('');
    };

    const handleDelete = async (nodeId: string) => {
        if (!confirm('Are you sure you want to delete this memory? This cannot be undone.')) {
            return;
        }

        try {
            await api.deleteNode(nodeId);
            onMemoryDeleted();
        } catch (error) {
            console.error('Failed to delete memory:', error);
        }
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

        setIsDeleting(true);
        try {
            await api.bulkDeleteNodes(selectedIds);
            setSelectedMemories(new Set());
            onMemoryDeleted();
        } catch (error) {
            console.error('Failed to bulk delete memories:', error);
        } finally {
            setIsDeleting(false);
        }
    };

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
                            <div className="select-controls">
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
                                <span className="selected-count">
                                    {selectedMemories.size} selected
                                </span>
                            </div>
                            <div className="bulk-actions">
                                <button 
                                    className="danger-btn bulk-delete-btn" 
                                    onClick={handleBulkDelete}
                                    disabled={selectedMemories.size === 0 || isDeleting}
                                >
                                    {isDeleting ? 'Deleting...' : 'Delete Selected'}
                                </button>
                            </div>
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
                                            <textarea 
                                                value={editContent}
                                                onChange={(e) => setEditContent(e.target.value)}
                                                onClick={(e) => e.stopPropagation()}
                                                className="edit-textarea"
                                                autoFocus
                                            />
                                        ) : (
                                            memory.content || ''
                                        )}
                                    </div>
                                </div>
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

export default MemoryList;