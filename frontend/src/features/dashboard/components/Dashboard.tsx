import React from 'react';
import { useNavigate } from 'react-router-dom';
import { User } from '@supabase/supabase-js';
import { Header, LeftPanel, NotificationBanner } from '../../../common';
import { GraphVisualization, type GraphVisualizationRef, MemoryInput } from '../../memories';
import { useDashboardData } from '../hooks/useDashboardData';

interface DashboardProps {
    user: User;
    onSignOut: () => void;
}

const Dashboard: React.FC<DashboardProps> = ({ user, onSignOut }) => {
    const navigate = useNavigate();
    const graphRef = React.useRef<GraphVisualizationRef>(null);
    const [isLeftPanelCollapsed, setIsLeftPanelCollapsed] = React.useState(() => {
        if (typeof window === 'undefined') {
            return false;
        }
        return window.innerWidth <= 768;
    });

    const {
        memories,
        pagination,
        isLoading,
        error,
        graphRefreshKey,
        sidebarRefreshKey,
        onPageChange,
        onMemoryCreated,
        onMemoryDeleted,
        onMemoryUpdated,
        clearError,
    } = useDashboardData();

    const handleToggleLeftPanel = React.useCallback(() => {
        setIsLeftPanelCollapsed(previous => !previous);
    }, []);

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

    const handlePageChange = React.useCallback((page: number) => {
        onPageChange(page);
    }, [onPageChange]);

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
        <div className="app-container">
            <Header
                userEmail={user.email || ''}
                onSignOut={onSignOut}
                onToggleSidebar={handleToggleLeftPanel}
                isSidebarCollapsed={isLeftPanelCollapsed}
                memoryCount={pagination.totalMemories}
            />

            <main className="dashboard-layout-mobile-ready">
                <LeftPanel
                    user={user}
                    isCollapsed={isLeftPanelCollapsed}
                    onToggleCollapse={handleToggleLeftPanel}
                    onMemorySelect={handleViewInGraph}
                    onCategorySelect={(categoryId) => navigate(`/categories/${categoryId}`)}
                    refreshTrigger={sidebarRefreshKey}
                    memories={memories}
                    totalMemories={pagination.totalMemories}
                    currentPage={pagination.currentPage}
                    totalPages={pagination.totalPages}
                    isLoading={isLoading}
                    onPageChange={handlePageChange}
                    onMemoryDeleted={handleDeleteMemory}
                    onMemoryUpdated={handleUpdateMemory}
                    useVirtualScrolling={pagination.totalMemories > 100}
                />

                <div className="main-content-area">
                    {error && (
                        <NotificationBanner
                            variant="error"
                            message={error}
                            onDismiss={clearError}
                        />
                    )}

                    <div className="graph-container">
                        <GraphVisualization
                            ref={graphRef}
                            refreshTrigger={graphRefreshKey}
                            hasOverlayInput={true}
                        />
                    </div>

                    <div className="memory-input-container">
                        <div className="memory-input-overlay desktop-input">
                            <MemoryInput
                                onMemoryCreated={handleCreateMemory}
                                isCompact={true}
                                onDocumentModeOpen={handleDocumentModeOpen}
                            />
                        </div>
                    </div>

                    <div className="mobile-memory-input">
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
