import React, { useEffect, useRef, useState } from 'react';
import cytoscape, { Core, ElementDefinition, NodeSingular } from 'cytoscape';
import cola from 'cytoscape-cola';
import { api } from '../ts/apiClient';
import { components } from '../ts/generated-types';
import { useFullscreen } from '../hooks/useFullscreen';

// Register the cola layout
cytoscape.use(cola);

// Type aliases for easier usage
type NodeDetails = components['schemas']['NodeDetails'];

interface GraphVisualizationProps {
    refreshTrigger: number;
}

interface DisplayNode {
    id: string;
    content: string;
    label: string;
    timestamp: string;
}

// Color spectrum for node clustering
const NODE_COLORS = [
    '#2563eb', // blue (primary)
    '#10b981', // green
    '#f59e0b', // yellow
    '#ef4444', // red
    '#8b5cf6', // purple
    '#06b6d4', // cyan
    '#f97316', // orange
    '#ec4899', // pink
    '#84cc16', // lime
    '#6366f1', // indigo
];

const GraphVisualization: React.FC<GraphVisualizationProps> = ({ refreshTrigger }) => {
    const containerRef = useRef<HTMLDivElement>(null);
    const graphContainerRef = useRef<HTMLDivElement>(null);
    const cyRef = useRef<Core | null>(null);
    const [selectedNode, setSelectedNode] = useState<DisplayNode | null>(null);
    
    // Fullscreen functionality
    const { isFullscreen, toggleFullscreen } = useFullscreen(graphContainerRef);

    // Initialize cytoscape once on component mount
    useEffect(() => {
        if (!containerRef.current) return;

        const cy = cytoscape({
            container: containerRef.current,
            elements: [],
            zoom: 0.8,
            minZoom: 0.1,
            maxZoom: 3.0,
            wheelSensitivity: 2,
            style: [
                {
                    selector: 'node',
                    style: {
                        'background-color': 'data(color)',
                        'label': 'data(label)',
                        'color': '#1e293b',
                        'text-valign': 'center',
                        'text-halign': 'center',
                        'font-size': 12,
                        'width': 50,
                        'height': 50,
                        'text-wrap': 'ellipsis',
                        'text-max-width': '80px',
                        'border-width': 2,
                        'border-color': '#e2e8f0',
                        'transition-property': 'background-color, border-color, border-width, width, height, opacity',
                        'transition-duration': 200
                    }
                },
                {
                    selector: 'edge',
                    style: {
                        'width': 1.5,
                        'line-color': '#cbd5e1',
                        'target-arrow-color': '#cbd5e1',
                        'target-arrow-shape': 'triangle',
                        'curve-style': 'bezier',
                        'opacity': 0.7,
                        'transition-property': 'line-color, width, opacity',
                        'transition-duration': 200
                    }
                },
                {
                    selector: 'node:selected',
                    style: {
                        'background-color': '#1d4ed8',
                        'border-width': 3,
                        'border-color': '#2563eb',
                        'width': 60,
                        'height': 60
                    }
                },
                {
                    selector: 'node.highlighted',
                    style: {
                        'background-color': '#10b981',
                        'border-color': '#059669',
                        'border-width': 3
                    }
                },
                {
                    selector: 'edge.highlighted',
                    style: {
                        'line-color': '#10b981',
                        'target-arrow-color': '#10b981',
                        'width': 3,
                        'opacity': 1
                    }
                }
            ],
            layout: { name: 'preset' }
        });

        cyRef.current = cy;

        // Set up event handlers
        setupEventHandlers(cy);

        // Store global reference for legacy compatibility
        (window as any).cy = cy;

        return () => {
            if (cyRef.current) {
                cyRef.current.destroy();
                cyRef.current = null;
            }
        };
    }, []);

    const setupEventHandlers = (cy: Core) => {
        cy.on('tap', 'node', (evt) => {
            const node = evt.target;
            highlightConnectedNodes(node);
            showNodeDetails(node.id());
        });

        cy.on('tap', (evt) => {
            if (evt.target === cy) {
                hideNodeDetails();
                unhighlightAll();
            }
        });
    };

    function highlightConnectedNodes(node: cytoscape.NodeSingular): void {
        if (!cyRef.current) return;
        const cy = cyRef.current;
        cy.batch(() => {
            cy.elements().removeClass('highlighted');
            node.addClass('highlighted');
            node.neighborhood().addClass('highlighted');
        });
    }

    function unhighlightAll(): void {
        if (!cyRef.current) return;
        cyRef.current.elements().removeClass('highlighted');
    }

    async function showNodeDetails(nodeId: string): Promise<void> {
        try {
            const nodeData: NodeDetails = await api.getNode(nodeId);
            setSelectedNode({
                id: nodeId,
                content: nodeData.content || '',
                label: nodeData.content ? (nodeData.content.length > 50 ? nodeData.content.substring(0, 47) + '...' : nodeData.content) : '',
                timestamp: nodeData.timestamp || ''
            });
        } catch (error) {
            console.error('Error loading node details:', error);
        }
    }

    function hideNodeDetails(): void {
        setSelectedNode(null);
    }

    // Handle fullscreen changes - resize cytoscape when entering/exiting fullscreen
    useEffect(() => {
        if (cyRef.current) {
            // Small delay to allow fullscreen transition to complete
            const timer = setTimeout(() => {
                cyRef.current?.resize();
                cyRef.current?.fit();
                cyRef.current?.center();
            }, 100);

            return () => clearTimeout(timer);
        }
    }, [isFullscreen]);

    // Load and update graph data - similar to refreshGraph from graph-viz.ts
    useEffect(() => {
        loadGraphData();
    }, [refreshTrigger]);

    const loadGraphData = async () => {
        if (!cyRef.current) return;
        
        try {
            const graphData = await api.getGraphData();
            const elements = graphData.elements || [];

            if (elements.length === 0) return;

            const cy = cyRef.current;
            const processedElements = preprocessGraphData(elements);

            cy.batch(() => {
                cy.elements().remove();
                cy.add(processedElements as ElementDefinition[]);
            });

            // Apply cola layout with minimal stable parameters
            const layout = cy.layout({
                name: 'cola',
                animate: true,
                fit: true,
                padding: 50,
                nodeSpacing: () => 50,
                edgeLength: () => 100,
                avoidOverlap: true,
                handleDisconnected: true,
                convergenceThreshold: 0.01,
                maxSimulationTime: 2000,
                stop: () => {
                    cy.animate({
                        fit: { eles: cy.elements(), padding: 30 },
                        duration: 300
                    } as any);
                }
            } as any);

            layout.run();

        } catch (error) {
            console.error('Error loading graph data:', error);
        }
    };

    function preprocessGraphData(elements: any[]): any[] {
        const nodes = elements.filter(el => el.data && !el.data.source);
        const edges = elements.filter(el => el.data && el.data.source);
        
        // Create adjacency map for clustering
        const adjacency = new Map<string, Set<string>>();
        edges.forEach(edge => {
            if (!adjacency.has(edge.data.source)) {
                adjacency.set(edge.data.source, new Set());
            }
            if (!adjacency.has(edge.data.target)) {
                adjacency.set(edge.data.target, new Set());
            }
            adjacency.get(edge.data.source)!.add(edge.data.target);
            adjacency.get(edge.data.target)!.add(edge.data.source);
        });
        
        // Sort nodes by connectivity for better clustering
        const nodesByConnectivity = nodes.sort((a, b) => {
            const aConn = adjacency.get(a.data.id)?.size || 0;
            const bConn = adjacency.get(b.data.id)?.size || 0;
            return bConn - aConn;
        });
        
        // Assign colors based on connectivity clusters
        const processedNodes = nodesByConnectivity.map((node) => {
            const connectivity = adjacency.get(node.data.id)?.size || 0;
            // Use different colors based on connectivity level
            let colorIndex = 0;
            if (connectivity > 3) colorIndex = 1; // Green for highly connected
            else if (connectivity > 1) colorIndex = 2; // Yellow for moderately connected
            else colorIndex = 0; // Blue for isolated/low connected
            
            const color = NODE_COLORS[colorIndex];
            const label = node.data.label ? (node.data.label.length > 50 ? node.data.label.substring(0, 47) + '...' : node.data.label) : '';
            
            return {
                ...node,
                data: {
                    ...node.data,
                    label,
                    color
                }
            };
        });
        
        return [...processedNodes, ...edges];
    }

    const formatDate = (dateString: string): string => {
        if (!dateString) return '';
        
        try {
            const date = new Date(dateString);
            return date.toLocaleDateString(undefined, {
                year: 'numeric',
                month: 'short',
                day: 'numeric',
                hour: '2-digit',
                minute: '2-digit'
            });
        } catch (e) {
            return dateString;
        }
    };

    return (
        <div className="dashboard-container" id="graph-container" data-container="graph" ref={graphContainerRef}>
            <div className="container-header" data-drag-handle>
                <span className="container-title">Memory Graph</span>
                <div className="container-controls">
                    <button 
                        className="secondary-btn"
                        onClick={loadGraphData}
                    >
                        Refresh
                    </button>
                    <button 
                        className="fullscreen-btn"
                        onClick={toggleFullscreen}
                    >
                        {isFullscreen ? '🗗 Exit Fullscreen' : '⛶ Fullscreen'}
                    </button>
                    <span className="drag-handle">⋮⋮</span>
                </div>
            </div>
            <div className="container-content graph-content">
                <div ref={containerRef} className="graph-container"></div>
                
                {/* Node Details Panel */}
                {selectedNode && (
                    <div className="node-details floating-panel">
                        <div className="panel-header">
                            <h3>Memory Details</h3>
                            <button 
                                className="close-btn"
                                onClick={() => setSelectedNode(null)}
                            >
                                ×
                            </button>
                        </div>
                        <div className="panel-content">
                            <div className="node-content-section">
                                {selectedNode.content}
                            </div>
                            <div className="connections-section">
                                <h4>Connected Memories</h4>
                                <div className="scrollable-connections">
                                    <p>Created: {formatDate(selectedNode.timestamp)}</p>
                                </div>
                            </div>
                        </div>
                    </div>
                )}
            </div>
        </div>
    );
};

export default GraphVisualization;