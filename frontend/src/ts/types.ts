import { Session, User } from '@supabase/supabase-js';
import cytoscape from 'cytoscape';

// Type for the data returned by our /api/nodes endpoint
export interface MemoryNode {
    nodeId: string;
    content: string;
    timestamp: string; // ISO 8601 date string
}

// Type for the data returned by our /api/graph-data endpoint
export interface GraphData {
    elements: cytoscape.ElementDefinition[];
}

// Type for the full node details from /api/nodes/{nodeId}
export interface NodeDetails extends MemoryNode {
    edges: string[]; // List of connected node IDs
}

// Extending the global Window interface to avoid TypeScript errors
// for properties we add to it.
declare global {
    interface Window {
        cy: cytoscape.Core;
        showApp: (email: string) => void;
    }
}