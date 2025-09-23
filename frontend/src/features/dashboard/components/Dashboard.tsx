import React from 'react';
import { useNavigate } from 'react-router-dom';
import { User } from '@supabase/supabase-js';
import { Header, LeftPanel, NotificationBanner } from '../../../common';
import { GraphVisualization, type GraphVisualizationRef, MemoryInput } from '../../memories';
import { useDashboardData } from '../hooks/useDashboardData';
import { useLayoutStore } from '../../../stores/layoutStore';
import styles from './Dashboard.module.css';

interface DashboardProps {
    user: User;
    onSignOut: () => void;
}

const Dashboard: React.FC<DashboardProps> = ({ user, onSignOut }) => {
    const navigate = useNavigate();
    const graphRef = React.useRef<GraphVisualizationRef>(null);
    const isLeftPanelOpen = useLayoutStore(state => state.isLeftPanelOpen);
    const toggleLeftPanel = useLayoutStore(state => state.toggleLeftPanel);
    const setLeftPanelOpen = useLayoutStore(state => state.setLeftPanelOpen);

    const {
        memories,
        totalMemories,
        isLoading,
        isFetchingMore,
        hasMore,
        loadMore,
        error,
        graphRefreshKey,
        sidebarRefreshKey,
        onMemoryCreated,
        onMemoryDeleted,
        onMemoryUpdated,
        clearError,
    } = useDashboardData();

    const handleToggleLeftPanel = React.useCallback(() => {
        toggleLeftPanel();
    }, [toggleLeftPanel]);

    React.useEffect(() => {
        if (typeof window === 'undefined') {
            return;
        }
        setLeftPanelOpen(window.innerWidth > 768);
    }, [setLeftPanelOpen]);

    const handleViewInGraph = React.useCallback((nodeId: string) => {
        if (!graphRef.current) {
            return;
        }
        const success = graphRef.current.selectAndCenterNode(nodeId);
        if (!success) {
            console.warn('Could not find node in graph. The graph may still be loading.');
        }
    }, []);

    const handleDocumentModeOpen = React.useCallback(() => {
        graphRef.current?.hideNodeDetails();
    }, []);

    const handleCreateMemory = React.useCallback(() => {
        void onMemoryCreated();
    }, [onMemoryCreated]);

    const handleDeleteMemory = React.useCallback(() => {
        void onMemoryDeleted();
    }, [onMemoryDeleted]);

    const handleUpdateMemory = React.useCallback(() => {
        void onMemoryUpdated();
    }, [onMemoryUpdated]);

    return (
        <div className={styles.page}>
            <Header
                userEmail={user.email || ''}
                onSignOut={onSignOut}
                onToggleSidebar={handleToggleLeftPanel}
                isSidebarCollapsed={!isLeftPanelOpen}
                memoryCount={totalMemories}
            />

            <main className={isLeftPanelOpen ? styles.layout : styles.layoutCollapsed}>
                <LeftPanel
                    isCollapsed={!isLeftPanelOpen}
                    onToggleCollapse={handleToggleLeftPanel}
                    onMemorySelect={handleViewInGraph}
                    onCategorySelect={(categoryId) => navigate(`/categories/${categoryId}`)}
                    refreshTrigger={sidebarRefreshKey}
                    memories={memories}
                    isLoading={isLoading}
                    onMemoryDeleted={handleDeleteMemory}
                    onMemoryUpdated={handleUpdateMemory}
                    totalMemories={totalMemories}
                    hasMore={hasMore}
                    isFetchingMore={isFetchingMore}
                    onLoadMore={loadMore}
                    useVirtualScrolling={totalMemories > 100}
                />

                <div className={styles.main}>
                    {error && (
                        <NotificationBanner
                            variant="error"
                            message={error}
                            onDismiss={clearError}
                        />
                    )}

                    <div className={styles.graphSection}>
                        <GraphVisualization
                            ref={graphRef}
                            refreshTrigger={graphRefreshKey}
                            hasOverlayInput={true}
                        />
                        <div className={styles.memoryInputOverlay}>
                            <MemoryInput
                                onMemoryCreated={handleCreateMemory}
                                isCompact={true}
                                onDocumentModeOpen={handleDocumentModeOpen}
                            />
                        </div>
                    </div>

                    <div className={styles.mobileInput}>
                        <MemoryInput
                            onMemoryCreated={handleCreateMemory}
                            isCompact={true}
                            isMobile={true}
                            onDocumentModeOpen={handleDocumentModeOpen}
                        />
                    </div>
                </div>
            </main>
        </div>
    );
};

export default Dashboard;
