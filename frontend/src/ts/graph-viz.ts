import cytoscape, { Core, LayoutOptions } from 'cytoscape';
import { api } from './api';
import { NodeDetails } from './types';

let cy: Core | null = null;
let currentLayout: cytoscape.Layouts | null = null;

// DOM Elements
const nodeDetailsPanel = document.getElementById('node-details') as HTMLElement;
const nodeContentEl = document.getElementById('node-content') as HTMLElement;
const nodeConnectionsEl = document.getElementById('node-connections') as HTMLElement;
const closeDetailsBtn = document.getElementById('close-details') as HTMLButtonElement;

export function initGraph(): void {
    cy = cytoscape({
        container: document.getElementById('cy'),
        
        // Initial zoom and viewport settings
        zoom: 0.8,
        minZoom: 0.1,
        maxZoom: 3.0,
        wheelSensitivity: 5,
        
        // Performance optimizations
        hideEdgesOnViewport: true,
        hideLabelsOnViewport: true,
        textureOnViewport: true,
        motionBlur: true,
        motionBlurOpacity: 0.2,
        pixelRatio: 'auto',

        style: [
            {
                selector: 'node',
                style: {
                    'background-color': '#2563eb',
                    'label': 'data(label)',
                    'color': '#1e293b',
                    'text-valign': 'center',
                    'text-halign': 'center',
                    'font-size': 12,
                    'width': 50,
                    'height': 50,
                    'text-wrap': 'ellipsis',
                    'text-max-width': '80px',
                    'text-overflow-wrap': 'anywhere',
                    'overlay-padding': 8,
                    'z-compound-depth': 'auto',
                    'border-width': 2,
                    'border-color': '#e2e8f0',
                    'transition-property': 'background-color, border-color, border-width',
                    'transition-duration': 200
                }
            },
            {
                selector: 'node:active',
                style: {
                    'overlay-opacity': 0.2,
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
                    'arrow-scale': 0.8,
                    'opacity': 0.7,
                    'transition-property': 'line-color, width',
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
                    'height': 60,
                }
            },
            {
                selector: 'node.highlighted',
                style: {
                    'background-color': '#10b981',
                    'border-color': '#059669',
                    'border-width': 3,
                }
            },
            {
                selector: 'edge.highlighted',
                style: {
                    'line-color': '#10b981',
                    'target-arrow-color': '#10b981',
                    'width': 3,
                    'opacity': 1,
                }
            }
        ],
        
        // Layout configuration to prevent initial clustering
        layout: {
            name: 'preset' // Start with preset positions to avoid initial jumble
        }
    });

    // Event handlers
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

    // Batch rendering for better performance
    if (cy) {
        cy.batch(() => {
            cy!.nodes().forEach(node => {
                node.data('initPos', { x: node.position('x'), y: node.position('y') });
            });
        });
    }

    closeDetailsBtn.addEventListener('click', () => {
        hideNodeDetails();
        unhighlightAll();
    });
    
    window.cy = cy;
}

function highlightConnectedNodes(node: cytoscape.NodeSingular): void {
    if (!cy) return;
    
    cy.batch(() => {
        // Reset all highlights
        cy!.elements().removeClass('highlighted');
        
        // Highlight the selected node
        node.addClass('highlighted');
        
        // Highlight connected nodes and edges
        const connectedEdges = node.connectedEdges();
        const connectedNodes = node.neighborhood();
        
        connectedEdges.addClass('highlighted');
        connectedNodes.addClass('highlighted');
    });
}

function unhighlightAll(): void {
    if (!cy) return;
    cy.elements().removeClass('highlighted');
}

export async function refreshGraph(): Promise<void> {
    if (!cy) return;

    try {
        const graphData = await api.getGraphData();
        
        // Pre-process the data to add initial positions
        const processedElements = preprocessGraphData(graphData.elements);
        
        // Stop any running layout
        if (currentLayout) {
            currentLayout.stop();
        }
        
        // Batch all updates for performance
        cy.batch(() => {
            cy!.elements().remove();
            cy!.add(processedElements);
        });
        
        // Use a more efficient layout with better initial positions
        const layoutOptions: LayoutOptions = {
            name: 'cose',
            animate: false, // Disable animation for immediate positioning
            fit: true,
            padding: 50,
            
            // Improved physics parameters
            nodeRepulsion: () => 400000,
            idealEdgeLength: () => 150,
            nodeOverlap: 20,
            
            // Better distribution
            gravity: 0.25,
            numIter: 200, // Reduced iterations since we have good initial positions
            
            // Prevent edge crossings
            edgeElasticity: () => 100,
            nestingFactor: 0.9,
            
            // Initial positions from preprocessing
            initialTemp: 200,
            coolingFactor: 0.95,
            minTemp: 1.0,
            
            // Use the entire viewport
            boundingBox: { x1: 0, y1: 0, w: cy!.width(), h: cy!.height() },
            
            // Better convergence
            randomize: false, // Use our preprocessed positions
            componentSpacing: 40,
            
            // Called when layout stops
            stop: () => {
                // Smooth animation after initial positioning
                cy!.animate({
                    fit: {
                        eles: cy!.elements(),
                        padding: 30
                    },
                    duration: 300,
                    easing: 'ease-out'
                });
            }
        };
        
        currentLayout = cy.layout(layoutOptions);
        currentLayout.run();

    } catch (error) {
        console.error('Error refreshing graph:', error);
    }
}

// Preprocess graph data to assign better initial positions
function preprocessGraphData(elements: any[]): any[] {
    const nodes = elements.filter(el => el.data && !el.data.source);
    const edges = elements.filter(el => el.data && el.data.source);
    
    // Create adjacency map
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
    
    // Find nodes with most connections (hubs)
    const nodesByConnectivity = nodes.sort((a, b) => {
        const aConn = adjacency.get(a.data.id)?.size || 0;
        const bConn = adjacency.get(b.data.id)?.size || 0;
        return bConn - aConn;
    });
    
    // Position nodes in a circle with variations based on connectivity
    const centerX = window.innerWidth / 2;
    const centerY = 300;
    const baseRadius = Math.min(centerX * 0.8, 250);
    
    nodesByConnectivity.forEach((node, index) => {
        const angle = (index / nodes.length) * 2 * Math.PI;
        const connectivity = adjacency.get(node.data.id)?.size || 0;
        
        // Vary radius based on connectivity - hubs closer to center
        const radiusMultiplier = connectivity > 3 ? 0.7 : (connectivity > 1 ? 0.85 : 1);
        const radius = baseRadius * radiusMultiplier + (Math.random() * 40 - 20);
        
        node.position = {
            x: centerX + radius * Math.cos(angle),
            y: centerY + radius * Math.sin(angle)
        };
    });
    
    return elements;
}

async function showNodeDetails(nodeId: string): Promise<void> {
    if (!cy) return;

    try {
        const nodeData: NodeDetails = await api.getNode(nodeId);
        
        nodeContentEl.textContent = nodeData.content;
        
        if (nodeData.edges && nodeData.edges.length > 0) {
            const connectedNodesInfo = await Promise.all(
                nodeData.edges.map(async edgeId => {
                    const connectedNode = cy!.getElementById(edgeId);
                    if (connectedNode?.length) {
                        return {
                            id: edgeId,
                            label: connectedNode.data('label')
                        };
                    }
                    return null;
                })
            );
            
            const validNodes = connectedNodesInfo.filter(Boolean);
            const connectedNodesHtml = validNodes
                .map(node => `<li data-node-id="${node!.id}" class="clickable-connection">• ${node!.label}</li>`)
                .join('');
            
            nodeConnectionsEl.innerHTML = `
                <h4>Connected Memories (${validNodes.length})</h4>
                <ul>${connectedNodesHtml}</ul>
            `;
            
            // Add click handlers to connected nodes
            nodeConnectionsEl.querySelectorAll('.clickable-connection').forEach(li => {
                li.addEventListener('click', (e) => {
                    const targetNodeId = (e.currentTarget as HTMLElement).dataset.nodeId;
                    if (targetNodeId && cy) {
                        const targetNode = cy.getElementById(targetNodeId);
                        if (targetNode.length) {
                            targetNode.trigger('tap');
                            // Center on the clicked node
                            cy.animate({
                                center: { eles: targetNode },
                                zoom: 1.2,
                                duration: 300
                            });
                        }
                    }
                });
            });
        } else {
            nodeConnectionsEl.innerHTML = '<p>No connections yet</p>';
        }
        
        nodeDetailsPanel.style.display = 'block';
        cy.elements().unselect();
        cy.getElementById(nodeId).select();

    } catch (error) {
        console.error('Error loading node details:', error);
    }
}

function hideNodeDetails(): void {
    if (!cy) return;
    nodeDetailsPanel.style.display = 'none';
    cy.elements().unselect();
}