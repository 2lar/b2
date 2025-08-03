import React, { useState, useEffect, useRef } from 'react';
import { useNavigate } from 'react-router-dom';
import { User } from '@supabase/supabase-js';
import Header from './Header';
import GraphVisualization, { GraphVisualizationRef } from './GraphVisualization';
import MemoryInput from './MemoryInput';
import MemoryList from './MemoryList';
import { api, type Node } from '../services';

interface DashboardProps {
    user: User;
    onSignOut: () => void;
}

const Dashboard: React.FC<DashboardProps> = ({ user, onSignOut }) => {
    const navigate = useNavigate();
    const [memories, setMemories] = useState<Node[]>([]);
    const [isLoading, setIsLoading] = useState(false);
    const [currentPage, setCurrentPage] = useState(1);
    const [totalPages, setTotalPages] = useState(1);
    const [refreshGraph, setRefreshGraph] = useState(0);
    const graphRef = useRef<GraphVisualizationRef>(null);

    const MEMORIES_PER_PAGE = 50;

    useEffect(() => {
        loadMemories();
    }, []);

    const loadMemories = async () => {
        setIsLoading(true);
        try {
            const data = await api.listNodes();
            const allNodes = data.nodes || [];
            
            // Sort by timestamp (newest first)
            allNodes.sort((a, b) => 
                new Date(b.timestamp || '').getTime() - new Date(a.timestamp || '').getTime()
            );

            setMemories(allNodes);
            setTotalPages(Math.ceil(allNodes.length / MEMORIES_PER_PAGE));
        } catch (error) {
            console.error('Error loading memories:', error);
        } finally {
            setIsLoading(false);
        }
    };

    const handleMemoryCreated = () => {
        loadMemories();
        // Trigger graph refresh
        setRefreshGraph(prev => prev + 1);
    };

    const handleMemoryDeleted = () => {
        loadMemories();
        // Trigger graph refresh
        setRefreshGraph(prev => prev + 1);
    };

    const handleMemoryUpdated = () => {
        loadMemories();
        // Trigger graph refresh
        setRefreshGraph(prev => prev + 1);
    };

    const handleViewInGraph = (nodeId: string) => {
        if (graphRef.current) {
            const success = graphRef.current.selectAndCenterNode(nodeId);
            if (!success) {
                console.warn(`Could not find node ${nodeId} in graph. The graph may still be loading.`);
            }
        }
    };

    // Get current page memories
    const startIndex = (currentPage - 1) * MEMORIES_PER_PAGE;
    const endIndex = startIndex + MEMORIES_PER_PAGE;
    const currentPageMemories = memories.slice(startIndex, endIndex);

    return (
        <div className="app-container">
            <Header 
                userEmail={user.email || ''} 
                onSignOut={onSignOut} 
            />

            <main className="dashboard-layout">
                {/* Left Sidebar - Category Navigation */}
                <div className="left-sidebar">
                    <div className="sidebar-header">
                        <h3>Categories</h3>
                    </div>
                    <div className="sidebar-content">
                        <button 
                            className="sidebar-btn primary"
                            onClick={() => navigate('/categories')}
                        >
                            <span className="sidebar-icon">📁</span>
                            All Categories
                        </button>
                        <div className="sidebar-divider"></div>
                        <div className="category-list">
                            <div className="category-item" onClick={() => navigate('/categories')}>
                                <span className="category-icon">💼</span>
                                <span className="category-name">Work</span>
                                <span className="memory-count">5</span>
                            </div>
                            <div className="category-item" onClick={() => navigate('/categories')}>
                                <span className="category-icon">🎓</span>
                                <span className="category-name">Learning</span>
                                <span className="memory-count">12</span>
                            </div>
                            <div className="category-item" onClick={() => navigate('/categories')}>
                                <span className="category-icon">💡</span>
                                <span className="category-name">Ideas</span>
                                <span className="memory-count">8</span>
                            </div>
                        </div>
                    </div>
                </div>

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
                        memories={currentPageMemories}
                        totalMemories={memories.length}
                        currentPage={currentPage}
                        totalPages={totalPages}
                        isLoading={isLoading}
                        onPageChange={setCurrentPage}
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