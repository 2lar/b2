export interface DisplayNode {
    id: string;
    content: string;
    title?: string;
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
    onOpenDocumentMode?: (node: DisplayNode, connections: ConnectedMemory[]) => void;
}

export interface PanelHeaderProps {
    title: string;
    onClose: () => void;
    onMouseDown: (event: React.MouseEvent) => void;
    onTouchStart?: (event: React.TouchEvent) => void;
    onDoubleClick: () => void;
    isDragging: boolean;
    onDocumentMode?: () => void;
    isDesktop?: boolean;
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
