/**
 * Dashboard Component - Main Application Interface
 * 
 * Purpose:
 * The primary dashboard with centered graph visualization and integrated memory input.
 * Acts as the central hub where users interact with their memory data through a streamlined interface.
 * 
 * Key Features:
 * - Centered graph visualization with maximum screen space
 * - Integrated memory input at top center of graph area
 * - Collapsible file system sidebar on the left
 * - Collapsible memory list panel on the right (dropdown style)
 * - Real-time synchronization between all components
 * - Automatic refresh coordination across panels
 * 
 * Layout Structure:
 * - Left: Collapsible FileSystemSidebar (categories and memories)
 * - Center: Main area with GraphVisualization + integrated MemoryInput
 * - Right: Collapsible MemoryList panel (slide-in dropdown)
 * 
 * State Management:
 * - Manages memory and category data loading
 * - Coordinates refresh triggers across all child components
 * - Handles pagination state for memory list
 * - Manages panel visibility states (sidebar, memory list)
 * - Manages navigation between different views
 * 
 * Integration:
 * - Receives user session from App component
 * - Passes memory selection events to graph visualization
 * - Coordinates data refresh after CRUD operations
 * - Handles routing to category detail views
 */

import React, { useState, useEffect, useRef } from 'react';
import { useNavigate } from 'react-router-dom';
import { User } from '@supabase/supabase-js';
import { Header, LeftPanel } from '../../../common';
import { GraphVisualization, type GraphVisualizationRef, MemoryInput, nodesApi } from '../../memories';
import { categoriesApi } from '../../categories';
import type { Node } from '../../../services';
import { components } from '../../../types/generated/generated-types';

interface DashboardProps {
    /** Authenticated user object from Supabase */
    user: User;
    /** Callback function to handle user sign-out */
    onSignOut: () => void;
}

type Category = components['schemas']['Category'];

const Dashboard: React.FC<DashboardProps> = ({ user, onSignOut }) => {
    const navigate = useNavigate();
    const [memories, setMemories] = useState<Node[]>([]);
    const [categories, setCategories] = useState<Category[]>([]);
    const [isLoading, setIsLoading] = useState(false);
    const [currentPage, setCurrentPage] = useState(1);
    const [totalMemories, setTotalMemories] = useState(0);
    const [totalPages, setTotalPages] = useState(1);
    const [nextToken, setNextToken] = useState<string | undefined>(undefined);
    const [refreshGraph, setRefreshGraph] = useState(0);
    const [refreshSidebar, setRefreshSidebar] = useState(0);
    const [isLeftPanelCollapsed, setIsLeftPanelCollapsed] = useState(false);
    const graphRef = useRef<GraphVisualizationRef>(null);

    const MEMORIES_PER_PAGE = 50;

    useEffect(() => {
        loadMemories(1);
        loadCategories();
    }, []);

    const loadMemories = async (page: number, token?: string) => {
        setIsLoading(true);
        try {
            const data = await nodesApi.listNodes(MEMORIES_PER_PAGE, token);
            const pageNodes = data.nodes || [];
            
            // Sort by timestamp (newest first) - backend should handle this eventually
            pageNodes.sort((a, b) => 
                new Date(b.timestamp || '').getTime() - new Date(a.timestamp || '').getTime()
            );

            setMemories(pageNodes);
            setTotalMemories(data.total || 0);
            setTotalPages(Math.ceil((data.total || 0) / MEMORIES_PER_PAGE));
            setNextToken(data.nextToken);
            setCurrentPage(page);
        } catch (error) {
            console.error('Error loading memories:', error);
            
            const errorMessage = (error as Error).message;
            if (errorMessage.includes('Authentication') || errorMessage.includes('expired') || errorMessage.includes('sign in')) {
                // Show user-friendly authentication error
                alert('Your session has expired. Please refresh the page or sign in again.');
            }
        } finally {
            setIsLoading(false);
        }
    };

    const loadCategories = async () => {
        try {
            const data = await categoriesApi.listCategories();
            setCategories(data.categories || []);
        } catch (error) {
            console.error('Error loading categories:', error);
        }
    };

    const handleMemoryCreated = () => {
        loadMemories(1); // Go to first page to see new memory
        loadCategories(); // Refresh categories as new memories might be auto-categorized
        // Trigger graph and sidebar refresh
        setRefreshGraph(prev => prev + 1);
        setRefreshSidebar(prev => prev + 1);
    };

    const handleMemoryDeleted = () => {
        loadMemories(currentPage); // Stay on current page, but reload to update counts
        loadCategories(); // Refresh categories as counts might change
        // Trigger graph and sidebar refresh
        setRefreshGraph(prev => prev + 1);
        setRefreshSidebar(prev => prev + 1);
    };

    const handleMemoryUpdated = () => {
        loadMemories(currentPage); // Stay on current page and reload
        loadCategories(); // Refresh categories as content might affect categorization
        // Trigger graph and sidebar refresh
        setRefreshGraph(prev => prev + 1);
        setRefreshSidebar(prev => prev + 1);
    };

    const handlePageChange = (newPage: number) => {
        // For now, we'll implement simple pagination by reloading
        // In the future, we could cache pages for better UX
        loadMemories(newPage);
    };

    const handleViewInGraph = (nodeId: string) => {
        if (graphRef.current) {
            const success = graphRef.current.selectAndCenterNode(nodeId);
            if (!success) {
                console.warn('Could not find node in graph. The graph may still be loading.');
            }
        }
        // Memory list is now in left panel, no need to close
    };

    const toggleLeftPanel = () => {
        setIsLeftPanelCollapsed(!isLeftPanelCollapsed);
    };

    // memories already contains the current page data from server

    return (
        <div className="app-container">
            <Header 
                userEmail={user.email || ''} 
                onSignOut={onSignOut}
                onToggleSidebar={toggleLeftPanel}
                isSidebarCollapsed={isLeftPanelCollapsed}
                memoryCount={totalMemories}
            />

            <main className="dashboard-layout-refined">
                {/* Left Panel with Tabs */}
                <LeftPanel
                    user={user}
                    isCollapsed={isLeftPanelCollapsed}
                    onToggleCollapse={toggleLeftPanel}
                    onMemorySelect={handleViewInGraph}
                    onCategorySelect={(categoryId) => navigate(`/categories/${categoryId}`)}
                    refreshTrigger={refreshSidebar}
                    memories={memories}
                    totalMemories={totalMemories}
                    currentPage={currentPage}
                    totalPages={totalPages}
                    isLoading={isLoading}
                    onPageChange={handlePageChange}
                    onMemoryDeleted={handleMemoryDeleted}
                    onMemoryUpdated={handleMemoryUpdated}
                />

                {/* Main Center Area - Graph with Integrated Memory Input */}
                <div className="main-content-area">
                    {/* Integrated Memory Input at Top */}
                    <div className="memory-input-overlay">
                        <MemoryInput 
                            onMemoryCreated={handleMemoryCreated}
                            isCompact={true}
                        />
                    </div>

                    {/* Graph Visualization */}
                    <GraphVisualization 
                        ref={graphRef} 
                        refreshTrigger={refreshGraph}
                        hasOverlayInput={true}
                    />
                </div>
            </main>
        </div>
    );
};

export default Dashboard;