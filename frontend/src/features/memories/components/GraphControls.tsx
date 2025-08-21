/**
 * GraphControls Component - Interactive Controls for Graph Visualization
 * 
 * Purpose:
 * Provides user controls for interacting with the graph visualization.
 * Extracted from GraphVisualization to improve maintainability and reusability.
 * 
 * Key Features:
 * - Refresh graph data manually
 * - Toggle fullscreen mode
 * - Zoom controls
 * - Fit to screen functionality
 * - Reset viewport controls
 * - Export/screenshot functionality (future)
 * 
 * Integration:
 * - Receives callbacks from parent GraphVisualization
 * - Controls both overlay and traditional header layouts
 * - Responsive design for different screen sizes
 */

import React, { memo } from 'react';

interface GraphControlsProps {
    /** Whether to render as overlay controls */
    isOverlay?: boolean;
    /** Whether currently in fullscreen mode */
    isFullscreen: boolean;
    /** Callback to refresh graph data */
    onRefresh: () => void;
    /** Callback to toggle fullscreen mode */
    onToggleFullscreen: () => void;
    /** Callback to fit graph to screen */
    onFitToScreen?: () => void;
    /** Callback to reset zoom */
    onResetZoom?: () => void;
    /** Optional title for the controls section */
    title?: string;
}

const GraphControls: React.FC<GraphControlsProps> = ({
    isOverlay = false,
    isFullscreen,
    onRefresh,
    onToggleFullscreen,
    onFitToScreen,
    onResetZoom,
    title = "Memory Graph"
}) => {
    if (isOverlay) {
        return (
            <div className="graph-controls-overlay">
                <button 
                    className="graph-control-btn"
                    onClick={onRefresh}
                    title="Refresh Graph"
                >
                    🔄
                </button>
                {onFitToScreen && (
                    <button 
                        className="graph-control-btn"
                        onClick={onFitToScreen}
                        title="Fit to Screen"
                    >
                        ⌂
                    </button>
                )}
                {onResetZoom && (
                    <button 
                        className="graph-control-btn"
                        onClick={onResetZoom}
                        title="Reset Zoom"
                    >
                        🔍
                    </button>
                )}
                <button 
                    className="graph-control-btn"
                    onClick={onToggleFullscreen}
                    title={isFullscreen ? 'Exit Fullscreen' : 'Fullscreen'}
                >
                    {isFullscreen ? '🗗' : '⛶'}
                </button>
            </div>
        );
    }

    return (
        <div className="container-header" data-drag-handle>
            <span className="container-title">{title}</span>
            <div className="container-controls">
                <button 
                    className="secondary-btn"
                    onClick={onRefresh}
                    title="Refresh Graph"
                >
                    Refresh
                </button>
                {onFitToScreen && (
                    <button 
                        className="secondary-btn"
                        onClick={onFitToScreen}
                        title="Fit to Screen"
                    >
                        Fit
                    </button>
                )}
                {onResetZoom && (
                    <button 
                        className="secondary-btn"
                        onClick={onResetZoom}
                        title="Reset Zoom"
                    >
                        Reset
                    </button>
                )}
                <button 
                    className="fullscreen-btn"
                    onClick={onToggleFullscreen}
                    title={isFullscreen ? 'Exit Fullscreen' : 'Fullscreen'}
                >
                    {isFullscreen ? '🗗 Exit Fullscreen' : '⛶ Fullscreen'}
                </button>
                <span className="drag-handle">⋮⋮</span>
            </div>
        </div>
    );
};

export default memo(GraphControls);