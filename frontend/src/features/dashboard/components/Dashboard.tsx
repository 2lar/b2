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
    const [pageTokens, setPageTokens] = useState<Map<number, string>>(new Map());
    const [refreshGraph, setRefreshGraph] = useState(0);
    const [refreshSidebar, setRefreshSidebar] = useState(0);
    const [isLeftPanelCollapsed, setIsLeftPanelCollapsed] = useState(() => {
        // Default to collapsed on mobile devices
        return window.innerWidth <= 768;
    });
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
            
            // Store the nextToken for the next page if it exists
            if (data.nextToken) {
                setPageTokens(prev => {
                    const newTokens = new Map(prev);
                    newTokens.set(page + 1, data.nextToken!);
                    return newTokens;
                });
            }
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
        setPageTokens(new Map()); // Clear pagination tokens when refreshing
        loadMemories(1); // Go to first page to see new memory
        loadCategories(); // Refresh categories as new memories might be auto-categorized
        // Trigger graph and sidebar refresh
        setRefreshGraph(prev => prev + 1);
        setRefreshSidebar(prev => prev + 1);
    };

    const handleMemoryDeleted = () => {
        // Don't clear tokens - we want to stay on the same page
        if (currentPage === 1) {
            loadMemories(1); // First page, no token needed
        } else {
            // Use loadFromPageOne to properly reload to current page
            loadFromPageOne(currentPage);
        }
        loadCategories(); // Refresh categories as counts might change
        // Trigger graph and sidebar refresh
        setRefreshGraph(prev => prev + 1);
        setRefreshSidebar(prev => prev + 1);
    };

    const handleMemoryUpdated = () => {
        // Don't clear tokens - we want to stay on the same page
        if (currentPage === 1) {
            loadMemories(1); // First page, no token needed
        } else {
            // Use loadFromPageOne to properly reload to current page
            loadFromPageOne(currentPage);
        }
        loadCategories(); // Refresh categories as content might affect categorization
        // Trigger graph and sidebar refresh
        setRefreshGraph(prev => prev + 1);
        setRefreshSidebar(prev => prev + 1);
    };

    const loadFromPageOne = async (targetPage: number) => {
        // Helper function to load a specific page by starting from page 1
        // This is needed for backward navigation in token-based pagination
        setIsLoading(true);
        try {
            let currentToken: string | undefined = undefined;
            const tokens = new Map<number, string>();
            
            // Load pages sequentially until we reach the target page
            for (let page = 1; page <= targetPage; page++) {
                const data = await nodesApi.listNodes(MEMORIES_PER_PAGE, currentToken);
                
                if (page === targetPage) {
                    // This is our target page - display it
                    const pageNodes = data.nodes || [];
                    pageNodes.sort((a, b) => 
                        new Date(b.timestamp || '').getTime() - new Date(a.timestamp || '').getTime()
                    );
                    
                    setMemories(pageNodes);
                    setTotalMemories(data.total || 0);
                    setTotalPages(Math.ceil((data.total || 0) / MEMORIES_PER_PAGE));
                    setNextToken(data.nextToken);
                    setCurrentPage(page);
                }
                
                // Store token for next page
                if (data.nextToken) {
                    tokens.set(page + 1, data.nextToken);
                }
                currentToken = data.nextToken;
            }
            
            // Update our token cache with the tokens we collected
            setPageTokens(tokens);
        } catch (error) {
            console.error('Error loading memories:', error);
            
            const errorMessage = (error as Error).message;
            if (errorMessage.includes('Authentication') || errorMessage.includes('expired') || errorMessage.includes('sign in')) {
                alert('Your session has expired. Please refresh the page or sign in again.');
            }
        } finally {
            setIsLoading(false);
        }
    };

    const handlePageChange = (newPage: number) => {
        
        if (newPage === 1) {
            // First page always loads without token
            loadMemories(1);
        } else if (newPage > currentPage) {
            // Going forward - use the stored token for this page
            const tokenForPage = pageTokens.get(newPage);
            loadMemories(newPage, tokenForPage);
        } else {
            // Going backward - need to reload from page 1 and build up to target page
            // This is a limitation of token-based pagination
            loadFromPageOne(newPage);
        }
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

    const handleDocumentModeOpen = () => {
        // Close any open node details panels when document mode opens
        graphRef.current?.hideNodeDetails();
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

            <main className="dashboard-layout-mobile-ready">
                {/* Left Panel with Tabs - Mobile Overlay */}
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
                    useVirtualScrolling={totalMemories > 100}
                />

                {/* Main Content Area */}
                <div className="main-content-area">
                    {/* Graph Visualization */}
                    <div className="graph-container">
                        <GraphVisualization 
                            ref={graphRef} 
                            refreshTrigger={refreshGraph}
                            hasOverlayInput={true}
                        />
                    </div>

                    {/* Memory Input - Now at bottom for all screen sizes */}
                    <div className="memory-input-container">
                        <div className="memory-input-overlay desktop-input">
                            <MemoryInput 
                                onMemoryCreated={handleMemoryCreated}
                                isCompact={true}
                                onDocumentModeOpen={handleDocumentModeOpen}
                            />
                        </div>
                    </div>

                    {/* Mobile Memory Input at Bottom (legacy support) */}
                    <div className="mobile-memory-input">
                        <MemoryInput 
                            onMemoryCreated={handleMemoryCreated}
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