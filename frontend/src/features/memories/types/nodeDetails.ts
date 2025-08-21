export interface DisplayNode {
    id: string;
    content: string;
    label: string;
    timestamp: string;
    tags?: string[];
}

export interface ConnectedMemory {
    id: string;
    label: string;
}

export interface NodeDetailsPanelProps {
    selectedNode: DisplayNode | null;
    connectedMemories: ConnectedMemory[];
    onConnectedMemoryClick: (memoryId: string) => void;
    onClose: () => void;
}

export interface PanelHeaderProps {
    title: string;
    onClose: () => void;
    onMouseDown: (event: React.MouseEvent) => void;
    onDoubleClick: () => void;
    isDragging: boolean;
}

export interface PanelContentProps {
    selectedNode: DisplayNode;
    connectedMemories: ConnectedMemory[];
    onConnectedMemoryClick: (memoryId: string) => void;
}

export interface ConnectionsListProps {
    connections: ConnectedMemory[];
    onConnectionClick: (memoryId: string) => void;
}