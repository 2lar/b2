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
    onClose,
    onOpenDocumentMode
}) => {
    const panelRef = useRef<HTMLDivElement>(null);
    
    // Check if desktop
    const isDesktop = useMemo(() => {
        if (typeof window === 'undefined') return false;
        return window.innerWidth > 1024;
    }, []);
    
    // Use a responsive default position that works for all screen sizes
    const defaultPosition = useMemo(() => {
        if (typeof window === 'undefined') {
            return { x: 400, y: 100 };
        }
        
        const isMobile = window.innerWidth <= 768;
        
        if (isMobile) {
            // On mobile, panel is positioned via CSS (fixed bottom), so position doesn't matter
            // Use centered coordinates to avoid any edge case issues
            return {
                x: window.innerWidth / 2 - 150, // Roughly center for 300px max-width panel
                y: window.innerHeight - 250     // Bottom positioning
            };
        } else {
            // Desktop positioning - get actual panel width from CSS breakpoints
            let panelWidth = 400; // default
            if (window.innerWidth <= 1600) panelWidth = 350;
            if (window.innerWidth <= 1400) panelWidth = 300;
            if (window.innerWidth <= 1300) panelWidth = 280;
            if (window.innerWidth <= 1200) panelWidth = 260;
            
            return {
                x: Math.min(window.innerWidth - panelWidth - 20, window.innerWidth * 0.6),
                y: Math.min(window.innerHeight - 300, window.innerHeight * 0.3)
            };
        }
    }, []);
    
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

    // Handle document mode toggle
    const handleDocumentMode = useCallback(() => {
        if (selectedNode && onOpenDocumentMode) {
            onOpenDocumentMode(selectedNode, connectedMemories);
        }
    }, [selectedNode, connectedMemories, onOpenDocumentMode]);

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
                    onDocumentMode={handleDocumentMode}
                    isDesktop={isDesktop}
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