import React, { memo } from 'react';
import { PanelHeaderProps } from '../../types/nodeDetails';
import { ARIA_LABELS } from '../../../../common/constants/draggable';

const PanelHeader: React.FC<PanelHeaderProps> = ({
    title,
    onClose,
    onMouseDown,
    onTouchStart,
    onDoubleClick,
    isDragging,
    onDocumentMode,
    isDesktop
}) => {
    return (
        <div 
            className="panel-header draggable-header"
            onMouseDown={onMouseDown}
            onTouchStart={onTouchStart}
            onDoubleClick={onDoubleClick}
            style={{ cursor: isDragging ? 'move' : 'move' }}
            aria-label={ARIA_LABELS.DRAG_HANDLE}
            role="button"
            tabIndex={0}
        >
            <h3>{title}</h3>
            <div className="panel-header-buttons">
                {isDesktop && onDocumentMode && (
                    <button 
                        className="document-mode-btn"
                        onClick={onDocumentMode}
                        onMouseDown={(e) => e.stopPropagation()}
                        aria-label="Open in Document Mode"
                        type="button"
                        title="Open in Document Mode"
                    >
                        📄
                    </button>
                )}
                <button 
                    className="close-btn"
                    onClick={onClose}
                    onMouseDown={(e) => e.stopPropagation()}
                    aria-label={ARIA_LABELS.CLOSE_PANEL}
                    type="button"
                >
                    ×
                </button>
            </div>
        </div>
    );
};

export default memo(PanelHeader);
