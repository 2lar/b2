import React, { useMemo, useState, memo } from 'react';
import type { Node } from '../../../services';
import { useDeleteMemory, useBulkDeleteMemories } from '../hooks/useDeleteMemory';
import { useUpdateMemory } from '../hooks/useUpdateMemory';
import VirtualMemoryList from './VirtualMemoryList';
import styles from './MemoryList.module.css';

interface MemoryListProps {
    memories: Node[];
    totalMemories: number;
    isLoading: boolean;
    isFetchingMore?: boolean;
    hasMore?: boolean;
    onLoadMore?: () => Promise<void> | void;
    onMemoryDeleted: () => void;
    onMemoryUpdated: () => void;
    onMemoryViewInGraph?: (nodeId: string) => void;
    isInPanel?: boolean;
    useVirtualScrolling?: boolean;
}

type EditingState = {
    id: string | null;
    title: string;
    content: string;
};

const INITIAL_EDITING: EditingState = {
    id: null,
    title: '',
    content: '',
};

const formatDate = (value?: string): string => {
    if (!value) {
        return '';
    }
    const date = new Date(value);
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

const MemoryList: React.FC<MemoryListProps> = ({
    memories,
    totalMemories,
    isLoading,
    isFetchingMore = false,
    hasMore = false,
    onLoadMore,
    onMemoryDeleted,
    onMemoryUpdated,
    onMemoryViewInGraph,
    isInPanel = false,
    useVirtualScrolling = false,
}) => {
    const [editing, setEditing] = useState<EditingState>(INITIAL_EDITING);
    const [selectedIds, setSelectedIds] = useState<Set<string>>(new Set());

    const updateMemoryMutation = useUpdateMemory();
    const deleteMemoryMutation = useDeleteMemory();
    const bulkDeleteMutation = useBulkDeleteMemories();

    const allIdsOnPage = useMemo(() => memories.map(memory => memory.nodeId || ''), [memories]);
    const allSelected = selectedIds.size > 0 && selectedIds.size === allIdsOnPage.length;

    const toggleAll = () => {
        if (allSelected) {
            setSelectedIds(new Set());
            return;
        }
        setSelectedIds(new Set(allIdsOnPage.filter(Boolean)));
    };

    const toggleSelection = (id: string) => {
        setSelectedIds(prev => {
            const next = new Set(prev);
            if (next.has(id)) {
                next.delete(id);
            } else {
                next.add(id);
            }
            return next;
        });
    };

    const startEditing = (memory: Node) => {
        setEditing({
            id: memory.nodeId || null,
            title: memory.title || '',
            content: memory.content || '',
        });
    };

    const cancelEditing = () => setEditing(INITIAL_EDITING);

    const saveEditing = (nodeId: string) => {
        if (!editing.content.trim()) {
            return;
        }
        updateMemoryMutation.mutate(
            {
                nodeId,
                content: editing.content.trim(),
                title: editing.title.trim() || undefined,
            },
            {
                onSuccess: () => {
                    setEditing(INITIAL_EDITING);
                    onMemoryUpdated();
                },
            },
        );
    };

    const deleteMemory = (nodeId: string) => {
        if (!confirm('Delete this memory? This action cannot be undone.')) {
            return;
        }
        deleteMemoryMutation.mutate(
            { nodeId },
            {
                onSuccess: () => {
                    setSelectedIds(prev => {
                        const next = new Set(prev);
                        next.delete(nodeId);
                        return next;
                    });
                    onMemoryDeleted();
                },
            },
        );
    };

    const bulkDelete = () => {
        const ids = Array.from(selectedIds);
        if (ids.length === 0) {
            return;
        }
        const message = ids.length === 1
            ? 'Delete this memory? This action cannot be undone.'
            : `Delete ${ids.length} memories? This action cannot be undone.`;
        if (!confirm(message)) {
            return;
        }
        bulkDeleteMutation.mutate(ids, {
            onSuccess: () => {
                setSelectedIds(new Set());
                onMemoryDeleted();
            },
        });
    };

    if (useVirtualScrolling) {
        return (
            <VirtualMemoryList
                memories={memories}
                totalMemories={totalMemories}
                isLoading={isLoading}
                isFetchingMore={isFetchingMore}
                hasMore={hasMore}
                onLoadMore={onLoadMore}
                onMemoryDeleted={onMemoryDeleted}
                onMemoryUpdated={onMemoryUpdated}
                onMemoryViewInGraph={onMemoryViewInGraph}
                isInPanel={isInPanel}
            />
        );
    }

    const containerClass = isInPanel ? styles.panelWrapper : styles.wrapper;

    return (
        <div className={containerClass}>
            <div className={styles.toolbar}>
                <div className={styles.toolbarMain}>
                    <label className={styles.selector}>
                        <input
                            type="checkbox"
                            className={styles.checkbox}
                            checked={allSelected}
                            onChange={toggleAll}
                            aria-label="Select all memories on this page"
                        />
                        <span>Select all ({memories.length})</span>
                    </label>
                    <span className={styles.summary}>Total memories: {totalMemories}</span>
                </div>
                <div className={styles.toolbarFooter}>
                    <button
                        type="button"
                        className={styles.dangerButton}
                        onClick={bulkDelete}
                        disabled={selectedIds.size === 0 || bulkDeleteMutation.isPending}
                    >
                        {bulkDeleteMutation.isPending ? 'Deleting…' : `Delete selected (${selectedIds.size})`}
                    </button>
                    <button
                        type="button"
                        className={styles.secondaryButton}
                        onClick={() => setSelectedIds(new Set())}
                        disabled={selectedIds.size === 0}
                    >
                        Clear selection
                    </button>
                </div>
            </div>

            <div className={styles.listContainer}>
                {isLoading ? (
                    <div className={styles.emptyState}>Loading memories…</div>
                ) : memories.length === 0 ? (
                    <div className={styles.emptyState}>No memories yet. Create your first memory above!</div>
                ) : (
                    <div className={styles.list}>
                        {memories.map(memory => {
                           const nodeId = memory.nodeId || '';
                           const isEditing = editing.id === nodeId;
                           const isSelected = selectedIds.has(nodeId);
                            const createdAt = (memory as any).createdAt ?? memory.timestamp;
                            const updatedAt = (memory as any).updatedAt ?? memory.timestamp;

                            return (
                                <article key={nodeId || memory.content} className={styles.row}>
                                    <label>
                                        <input
                                            type="checkbox"
                                            className={styles.checkbox}
                                            checked={isSelected}
                                            onChange={() => toggleSelection(nodeId)}
                                            aria-label={`Select memory ${memory.title || memory.content.substring(0, 30)}`}
                                        />
                                    </label>
                                    <div className={styles.rowMeta}>
                                        {isEditing ? (
                                            <div className={styles.inlineInputs}>
                                                <input
                                                    className={styles.titleInput}
                                                    value={editing.title}
                                                    onChange={(event) => setEditing(prev => ({ ...prev, title: event.target.value }))}
                                                    placeholder="Title"
                                                />
                                                <textarea
                                                    className={styles.contentInput}
                                                    value={editing.content}
                                                    onChange={(event) => setEditing(prev => ({ ...prev, content: event.target.value }))}
                                                    placeholder="Memory content"
                                                />
                                            </div>
                                        ) : (
                                            <button
                                                type="button"
                                                className={styles.rowButton}
                                                onClick={() => onMemoryViewInGraph?.(nodeId)}
                                            >
                                                <div className={styles.title}>{memory.title || 'Untitled memory'}</div>
                                                <div className={styles.content}>{memory.content}</div>
                                                {memory.tags && memory.tags.length > 0 && (
                                                    <div className={styles.tags}>
                                                        {memory.tags.map(tag => (
                                                            <span key={tag} className={styles.tag}>#{tag}</span>
                                                        ))}
                                                    </div>
                                                )}
                                            </button>
                                        )}
                                        <div className={styles.meta}>
                                            <span>Created {formatDate(createdAt)}</span>
                                            {' · '}
                                            <span>Updated {formatDate(updatedAt)}</span>
                                        </div>
                                    </div>
                                    <div className={styles.actions}>
                                        {isEditing ? (
                                            <>
                                                <button
                                                    type="button"
                                                    className={styles.primaryButton}
                                                    onClick={() => saveEditing(nodeId)}
                                                    disabled={updateMemoryMutation.isPending}
                                                >
                                                    {updateMemoryMutation.isPending ? 'Saving…' : 'Save'}
                                                </button>
                                                <button
                                                    type="button"
                                                    className={styles.secondaryButton}
                                                    onClick={cancelEditing}
                                                >
                                                    Cancel
                                                </button>
                                            </>
                                        ) : (
                                            <>
                                                <button
                                                    type="button"
                                                    className={styles.secondaryButton}
                                                    onClick={() => startEditing(memory)}
                                                >
                                                    Edit
                                                </button>
                                                <button
                                                    type="button"
                                                    className={styles.secondaryButton}
                                                    onClick={() => onMemoryViewInGraph?.(nodeId)}
                                                >
                                                    View in graph
                                                </button>
                                            </>
                                        )}
                                        <button
                                            type="button"
                                            className={styles.dangerButton}
                                            onClick={() => deleteMemory(nodeId)}
                                            disabled={deleteMemoryMutation.isPending}
                                        >
                                            {deleteMemoryMutation.isPending ? 'Deleting…' : 'Delete'}
                                        </button>
                                    </div>
                                </article>
                            );
                        })}
                    </div>
                )}
            </div>

            {hasMore && (
                <div className={styles.pagination}>
                    <button
                        type="button"
                        className={styles.pageButton}
                        onClick={() => onLoadMore?.()}
                        disabled={isFetchingMore}
                    >
                        {isFetchingMore ? 'Loading…' : 'Load more memories'}
                    </button>
                </div>
            )}
        </div>
    );
};

export default memo(MemoryList);
