import React, { useState } from 'react';
import { FileSystemSidebar, MemoryList } from '../../features/memories';
import type { Node } from '../../services';
import styles from './LeftPanel.module.css';

type TabType = 'explorer' | 'memories';

interface LeftPanelProps {
    isCollapsed: boolean;
    onToggleCollapse: () => void;
    onMemorySelect: (nodeId: string) => void;
    onCategorySelect: (categoryId: string) => void;
    refreshTrigger?: number;
    memories: Node[];
    totalMemories: number;
    isLoading: boolean;
    isFetchingMore?: boolean;
    hasMore?: boolean;
    onLoadMore?: () => Promise<void> | void;
    onMemoryDeleted: () => void;
    onMemoryUpdated: () => void;
    useVirtualScrolling?: boolean;
}

const LeftPanel: React.FC<LeftPanelProps> = ({
    isCollapsed,
    onToggleCollapse,
    onMemorySelect,
    onCategorySelect,
    refreshTrigger,
    memories,
    totalMemories,
    isLoading,
    isFetchingMore = false,
    hasMore = false,
    onLoadMore,
    onMemoryDeleted,
    onMemoryUpdated,
    useVirtualScrolling = false,
}) => {
    const [activeTab, setActiveTab] = useState<TabType>('memories');

    const changeTab = (tab: TabType) => {
        if (isCollapsed) {
            return;
        }
        setActiveTab(tab);
    };

    const renderTabs = () => (
        <div className={styles.tabs}>
            <button
                type="button"
                className={`${styles.tabButton} ${activeTab === 'explorer' ? styles.tabButtonActive : ''}`}
                onClick={() => changeTab('explorer')}
            >
                📁 Explorer
            </button>
            <button
                type="button"
                className={`${styles.tabButton} ${activeTab === 'memories' ? styles.tabButtonActive : ''}`}
                onClick={() => changeTab('memories')}
            >
                📋 Memories
                <span className={styles.tabCount}>({totalMemories})</span>
            </button>
        </div>
    );

    return (
        <>
            {!isCollapsed && <div className={styles.backdrop} onClick={onToggleCollapse} aria-hidden="true" />}

            <aside className={`${styles.panel} ${isCollapsed ? styles.panelCollapsed : ''}`}>
                <div className={styles.header}>
                    {!isCollapsed && renderTabs()}
                    <button
                        type="button"
                        className={styles.collapseToggle}
                        onClick={onToggleCollapse}
                        aria-label={isCollapsed ? 'Expand panel' : 'Collapse panel'}
                        aria-expanded={!isCollapsed}
                    >
                        {isCollapsed ? '▶' : '✕'}
                    </button>
                </div>

                <div className={styles.content}>
                    {!isCollapsed ? (
                        <div className={styles.scrollArea}>
                            {activeTab === 'explorer' ? (
                                <FileSystemSidebar
                                    onMemorySelect={onMemorySelect}
                                    onCategorySelect={onCategorySelect}
                                    refreshTrigger={refreshTrigger}
                                    isCollapsed={false}
                                />
                            ) : (
                                <MemoryList
                                    memories={memories}
                                    totalMemories={totalMemories}
                                    isLoading={isLoading}
                                    isFetchingMore={isFetchingMore}
                                    hasMore={hasMore}
                                    onLoadMore={onLoadMore}
                                    onMemoryDeleted={onMemoryDeleted}
                                    onMemoryUpdated={onMemoryUpdated}
                                    onMemoryViewInGraph={onMemorySelect}
                                    isInPanel
                                    useVirtualScrolling={useVirtualScrolling}
                                />
                            )}
                        </div>
                    ) : (
                        <div className={styles.collapsedSummary}>
                            <span>📁 / 📋</span>
                            <span>
                                <strong>{totalMemories}</strong> items
                            </span>
                        </div>
                    )}
                </div>
            </aside>
        </>
    );
};

export default LeftPanel;
