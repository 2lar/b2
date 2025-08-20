import React, { memo, useCallback } from 'react';
import { ConnectionsListProps } from '../../types/nodeDetails';
import { KEYBOARD_KEYS, ARIA_LABELS } from '../../../../common/constants/draggable';

const ConnectionsList: React.FC<ConnectionsListProps> = ({
    connections,
    onConnectionClick
}) => {
    const handleKeyDown = useCallback((e: React.KeyboardEvent, memoryId: string) => {
        if (e.key === KEYBOARD_KEYS.ENTER || e.key === KEYBOARD_KEYS.SPACE) {
            e.preventDefault();
            onConnectionClick(memoryId);
        }
    }, [onConnectionClick]);

    if (connections.length === 0) {
        return <p className="no-connections">No connections yet</p>;
    }

    return (
        <ul className="connected-memories-list">
            {connections.map(memory => (
                <li 
                    key={memory.id}
                    className="connected-memory-item"
                    onClick={() => onConnectionClick(memory.id)}
                    role="button"
                    tabIndex={0}
                    onKeyDown={(e) => handleKeyDown(e, memory.id)}
                    aria-label={`${ARIA_LABELS.CONNECTED_MEMORY}: ${memory.label}`}
                >
                    • {memory.label}
                </li>
            ))}
        </ul>
    );
};

export default memo(ConnectionsList, (prevProps, nextProps) => {
    return (
        prevProps.connections.length === nextProps.connections.length &&
        prevProps.connections.every((conn, idx) => 
            conn.id === nextProps.connections[idx]?.id &&
            conn.label === nextProps.connections[idx]?.label
        )
    );
});