import React, { useState } from 'react';
import { User } from '@supabase/supabase-js';
import { FileSystemSidebar, MemoryList } from '../../features/memories';
import type { Node } from '../../services';

type TabType = 'explorer' | 'memories';

interface LeftPanelProps {
    /** Authenticated user object */
    user: User;
    /** Whether panel is in collapsed state */
    isCollapsed: boolean;
    /** Callback to toggle panel collapse state */
    onToggleCollapse: () => void;
    
    // FileSystemSidebar props
    /** Callback when a memory is selected for viewing */
    onMemorySelect: (nodeId: string) => void;
    /** Callback when a category is selected for viewing */
    onCategorySelect: (categoryId: string) => void;
    /** Optional trigger number that causes refresh when changed */
    refreshTrigger?: number;
    
    // MemoryList props
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
    /** Whether to use virtual scrolling for better performance */
    useVirtualScrolling?: boolean;
}

const LeftPanel: React.FC<LeftPanelProps> = ({
    user,
    isCollapsed,
    onToggleCollapse,
    onMemorySelect,
    onCategorySelect,
    refreshTrigger,
    memories,
    totalMemories,
    currentPage,
    totalPages,
    isLoading,
    onPageChange,
    onMemoryDeleted,
    onMemoryUpdated,
    useVirtualScrolling = false
}) => {
    const [activeTab, setActiveTab] = useState<TabType>('memories');

    const handleTabChange = (tab: TabType) => {
        // Don't allow tab switching when collapsed
        if (!isCollapsed) {
            setActiveTab(tab);
        }
    };

    const handleKeyDown = (event: React.KeyboardEvent, tab: TabType) => {
        if (event.key === 'Enter' || event.key === ' ') {
            event.preventDefault();
            handleTabChange(tab);
        }
    };

    return (
        <>
            {/* Mobile Backdrop */}
            {!isCollapsed && (
                <div 
                    className="mobile-backdrop"
                    onClick={onToggleCollapse}
                    aria-hidden="true"
                />
            )}
            
            <div className={`left-panel ${isCollapsed ? 'collapsed' : 'expanded'}`}>
                {/* Panel Header with Tabs */}
                <div className="left-panel-header">
                    {!isCollapsed && (
                        <div className="left-panel-tabs">
                            <button
                                className={`tab-button ${activeTab === 'explorer' ? 'active' : ''}`}
                                onClick={() => handleTabChange('explorer')}
                                onKeyDown={(e) => handleKeyDown(e, 'explorer')}
                                title="File Explorer"
                                aria-label="File Explorer"
                            >
                                <span className="tab-icon">📁</span>
                                <span className="tab-label">Explorer</span>
                            </button>
                            <button
                                className={`tab-button ${activeTab === 'memories' ? 'active' : ''}`}
                                onClick={() => handleTabChange('memories')}
                                onKeyDown={(e) => handleKeyDown(e, 'memories')}
                                title={`Memory List (${totalMemories})`}
                                aria-label={`Memory List (${totalMemories} memories)`}
                            >
                                <span className="tab-icon">📋</span>
                                <span className="tab-label">
                                    Memories
                                    <span className="tab-count">({totalMemories})</span>
                                </span>
                            </button>
                        </div>
                    )}
                    
                    <button
                        className="collapse-toggle"
                        onClick={onToggleCollapse}
                        title={isCollapsed ? 'Expand Panel' : 'Collapse Panel'}
                        aria-label={isCollapsed ? 'Expand Panel' : 'Collapse Panel'}
                    >
                        {isCollapsed ? '▶️' : '❌'}
                    </button>
                </div>

                {/* Panel Content */}
                <div className="left-panel-content">
                    {!isCollapsed && (
                        <>
                            {activeTab === 'explorer' && (
                                <FileSystemSidebar
                                    userId={user.id}
                                    onMemorySelect={onMemorySelect}
                                    onCategorySelect={onCategorySelect}
                                    refreshTrigger={refreshTrigger}
                                    isCollapsed={false}
                                />
                            )}
                            
                            {activeTab === 'memories' && (
                                <MemoryList
                                    memories={memories}
                                    totalMemories={totalMemories}
                                    currentPage={currentPage}
                                    totalPages={totalPages}
                                    isLoading={isLoading}
                                    onPageChange={onPageChange}
                                    onMemoryDeleted={onMemoryDeleted}
                                    onMemoryUpdated={onMemoryUpdated}
                                    onMemoryViewInGraph={onMemorySelect}
                                    isInPanel={true}
                                    useVirtualScrolling={useVirtualScrolling}
                                />
                            )}
                        </>
                    )}
                    
                    {isCollapsed && (
                        <div className="collapsed-content">
                            <div className="collapsed-indicators">
                                <div 
                                    className={`collapsed-tab ${activeTab === 'explorer' ? 'active' : ''}`}
                                    title="File Explorer"
                                >
                                    📁
                                </div>
                                <div 
                                    className={`collapsed-tab ${activeTab === 'memories' ? 'active' : ''}`}
                                    title={`Memory List (${totalMemories})`}
                                >
                                    📋
                                    <span className="collapsed-count">{totalMemories}</span>
                                </div>
                            </div>
                        </div>
                    )}
                </div>
            </div>
        </>
    );
};

export default LeftPanel;
