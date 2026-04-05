import Graph from 'graphology';
import { getCommunityColor } from './communityColors';
import type { NodeAttributes, EdgeAttributes } from './types';

const DEFAULT_EDGE_COLOR = 'rgba(203, 213, 225, 0.7)';

interface ApiNode {
    id: string;
    title?: string;
    content?: string;
    position?: { x?: number; y?: number };
    tags?: string[];
    metadata?: Record<string, string>;
    community_id?: string;
}

interface ApiEdge {
    id: string;
    source: string;
    target: string;
    weight?: number;
    type?: string;
}

interface LegacyElement {
    data: {
        id: string;
        label?: string;
        source?: string;
        target?: string;
        strength?: number;
    };
    position?: { x: number; y: number };
}

export interface GraphApiData {
    nodes?: ApiNode[];
    edges?: ApiEdge[];
    elements?: LegacyElement[];
}

export function buildGraph(apiData: GraphApiData): Graph<NodeAttributes, EdgeAttributes> {
    const graph = new Graph<NodeAttributes, EdgeAttributes>();

    if (apiData.nodes && Array.isArray(apiData.nodes)) {
        buildFromNewFormat(graph, apiData.nodes, apiData.edges || []);
    } else if (apiData.elements && Array.isArray(apiData.elements)) {
        buildFromLegacyFormat(graph, apiData.elements);
    }

    return graph;
}

function buildFromNewFormat(graph: Graph<NodeAttributes, EdgeAttributes>, nodes: ApiNode[], edges: ApiEdge[]) {
    for (const node of nodes) {
        const title = node.title || '';
        const content = node.content || '';
        const label = title || content.substring(0, 50) || 'Untitled';
        const communityId = node.community_id || '';
        const color = getCommunityColor(communityId);

        graph.addNode(node.id, {
            label,
            content,
            title,
            communityId,
            tags: node.tags || [],
            timestamp: node.metadata?.created_at || '',
            x: node.position?.x ?? (Math.random() - 0.5) * 1000,
            y: node.position?.y ?? (Math.random() - 0.5) * 1000,
            size: 8,
            color,
            originalColor: color,
            fixed: false,
        });
    }

    for (const edge of edges) {
        if (!graph.hasNode(edge.source) || !graph.hasNode(edge.target)) continue;
        if (edge.source === edge.target) continue;
        const key = `${edge.source}->${edge.target}`;
        if (graph.hasEdge(key)) continue;

        const weight = edge.weight ?? 1;
        graph.addEdgeWithKey(key, edge.source, edge.target, {
            weight,
            type: edge.type || 'normal',
            color: DEFAULT_EDGE_COLOR,
            size: Math.max(1.5, Math.min(weight * 7, 12)),
        });
    }
}

function buildFromLegacyFormat(graph: Graph<NodeAttributes, EdgeAttributes>, elements: LegacyElement[]) {
    const nodeElements = elements.filter(el => !el.data.source);
    const edgeElements = elements.filter(el => !!el.data.source);

    for (const el of nodeElements) {
        const label = el.data.label || el.data.id;
        graph.addNode(el.data.id, {
            label,
            content: label,
            title: label,
            communityId: '',
            tags: [],
            timestamp: '',
            x: el.position?.x ?? (Math.random() - 0.5) * 1000,
            y: el.position?.y ?? (Math.random() - 0.5) * 1000,
            size: 8,
            color: getCommunityColor(''),
            originalColor: getCommunityColor(''),
            fixed: false,
        });
    }

    for (const el of edgeElements) {
        const source = el.data.source!;
        const target = el.data.target!;
        if (!graph.hasNode(source) || !graph.hasNode(target)) continue;
        if (source === target) continue;
        const key = el.data.id || `${source}->${target}`;
        if (graph.hasEdge(key)) continue;

        const weight = el.data.strength ?? 1;
        graph.addEdgeWithKey(key, source, target, {
            weight,
            type: 'normal',
            color: DEFAULT_EDGE_COLOR,
            size: Math.max(1.5, Math.min(weight * 7, 12)),
        });
    }
}
