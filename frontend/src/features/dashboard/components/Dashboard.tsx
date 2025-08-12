/**
 * Dashboard Component - Main Application Interface
 * 
 * Purpose:
 * The primary dashboard that orchestrates all main UI components in a multi-panel layout.
 * Acts as the central hub where users interact with their memory data through different views.
 * 
 * Key Features:
 * - Multi-panel layout with resizable columns
 * - File system sidebar for browsing memories by category
 * - Interactive graph visualization of memory connections
 * - Memory input form for creating new memories
 * - Paginated memory list for browsing and management
 * - Real-time synchronization between all components
 * - Automatic refresh coordination across panels
 * 
 * Layout Structure:
 * - Left: FileSystemSidebar (categories and memories in folder structure)
 * - Center: GraphVisualization (interactive node graph)
 * - Right Top: MemoryInput (creation form)
 * - Right Bottom: MemoryList (paginated list view)
 * 
 * State Management:
 * - Manages memory and category data loading
 * - Coordinates refresh triggers across all child components
 * - Handles pagination state for memory list
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
import { Header } from '../../../common';
import { GraphVisualization, type GraphVisualizationRef, MemoryInput, MemoryList, FileSystemSidebar, nodesApi } from '../../memories';
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
    };

    // memories already contains the current page data from server

    return (
        <div className="app-container">
            <Header 
                userEmail={user.email || ''} 
                onSignOut={onSignOut} 
            />

            <main className="dashboard-layout">
                {/* Left Sidebar - File System Explorer */}
                <FileSystemSidebar
                    userId={user.id}
                    onMemorySelect={handleViewInGraph}
                    onCategorySelect={(categoryId) => navigate(`/categories/${categoryId}`)}
                    refreshTrigger={refreshSidebar}
                />

                {/* Column Resize Handle */}
                <div className="resize-handle horizontal" data-resize="horizontal-left"></div>

                {/* Middle Column - Memory Graph */}
                <GraphVisualization ref={graphRef} refreshTrigger={refreshGraph} />

                {/* Column Resize Handle */}
                <div className="resize-handle horizontal" data-resize="horizontal-right"></div>

                {/* Right Column Container */}
                <div className="right-column">
                    {/* Top Right - Memory Input */}
                    <MemoryInput onMemoryCreated={handleMemoryCreated} />

                    {/* Vertical Resize Handle */}
                    <div className="resize-handle vertical" data-resize="vertical"></div>

                    {/* Bottom Right - Memory List */}
                    <MemoryList 
                        memories={memories}
                        totalMemories={totalMemories}
                        currentPage={currentPage}
                        totalPages={totalPages}
                        isLoading={isLoading}
                        onPageChange={handlePageChange}
                        onMemoryDeleted={handleMemoryDeleted}
                        onMemoryUpdated={handleMemoryUpdated}
                        onMemoryViewInGraph={handleViewInGraph}
                    />
                </div>
            </main>
        </div>
    );
};

export default Dashboard;