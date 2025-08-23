/**
 * DocumentModeView Component - Split View for Document Editing with Connected Memories
 * 
 * Purpose:
 * Provides a split-pane view for editing a memory node in document mode
 * while displaying connected memories on the left side.
 * 
 * Key Features:
 * - Left sidebar shows connected memories list
 * - Right side shows DocumentEditor for the selected node
 * - Desktop-only feature for larger screens
 * - Seamless integration with node updates
 */

import React, { useState, useCallback } from 'react';
import { DocumentEditor } from '../../../../components/DocumentEditor';
import ConnectionsList from './ConnectionsList';
import { DisplayNode, ConnectedMemory } from '../../types/nodeDetails';
import { nodesApi } from '../../api/nodes';

interface DocumentModeViewProps {
    selectedNode: DisplayNode;
    connectedMemories: ConnectedMemory[];
    onConnectedMemoryClick: (memoryId: string) => void;
    onClose: () => void;
}

const DocumentModeView: React.FC<DocumentModeViewProps> = ({
    selectedNode,
    connectedMemories,
    onConnectedMemoryClick,
    onClose
}) => {
    const [content, setContent] = useState(selectedNode.content);
    const [title, setTitle] = useState(selectedNode.title || '');

    const handleSave = useCallback(async (newContent: string, newTitle?: string) => {
        try {
            // Update the node with new content and title
            await nodesApi.updateNode(selectedNode.id, newContent, undefined, newTitle);
            console.log('Node updated successfully');
        } catch (error) {
            console.error('Failed to update node:', error);
            throw error;
        }
    }, [selectedNode.id]);

    const handleClose = useCallback((finalContent: string, finalTitle: string) => {
        // Save the final state if needed
        setContent(finalContent);
        setTitle(finalTitle);
        onClose();
    }, [onClose]);

    return (
        <>
            {/* Backdrop */}
            <div 
                className="document-mode-backdrop"
                onClick={onClose}
            />
            
            {/* Split View Container */}
            <div className="document-mode-container">
                {/* Left Sidebar - Connected Memories */}
                <div className="document-mode-sidebar">
                    <div className="sidebar-header">
                        <h3>Connected Memories ({connectedMemories.length})</h3>
                    </div>
                    <div className="sidebar-content">
                        <ConnectionsList 
                            connections={connectedMemories}
                            onConnectionClick={onConnectedMemoryClick}
                        />
                    </div>
                </div>
                
                {/* Right Side - Document Editor */}
                <div className="document-mode-editor">
                    <DocumentEditor
                        initialContent={content}
                        initialTitle={title}
                        nodeId={selectedNode.id}
                        onClose={handleClose}
                        onSave={handleSave}
                        mode="embedded"
                    />
                </div>
            </div>
        </>
    );
};

export default DocumentModeView;