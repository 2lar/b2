import React, { useMemo, useState } from 'react';
import { useQueryClient } from '@tanstack/react-query';
import type { Category } from '../../../services';
import { useCategories, categoriesQueryKey } from '../../categories/hooks/useCategories';
import { useCategoryNodes } from '../../categories/hooks/useCategoryNodes';
import { useRecentMemories, recentMemoriesQueryKey } from '../hooks/useRecentMemories';
import styles from './FileSystemSidebar.module.css';

interface FileSystemSidebarProps {
    onMemorySelect: (nodeId: string) => void;
    onCategorySelect: (categoryId: string) => void;
    refreshTrigger?: number;
    isCollapsed?: boolean;
}

const MAX_CATEGORY_PREVIEW = 5;

const formatTimestamp = (timestamp?: string): string => {
    if (!timestamp) {
        return '';
    }
    const date = new Date(timestamp);
    return new Intl.DateTimeFormat(undefined, {
        dateStyle: 'medium',
        timeStyle: 'short',
    }).format(date);
};

export const FileSystemSidebar: React.FC<FileSystemSidebarProps> = ({
    onCategorySelect,
    onMemorySelect,
    refreshTrigger,
    isCollapsed = false,
}) => {
    const [expandedCategory, setExpandedCategory] = useState<string | null>(null);
    const queryClient = useQueryClient();

    const { categories, isFetching: isFetchingCategories, isError: categoriesError } = useCategories();
    const {
        nodes: categoryNodes,
        isLoading: isLoadingCategoryNodes,
        refetch: refetchCategoryNodes,
    } = useCategoryNodes(expandedCategory, MAX_CATEGORY_PREVIEW);
    const {
        memories: recentMemories,
        isFetching: isFetchingRecent,
        refetch: refetchRecent,
    } = useRecentMemories(MAX_CATEGORY_PREVIEW);

    React.useEffect(() => {
        if (refreshTrigger !== undefined) {
            queryClient.invalidateQueries({ queryKey: categoriesQueryKey });
            queryClient.invalidateQueries({ queryKey: recentMemoriesQueryKey, exact: false });
            void refetchRecent();
            if (expandedCategory) {
                refetchCategoryNodes();
            }
        }
    }, [expandedCategory, queryClient, refetchCategoryNodes, refetchRecent, refreshTrigger]);

    const previewNodes = useMemo(() => {
        if (!expandedCategory) {
            return [];
        }
        return categoryNodes.slice(0, MAX_CATEGORY_PREVIEW);
    }, [expandedCategory, categoryNodes]);

    if (isCollapsed) {
        return null;
    }

    return (
        <div className={styles.sidebar}>
            <section className={styles.section}>
                <header className={styles.sectionHeader}>
                    <h3 className={styles.sectionTitle}>Categories</h3>
                    {isFetchingCategories && <span className={styles.spinner} aria-label="Loading categories" />}
                </header>
                {categoriesError && (
                    <p className={styles.error}>Unable to load categories right now.</p>
                )}
                <ul className={styles.categoryList}>
                    {categories.map((category: Category) => {
                        const isExpanded = expandedCategory === category.id;
                        return (
                            <li key={category.id}>
                                <button
                                    type="button"
                                    className={styles.categoryButton}
                                    onClick={() => {
                                        onCategorySelect(category.id);
                                        setExpandedCategory(prev => (prev === category.id ? null : category.id));
                                    }}
                                >
                                    <span className={styles.categoryName}>{category.title}</span>
                                    {typeof category.noteCount === 'number' && (
                                        <span className={styles.badge}>{category.noteCount}</span>
                                    )}
                                </button>
                                {isExpanded && (
                                    <div className={styles.previewPanel}>
                                        {isLoadingCategoryNodes && <p className={styles.subtle}>Loading memories…</p>}
                                        {!isLoadingCategoryNodes && previewNodes.length === 0 && (
                                            <p className={styles.subtle}>No memories in this category yet.</p>
                                        )}
                                        <ul className={styles.nodeList}>
                                            {previewNodes.map(node => (
                                                <li key={node.nodeId}>
                                                    <button
                                                        type="button"
                                                        className={styles.nodeButton}
                                                        onClick={() => onMemorySelect(node.nodeId)}
                                                    >
                                                        <span className={styles.nodeTitle}>{node.title || 'Untitled memory'}</span>
                                                        <span className={styles.nodeMeta}>{formatTimestamp(node.timestamp)}</span>
                                                    </button>
                                                </li>
                                            ))}
                                        </ul>
                                    </div>
                                )}
                            </li>
                        );
                    })}
                </ul>
            </section>

            <section className={styles.section}>
                <header className={styles.sectionHeader}>
                    <h3 className={styles.sectionTitle}>Recent Memories</h3>
                    {isFetchingRecent && <span className={styles.spinner} aria-label="Loading recent memories" />}
                </header>
                <ul className={styles.nodeList}>
                    {recentMemories.map(node => (
                        <li key={node.nodeId}>
                            <button
                                type="button"
                                className={styles.nodeButton}
                                onClick={() => onMemorySelect(node.nodeId)}
                            >
                                <span className={styles.nodeTitle}>{node.title || 'Untitled memory'}</span>
                                <span className={styles.nodeMeta}>{formatTimestamp(node.timestamp)}</span>
                            </button>
                        </li>
                    ))}
                </ul>
            </section>
        </div>
    );
};

export default FileSystemSidebar;
