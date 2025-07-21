import cytoscape, { Core, LayoutOptions, ElementDefinition, NodeSingular, EdgeSingular } from 'cytoscape';
import { api } from './apiClient';
import { components } from './generated-types';

// Type alias for easier usage
type NodeDetails = components['schemas']['NodeDetails'];

let cy: Core | null | undefined = null;
let currentLayout: cytoscape.Layouts | null = null;

// DOM Elements are now cached within initGraph, as they are destroyed and recreated on sign-out.
let nodeDetailsPanel: HTMLElement | null;
let nodeContentEl: HTMLElement | null;
let nodeConnectionsEl: HTMLElement | null;
let closeDetailsBtn: HTMLButtonElement | null;

export function initGraph(): void {
    // Cache DOM elements now that the structure is guaranteed to exist
    nodeDetailsPanel = document.getElementById('node-details') as HTMLElement;
    nodeContentEl = document.getElementById('node-content') as HTMLElement;
    nodeConnectionsEl = document.getElementById('node-connections') as HTMLElement;
    closeDetailsBtn = document.getElementById('close-details') as HTMLButtonElement;

    cy = cytoscape({
        container: document.getElementById('cy'),
        zoom: 0.8,
        minZoom: 0.1,
        maxZoom: 3.0,
        wheelSensitivity: 2,
        hideEdgesOnViewport: true,
        hideLabelsOnViewport: true,
        // gotta learn what this thing is doing, but this is causing gray on zoomout
        textureOnViewport: false,
        motionBlur: true,
        motionBlurOpacity: 0.2,
        pixelRatio: 'auto',
        style: [
            {
                selector: 'node',
                style: {
                    'background-color': '#2563eb', 'label': 'data(label)', 'color': '#1e293b',
                    'text-valign': 'center', 'text-halign': 'center', 'font-size': 12, 'width': 50, 'height': 50,
                    'text-wrap': 'ellipsis', 'text-max-width': '80px', 'text-overflow-wrap': 'anywhere',
                    'overlay-padding': 8, 'z-compound-depth': 'auto', 'border-width': 2, 'border-color': '#e2e8f0',
                    'transition-property': 'background-color, border-color, border-width, width, height, opacity',
                    'transition-duration': 200
                }
            },
            { selector: 'node:active', style: { 'overlay-opacity': 0.2 } },
            {
                selector: 'edge',
                style: {
                    'width': 1.5, 'line-color': '#cbd5e1', 'target-arrow-color': '#cbd5e1',
                    'target-arrow-shape': 'triangle', 'curve-style': 'bezier', 'arrow-scale': 0.8,
                    'opacity': 0.7, 'transition-property': 'line-color, width, opacity', 'transition-duration': 200
                }
            },
            {
                selector: 'node:selected',
                style: { 'background-color': '#1d4ed8', 'border-width': 3, 'border-color': '#2563eb', 'width': 60, 'height': 60 }
            },
            {
                selector: 'node.highlighted',
                style: { 'background-color': '#10b981', 'border-color': '#059669', 'border-width': 3 }
            },
            {
                selector: 'node.newly-added',
                style: { 'background-color': '#10b981', 'border-color': '#059669', 'border-width': 3 }
            },
            {
                selector: 'edge.highlighted',
                style: { 'line-color': '#10b981', 'target-arrow-color': '#10b981', 'width': 3, 'opacity': 1 }
            }
        ],
        layout: { name: 'preset' }
    });

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

    if (closeDetailsBtn) {
        closeDetailsBtn.addEventListener('click', () => {
            hideNodeDetails();
            unhighlightAll();
        });
    }
    
    window.cy = cy;
}

/**
 * Destroys the Cytoscape instance and cleans up state, crucial for logout.
 */
export function destroyGraph(): void {
    if (cy) {
        cy.destroy();
        cy = null;
        window.cy = undefined; // Use undefined to clear from global scope
        console.log("Graph instance destroyed.");
    }
    // Also hide the details panel as it may contain old data
    hideNodeDetails();
}

function highlightConnectedNodes(node: cytoscape.NodeSingular): void {
    if (!cy) return;
    cy.batch(() => {
        cy!.elements().removeClass('highlighted');
        node.addClass('highlighted');
        node.neighborhood().addClass('highlighted');
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
        const processedElements = preprocessGraphData(graphData.elements ?? []);
        if (currentLayout) currentLayout.stop();
        cy.batch(() => {
            cy!.elements().remove();
            cy!.add(processedElements as ElementDefinition[]);
        });
        const layoutOptions: LayoutOptions = {
            name: 'cose', animate: false, fit: true, padding: 50,
            nodeRepulsion: () => 400000, idealEdgeLength: () => 150, nodeOverlap: 20,
            gravity: 0.25, numIter: 200, edgeElasticity: () => 100, nestingFactor: 0.9,
            initialTemp: 200, coolingFactor: 0.95, minTemp: 1.0,
            boundingBox: { x1: 0, y1: 0, w: cy!.width(), h: cy!.height() },
            randomize: false, componentSpacing: 40,
            stop: () => {
                cy!.animate({
                    fit: { eles: cy!.elements(), padding: 30 },
                    duration: 300, easing: 'ease-out'
                } as any);
            }
        };
        currentLayout = cy.layout(layoutOptions);
        currentLayout.run();
    } catch (error) {
        console.error('Error refreshing graph:', error);
    }
}

export async function addNodeAndAnimate(nodeDetails: NodeDetails): Promise<void> {
    console.log("[Graph-Viz] Starting addNodeAndAnimate for node:", nodeDetails.nodeId);
    
    if (!cy) {
        console.error("[Graph-Viz] Cytoscape instance is null");
        return;
    }
    
    if (cy.getElementById(nodeDetails.nodeId!).length > 0) {
        console.log("[Graph-Viz] Node already exists, skipping animation:", nodeDetails.nodeId);
        return;
    }

    const existingNodes = cy.nodes();
    try {
        console.log("[Graph-Viz] Locking existing nodes");
        existingNodes.lock();
        
        // Calculate base position and radius based on viewport or neighbors
        let basePosition = { x: cy.pan().x, y: cy.pan().y };
        let baseRadius = Math.min(cy.width(), cy.height()) * 0.2; // 20% of viewport size
        
        const connectedNodeIds = (nodeDetails.edges || []);
        console.log("[Graph-Viz] Connected node IDs:", connectedNodeIds);
        
        if (connectedNodeIds.length > 0) {
            const neighborNodes = cy.nodes().filter(node => connectedNodeIds.includes(node.id()));
            console.log("[Graph-Viz] Found neighbor nodes:", neighborNodes.length);
            if (neighborNodes.length > 0) {
                const bb = neighborNodes.boundingBox();
                basePosition = { x: bb.x1 + bb.w / 2, y: bb.y1 + bb.h / 2 };
                // Use smaller radius when adding to existing cluster
                baseRadius = Math.max(bb.w, bb.h) * 0.75;
                console.log("[Graph-Viz] Calculated base position:", basePosition);
            }
        }
        
        // Calculate position with random angle but consistent radius
        const angle = Math.random() * 2 * Math.PI;
        const jitter = baseRadius * 0.2; // 20% random variation in radius
        const radius = baseRadius + (Math.random() * jitter - jitter/2);
        
        const initialPosition = {
            x: basePosition.x + radius * Math.cos(angle),
            y: basePosition.y + radius * Math.sin(angle)
        };
        console.log("[Graph-Viz] Final position with offset:", initialPosition);
        
        const label = nodeDetails.content ? (nodeDetails.content.length > 50 ? nodeDetails.content.substring(0, 47) + '...' : nodeDetails.content) : '';
        const newNodeElement: ElementDefinition = {
            group: 'nodes', data: { id: nodeDetails.nodeId, label: label },
            style: { 'opacity': 0, 'width': 1, 'height': 1 },
            position: initialPosition, classes: 'newly-added'
        };
        
        console.log("[Graph-Viz] Creating new node element:", newNodeElement);
        const addedNode = cy.add(newNodeElement);
        
        console.log("[Graph-Viz] Starting node animation");
        await addedNode.animation({
            style: { 'opacity': 1, 'width': 50, 'height': 50 },
            duration: 800, easing: 'ease-out-cubic'
        } as any).play().promise();
        console.log("[Graph-Viz] Node animation completed");

        const newEdgeElements: ElementDefinition[] = (nodeDetails.edges || []).map(targetId => ({
            group: 'edges', data: { id: `edge-${nodeDetails.nodeId}-${targetId}`, source: nodeDetails.nodeId!, target: targetId },
            style: { 'opacity': 0 }
        }));
        
        console.log("[Graph-Viz] Adding edges:", newEdgeElements.length);
        for (const edgeDef of newEdgeElements) {
            console.log("[Graph-Viz] Adding edge:", edgeDef.data.id);
            cy.add(edgeDef).animation({
                style: { 'opacity': 0.7 },
                duration: 600
            } as any).play();
            await new Promise(resolve => setTimeout(resolve, 150));
        }
        
        console.log("[Graph-Viz] Running layout");
        const layout = cy.layout({
            name: 'cose',
            eles: addedNode.union(addedNode.neighborhood()),
            fit: false,
            animate: true,
            animationDuration: 1000,
            padding: 80,
            // Prevent overlap with adaptive spacing
            nodeRepulsion: (node: NodeSingular) => {
                const degree = node.degree(false); // Number of connections (undirected)
                return 4500 + (degree * 500); // More repulsion for highly connected nodes
            },
            nodeOverlap: 20,
            idealEdgeLength: (edge: EdgeSingular) => {
                const sourceConnections = edge.source().degree(false);
                const targetConnections = edge.target().degree(false);
                const avgConnections = (sourceConnections + targetConnections) / 2;
                return 100 + (avgConnections * 10); // Longer edges for highly connected nodes
            },
            springCoeff: 0.0008,
            gravity: 0.25,
            initialTemp: 200,
            coolingFactor: 0.95,
            minTemp: 1.0
        } as any);
        layout.run();
        
        setTimeout(() => {
            console.log("[Graph-Viz] Cleanup: removing newly-added class and unlocking nodes");
            addedNode.removeClass('newly-added');
            existingNodes.unlock();
        }, 2500);
        
        console.log("[Graph-Viz] Node addition and animation completed successfully");
    } catch (error) {
        console.error('[Graph-Viz] Error adding and animating node:', error);
        existingNodes.unlock();
        console.log("[Graph-Viz] Falling back to full graph refresh");
        await refreshGraph();
    }
}

function preprocessGraphData(elements: any[]): any[] {
    const nodes = elements.filter(el => el.data && !el.data.source);
    const edges = elements.filter(el => el.data && el.data.source);
    
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
    
    const nodesByConnectivity = nodes.sort((a, b) => {
        const aConn = adjacency.get(a.data.id)?.size || 0;
        const bConn = adjacency.get(b.data.id)?.size || 0;
        return bConn - aConn;
    });
    
    const centerX = window.innerWidth / 2;
    const centerY = 300;
    const baseRadius = Math.min(centerX * 0.8, 250);
    
    nodesByConnectivity.forEach((node, index) => {
        const angle = (index / nodes.length) * 2 * Math.PI;
        const connectivity = adjacency.get(node.data.id)?.size || 0;
        
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
    if (!cy || !nodeDetailsPanel || !nodeContentEl || !nodeConnectionsEl) return;
    try {
        const nodeData: NodeDetails = await api.getNode(nodeId);
        nodeContentEl.textContent = nodeData.content || '';
        if (nodeData.edges && nodeData.edges.length > 0) {
            const connectedNodesInfo = await Promise.all(
                nodeData.edges.map(async edgeId => {
                    const connectedNode = cy!.getElementById(edgeId);
                    return connectedNode?.length ? { id: edgeId, label: connectedNode.data('label') } : null;
                })
            );
            const validNodes = connectedNodesInfo.filter(Boolean);
            nodeConnectionsEl.innerHTML = `<h4>Connected Memories (${validNodes.length})</h4><ul>${validNodes.map(node => `<li data-node-id="${node!.id}" class="clickable-connection">• ${node!.label}</li>`).join('')}</ul>`;
            nodeConnectionsEl.querySelectorAll('.clickable-connection').forEach(li => {
                li.addEventListener('click', (e) => {
                    const targetNodeId = (e.currentTarget as HTMLElement).dataset.nodeId;
                    if (targetNodeId && cy) {
                        const targetNode = cy.getElementById(targetNodeId);
                        if (targetNode.length) {
                            targetNode.trigger('tap');
                            cy.animate({ center: { eles: targetNode }, zoom: 1.2, duration: 300 } as any);
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
    if (!cy || !nodeDetailsPanel) return;
    nodeDetailsPanel.style.display = 'none';
    cy.elements().unselect();
}
