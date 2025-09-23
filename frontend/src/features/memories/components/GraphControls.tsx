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
import styles from './GraphControls.module.css';

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
            <div className={styles.overlay}>
                <button
                    className={styles.overlayButton}
                    onClick={onRefresh}
                    title="Refresh graph"
                >
                    🔄
                </button>
                {onFitToScreen && (
                    <button
                        className={styles.overlayButton}
                        onClick={onFitToScreen}
                        title="Fit to screen"
                    >
                        ⌂
                    </button>
                )}
                {onResetZoom && (
                    <button
                        className={styles.overlayButton}
                        onClick={onResetZoom}
                        title="Reset zoom"
                    >
                        🔍
                    </button>
                )}
                <button
                    className={styles.overlayButton}
                    onClick={onToggleFullscreen}
                    title={isFullscreen ? 'Exit Fullscreen' : 'Fullscreen'}
                >
                    {isFullscreen ? '🗗' : '⛶'}
                </button>
            </div>
        );
    }

    return (
        <div className={styles.header} data-drag-handle>
            <span className={styles.headerTitle}>{title}</span>
            <div className={styles.headerControls}>
                <button
                    className={styles.secondaryButton}
                    onClick={onRefresh}
                    title="Refresh graph"
                >
                    Refresh
                </button>
                {onFitToScreen && (
                    <button
                        className={styles.secondaryButton}
                        onClick={onFitToScreen}
                        title="Fit to screen"
                    >
                        Fit
                    </button>
                )}
                {onResetZoom && (
                    <button
                        className={styles.secondaryButton}
                        onClick={onResetZoom}
                        title="Reset zoom"
                    >
                        Reset
                    </button>
                )}
                <button
                    className={styles.fullscreenButton}
                    onClick={onToggleFullscreen}
                    title={isFullscreen ? 'Exit Fullscreen' : 'Fullscreen'}
                >
                    {isFullscreen ? '🗗 Exit Fullscreen' : '⛶ Fullscreen'}
                </button>
                <span className={styles.dragHandle}>⋮⋮</span>
            </div>
        </div>
    );
};

export default memo(GraphControls);
