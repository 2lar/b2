/**
 * NodeDetailsPanel Component - Detailed View of Selected Memory Node
 * 
 * Purpose:
 * Displays detailed information about a selected memory node from the graph.
 * Extracted from GraphVisualization to improve component maintainability.
 * 
 * Key Features:
 * - Shows full memory content and metadata
 * - Lists connected memories with navigation
 * - Displays creation timestamp and tags
 * - Floating panel design with close controls
 * - Click navigation between connected memories
 * 
 * Integration:
 * - Receives selected node data from parent
 * - Calls back to parent for navigation between nodes
 * - Positioned as floating overlay on graph
 */

import React, { memo } from 'react';

interface DisplayNode {
    id: string;
    content: string;
    label: string;
    timestamp: string;
    tags?: string[];
}

interface ConnectedMemory {
    id: string;
    label: string;
}

interface NodeDetailsPanelProps {
    /** The selected node to display details for */
    selectedNode: DisplayNode | null;
    /** List of memories connected to the selected node */
    connectedMemories: ConnectedMemory[];
    /** Callback when user clicks on a connected memory */
    onConnectedMemoryClick: (memoryId: string) => void;
    /** Callback when user closes the panel */
    onClose: () => void;
}

const NodeDetailsPanel: React.FC<NodeDetailsPanelProps> = ({
    selectedNode,
    connectedMemories,
    onConnectedMemoryClick,
    onClose
}) => {
    if (!selectedNode) {
        return null;
    }

    const formatDate = (dateString: string): string => {
        if (!dateString) return '';

        try {
            const date = new Date(dateString);
            return date.toLocaleDateString(undefined, {
                year: 'numeric',
                month: 'short',
                day: 'numeric',
                hour: '2-digit',
                minute: '2-digit'
            });
        } catch (e) {
            return dateString;
        }
    };

    return (
        <div className="node-details floating-panel">
            <div className="panel-header">
                <h3>Memory Details</h3>
                <button 
                    className="close-btn"
                    onClick={onClose}
                    aria-label="Close panel"
                >
                    ×
                </button>
            </div>
            
            <div className="panel-content">
                <div className="node-content-section">
                    {selectedNode.content}
                </div>
                
                {selectedNode.tags && selectedNode.tags.length > 0 && (
                    <div className="memory-tags">
                        {selectedNode.tags.map((tag, index) => (
                            <span key={index} className="memory-tag">
                                {tag}
                            </span>
                        ))}
                    </div>
                )}
                
                <div className="connections-section">
                    <h4>Connected Memories ({connectedMemories.length})</h4>
                    <div className="scrollable-connections">
                        {connectedMemories.length > 0 ? (
                            <ul className="connected-memories-list">
                                {connectedMemories.map(memory => (
                                    <li 
                                        key={memory.id}
                                        className="connected-memory-item"
                                        onClick={() => onConnectedMemoryClick(memory.id)}
                                        role="button"
                                        tabIndex={0}
                                        onKeyDown={(e) => {
                                            if (e.key === 'Enter' || e.key === ' ') {
                                                e.preventDefault();
                                                onConnectedMemoryClick(memory.id);
                                            }
                                        }}
                                    >
                                        • {memory.label}
                                    </li>
                                ))}
                            </ul>
                        ) : (
                            <p className="no-connections">No connections yet</p>
                        )}
                    </div>
                </div>
            </div>
            
            <div className="panel-footer">
                <div className="memory-metadata">
                    <p>Created: {formatDate(selectedNode.timestamp)}</p>
                </div>
            </div>
        </div>
    );
};

export default memo(NodeDetailsPanel);