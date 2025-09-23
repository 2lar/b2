import React, { useMemo, useRef, useState } from 'react';
import { useVirtualizer } from '@tanstack/react-virtual';
import { useDeleteMemory, useBulkDeleteMemories } from '../hooks/useDeleteMemory';
import { useUpdateMemory } from '../hooks/useUpdateMemory';
import type { Node } from '../../../services';
import styles from './MemoryList.module.css';

type VirtualMemoryListProps = {
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
    estimatedItemHeight?: number;
    overscan?: number;
};

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

const formatDate = (value?: string) => (value ? new Date(value).toLocaleString() : '');

const VirtualMemoryList: React.FC<VirtualMemoryListProps> = ({
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
    estimatedItemHeight = 120,
    overscan = 5,
}) => {
    const [editing, setEditing] = useState<EditingState>(INITIAL_EDITING);
    const [selectedIds, setSelectedIds] = useState<Set<string>>(new Set());

    const updateMemoryMutation = useUpdateMemory();
    const deleteMemoryMutation = useDeleteMemory();
    const bulkDeleteMutation = useBulkDeleteMemories();

    const parentRef = useRef<HTMLDivElement>(null);
    const virtualizer = useVirtualizer({
        count: memories.length,
        getScrollElement: () => parentRef.current,
        estimateSize: () => estimatedItemHeight,
        overscan,
        getItemKey: index => memories[index]?.nodeId || index,
    });

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

            <div ref={parentRef} className={styles.listContainer} style={{ overflowY: 'auto' }}>
                {isLoading && memories.length === 0 ? (
                    <div className={styles.emptyState}>Loading memories…</div>
                ) : memories.length === 0 ? (
                    <div className={styles.emptyState}>No memories yet. Create your first memory above!</div>
                ) : (
                    <div
                        style={{
                            height: `${virtualizer.getTotalSize()}px`,
                            position: 'relative',
                        }}
                    >
                        {virtualizer.getVirtualItems().map(item => {
                            const memory = memories[item.index];
                            const nodeId = memory?.nodeId || '';
                            const isEditing = editing.id === nodeId;
                            const isSelected = selectedIds.has(nodeId);
                            const createdAt = memory ? (memory as any).createdAt ?? memory.timestamp : '';
                        const updatedAt = memory ? (memory as any).updatedAt ?? memory.timestamp : '';

                        return (
                            <article
                                key={item.key}
                                className={styles.row}
                                style={{
                                    position: 'absolute',
                                    top: 0,
                                    left: 0,
                                    width: '100%',
                                    transform: `translateY(${item.start}px)`
                                }}
                            >
                                <label>
                                    <input
                                        type="checkbox"
                                        className={styles.checkbox}
                                        checked={isSelected}
                                        onChange={() => toggleSelection(nodeId)}
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
                                            <div className={styles.title}>{memory?.title || 'Untitled memory'}</div>
                                            <div className={styles.content}>{memory?.content}</div>
                                            {memory?.tags && memory.tags.length > 0 && (
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
                                            <button type="button" className={styles.secondaryButton} onClick={cancelEditing}>
                                                Cancel
                                            </button>
                                        </>
                                    ) : (
                                        <>
                                            <button type="button" className={styles.secondaryButton} onClick={() => startEditing(memory)}>
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

export default VirtualMemoryList;
