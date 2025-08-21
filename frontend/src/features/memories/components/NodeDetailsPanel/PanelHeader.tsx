import React, { memo } from 'react';
import { PanelHeaderProps } from '../../types/nodeDetails';
import { ARIA_LABELS } from '../../../../common/constants/draggable';

const PanelHeader: React.FC<PanelHeaderProps> = ({
    title,
    onClose,
    onMouseDown,
    onDoubleClick,
    isDragging
}) => {
    return (
        <div 
            className="panel-header draggable-header"
            onMouseDown={onMouseDown}
            onDoubleClick={onDoubleClick}
            style={{ cursor: isDragging ? 'move' : 'move' }}
            aria-label={ARIA_LABELS.DRAG_HANDLE}
            role="button"
            tabIndex={0}
        >
            <h3>{title}</h3>
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
    );
};

export default memo(PanelHeader);