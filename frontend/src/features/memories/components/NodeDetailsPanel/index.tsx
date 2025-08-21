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
 * - Keyboard navigation support
 * - Touch-enabled dragging
 * - Accessibility features
 * 
 * Integration:
 * - Receives selected node data from parent
 * - Calls back to parent for navigation between nodes
 * - Positioned as floating overlay on graph
 */

import React, { memo, useRef, useMemo, useCallback, useEffect } from 'react';
import { useDraggable } from '../../../../common/hooks';
import { NodeDetailsPanelProps } from '../../types/nodeDetails';
import { KEYBOARD_KEYS, ARIA_LABELS } from '../../../../common/constants/draggable';
import PanelHeader from './PanelHeader';
import PanelContent from './PanelContent';
import ErrorBoundary from '../../../../common/components/ErrorBoundary';

const NodeDetailsPanel: React.FC<NodeDetailsPanelProps> = ({
    selectedNode,
    connectedMemories,
    onConnectedMemoryClick,
    onClose
}) => {
    const panelRef = useRef<HTMLDivElement>(null);
    
    // Use a conservative default position that works for all screen sizes
    const defaultPosition = useMemo(() => ({
        x: typeof window !== 'undefined' ? Math.min(window.innerWidth - 450, window.innerWidth * 0.6) : 400,
        y: typeof window !== 'undefined' ? Math.min(window.innerHeight - 300, window.innerHeight * 0.3) : 100
    }), []);
    
    // Setup draggable functionality
    const { 
        position, 
        isDragging, 
        onMouseDown, 
        onTouchStart,
        onKeyDown: handleDragKeyDown,
        resetPosition 
    } = useDraggable(panelRef, {
        storageKey: 'node-details-panel-position',
        defaultPosition,
        boundaryPadding: 20,
        enableKeyboard: true,
        enableTouch: true
    });

    // Handle double-click to reset position
    const handleHeaderDoubleClick = useCallback(() => {
        resetPosition();
    }, [resetPosition]);

    // Handle global keyboard shortcuts
    const handlePanelKeyDown = useCallback((e: React.KeyboardEvent) => {
        if (e.key === KEYBOARD_KEYS.ESCAPE) {
            e.preventDefault();
            onClose();
        } else {
            handleDragKeyDown(e);
        }
    }, [onClose, handleDragKeyDown]);

    // Focus management when panel opens
    useEffect(() => {
        if (selectedNode && panelRef.current) {
            panelRef.current.focus();
        }
    }, [selectedNode]);

    if (!selectedNode) {
        return null;
    }

    return (
        <ErrorBoundary>
            <div 
                ref={panelRef}
                className={`node-details floating-panel ${isDragging ? 'dragging' : ''}`}
                style={{
                    transform: `translate3d(${position.x}px, ${position.y}px, 0)`,
                    position: 'fixed',
                    left: 0,
                    top: 0,
                    contain: 'layout style',
                    willChange: isDragging ? 'transform' : 'auto'
                }}
                role="dialog"
                aria-label={ARIA_LABELS.PANEL_REGION}
                aria-modal="false"
                tabIndex={-1}
                onKeyDown={handlePanelKeyDown}
            >
                <PanelHeader
                    title="Memory Details"
                    onClose={onClose}
                    onMouseDown={onMouseDown}
                    onDoubleClick={handleHeaderDoubleClick}
                    isDragging={isDragging}
                />
                
                <PanelContent
                    selectedNode={selectedNode}
                    connectedMemories={connectedMemories}
                    onConnectedMemoryClick={onConnectedMemoryClick}
                />
            </div>
        </ErrorBoundary>
    );
};

export default memo(NodeDetailsPanel, (prevProps, nextProps) => {
    return (
        prevProps.selectedNode?.id === nextProps.selectedNode?.id &&
        prevProps.connectedMemories.length === nextProps.connectedMemories.length &&
        prevProps.connectedMemories === nextProps.connectedMemories
    );
});