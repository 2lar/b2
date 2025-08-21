import React, { memo } from 'react';
import { PanelContentProps } from '../../types/nodeDetails';
import ConnectionsList from './ConnectionsList';
import { formatDate } from '../../../../utils/dateFormatters';

const PanelContent: React.FC<PanelContentProps> = ({
    selectedNode,
    connectedMemories,
    onConnectedMemoryClick
}) => {
    return (
        <>
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
                        <ConnectionsList 
                            connections={connectedMemories}
                            onConnectionClick={onConnectedMemoryClick}
                        />
                    </div>
                </div>
            </div>
            
            <div className="panel-footer">
                <div className="memory-metadata">
                    <p>Created: {formatDate(selectedNode.timestamp)}</p>
                </div>
            </div>
        </>
    );
};

export default memo(PanelContent, (prevProps, nextProps) => {
    return (
        prevProps.selectedNode.id === nextProps.selectedNode.id &&
        prevProps.selectedNode.content === nextProps.selectedNode.content &&
        prevProps.connectedMemories.length === nextProps.connectedMemories.length
    );
});