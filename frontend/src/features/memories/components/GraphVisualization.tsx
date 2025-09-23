import React, { useEffect, useRef, useState, useImperativeHandle, forwardRef, memo, useCallback } from 'react';
import cytoscape, { Core, ElementDefinition, NodeSingular, NodeCollection } from 'cytoscape';
import cola from 'cytoscape-cola';
import { nodesApi } from '../api/nodes';
import type { NodeDetails } from '../../../services';
import { useFullscreen } from '../../../common/hooks/useFullscreen';
import { throttle } from 'lodash-es';
import GraphControls from './GraphControls';
import NodeDetailsPanel from './NodeDetailsPanel';
import DocumentModeView from './NodeDetailsPanel/DocumentModeView';
import StarField from './StarField';
import styles from './GraphVisualization.module.css';

// Register the cola layout
cytoscape.use(cola);

interface GraphVisualizationProps {
    /** Trigger number that causes graph refresh when changed */
    refreshTrigger: number;
    /** Whether the graph has an overlay input (affects layout) */
    hasOverlayInput?: boolean;
}

export interface GraphVisualizationRef {
    /** Programmatically select and center a specific node */
    selectAndCenterNode: (nodeId: string) => boolean;
    /** Programmatically hide/close the node details panel */
    hideNodeDetails: () => void;
}

interface DisplayNode {
    id: string;
    content: string;
    title?: string;
    label: string;
    timestamp: string;
    tags?: string[];
}

interface ConnectedMemory {
    id: string;
    label: string;
}

// Cosmic color spectrum for node clustering - vibrant space colors
const NODE_COLORS = [
    '#00d4ff', // electric cyan
    '#ff006e', // cosmic magenta
    '#8338ec', // nebula purple
    '#ffbe0b', // stellar yellow
    '#fb5607', // solar orange
    '#3a86ff', // deep space blue
    '#06ffa5', // alien green
    '#ff4081', // plasma pink
    '#7209b7', // galaxy purple
    '#f72585', // supernova red
] as const;

const GraphVisualization = forwardRef<GraphVisualizationRef, GraphVisualizationProps>(({ refreshTrigger, hasOverlayInput = false }, ref) => {
    const containerRef = useRef<HTMLDivElement>(null);
    const graphContainerRef = useRef<HTMLDivElement>(null);
    const cyRef = useRef<Core | null>(null);
    const [selectedNode, setSelectedNode] = useState<DisplayNode | null>(null);
    const [connectedMemories, setConnectedMemories] = useState<ConnectedMemory[]>([]);
    const [currentElementCount, setCurrentElementCount] = useState(0);
    const [prefersReducedMotion, setPrefersReducedMotion] = useState(false);

    // Interval refs for cleanup
    const viewportIntervalRef = useRef<number | null>(null);
    const jitterIntervalRef = useRef<number | null>(null);
    const pulseIntervalRef = useRef<number | null>(null);
    
    // Document mode state
    const [isDocumentMode, setIsDocumentMode] = useState(false);
    const [documentModeNode, setDocumentModeNode] = useState<DisplayNode | null>(null);
    const [documentModeConnections, setDocumentModeConnections] = useState<ConnectedMemory[]>([]);
    
    // Fullscreen functionality
    const { isFullscreen, toggleFullscreen } = useFullscreen(graphContainerRef);

    // Define hideNodeDetails early to avoid initialization errors
    const hideNodeDetails = useCallback((): void => {
        setSelectedNode(null);
        setConnectedMemories([]);
    }, []);

    // Expose methods to parent component via ref
    useImperativeHandle(ref, () => ({
        selectAndCenterNode: (nodeId: string) => {
            if (!cyRef.current) return false;
            
            const cy = cyRef.current;
            const node = cy.getElementById(nodeId);
            
            if (node.length === 0) {
                console.warn('Node not found in graph');
                return false;
            }
            
            // Center and zoom to the node
            cy.animate({
                center: { eles: node },
                zoom: 1.5
            }, {
                duration: 500,
                easing: 'ease-in-out'
            });
            
            // Highlight the node and its connections
            highlightConnectedNodes(node);
            
            // Show node details
            showNodeDetails(nodeId);
            
            return true;
        },
        hideNodeDetails
    }), [hideNodeDetails]);

    // Respect reduced motion user preference
    useEffect(() => {
        if (typeof window === 'undefined' || typeof window.matchMedia !== 'function') {
            return;
        }
        const mql = window.matchMedia('(prefers-reduced-motion: reduce)');
        const handler = () => setPrefersReducedMotion(mql.matches);
        handler();
        mql.addEventListener('change', handler);
        return () => mql.removeEventListener('change', handler);
    }, []);

    // Force resize on mobile to ensure proper dimensions
    useEffect(() => {
        if (cyRef.current && window.innerWidth <= 480) {
            setTimeout(() => {
                cyRef.current?.resize();
            }, 100);
        }
    }, []);

    // Initialize cytoscape once on component mount
    useEffect(() => {
        if (!containerRef.current) return;

        const cy = cytoscape({
            container: containerRef.current,
            elements: [],
            zoom: 0.8,
            minZoom: 0.1,
            maxZoom: 10,
            wheelSensitivity: 2,
            motionBlur: false,
            style: [
                {
                    selector: 'node',
                    style: {
                        'background-color': 'data(color)',
                        'width': 25,
                        'height': 25,
                        'border-width': 2,
                        'border-color': 'data(color)',
                        'border-opacity': 1,
                        'label': '', // No labels for cleaner look,
                        'transition-property': 'background-color, border-width, border-color, width, height, opacity',
                        'transition-duration': 300
                    }
                },
                {
                    selector: 'edge',
                    style: {
                        'width': (ele: any) => {
                            // Scale width based on edge strength
                            const strength = ele.data('strength') || 0.5;
                            return Math.max(1.5, Math.min(strength * 7, 12));
                        },
                        'line-color': '#cbd5e1',
                        'target-arrow-color': '#cbd5e1',
                        'opacity': 0.7,
                        'curve-style': 'bezier',
                        'transition-property': 'width, line-color, opacity',
                        'transition-duration': 300,
                        'target-arrow-shape': 'none',
                        'z-index': 0
                    }
                },
                {
                    selector: 'edge[color]',
                    style: {
                        'line-color': 'data(color)',
                        'target-arrow-color': 'data(color)'
                    }
                },
                {
                    selector: 'node:selected',
                    style: {
                        'border-width': 4,
                        'border-color': '#ffffff',
                        'width': 35,
                        'height': 35,
                        'z-index': 999
                    }
                },
                {
                    selector: 'edge:selected',
                    style: {
                        'width': 5,
                        'opacity': 1,
                        'z-index': 998
                    }
                },
                {
                    selector: '.highlighted',
                    style: {
                        'opacity': 1,
                        'z-index': 900
                    }
                }
            ],
            layout: { name: 'preset' }
        });

        cyRef.current = cy;

        // Ensure proper sizing, especially on mobile
        if (window.innerWidth <= 480) {
            setTimeout(() => cy.resize(), 200);
        }

        // Set up event handlers
        const teardownEvents = setupEventHandlers(cy);

        // Store global reference for legacy compatibility
        (window as any).cy = cy;

        // Prevent unwanted viewport resets
        const teardownViewport = preventViewportReset(cy);

        return () => {
            if (cyRef.current) {
                cyRef.current.destroy();
                cyRef.current = null;
            }
            // Clear any running intervals
            if (viewportIntervalRef.current) {
                window.clearInterval(viewportIntervalRef.current);
                viewportIntervalRef.current = null;
            }
            if (jitterIntervalRef.current) {
                window.clearInterval(jitterIntervalRef.current);
                jitterIntervalRef.current = null;
            }
            if (pulseIntervalRef.current) {
                window.clearInterval(pulseIntervalRef.current);
                pulseIntervalRef.current = null;
            }
            // Additional teardown
            teardownEvents?.();
            teardownViewport?.();
        };
    }, []);

    // Memoized event handlers to prevent recreation on every render
    const setupEventHandlers = useCallback((cy: Core) => {
        const handleNodeTap = (evt: any) => {
            const node = evt.target;
            highlightConnectedNodes(node);
            showNodeDetails(node.id());
        };

        const handleBackgroundTap = (evt: any) => {
            if (evt.target === cy) {
                hideNodeDetails();
                unhighlightAll();
            }
        };

        cy.on('tap', 'node', handleNodeTap);
        cy.on('tap', handleBackgroundTap);

        // Return cleanup function
        return () => {
            cy.off('tap', 'node', handleNodeTap);
            cy.off('tap', handleBackgroundTap);
        };
    }, []);

    const preventViewportReset = (cy: Core) => {
        // Create a custom viewport manager that keeps track of user's view
        let userViewport = {
            zoom: cy.zoom(),
            pan: cy.pan()
        };
        
        // Only update when user intentionally changes view
        let viewChanged = false;
        
        // Listen for user-initiated zoom/pan
        cy.on('zoom pan', () => {
            if (!viewChanged) {
                userViewport = {
                    zoom: cy.zoom(),
                    pan: cy.pan()
                };
            }
        });
        
        // Override the fit function
        const originalFit = cy.fit;
        cy.fit = function(eles?: any, padding?: number) {
            // Check for manual reset via a different approach
            if (arguments[2] === true || (arguments[0] as any)?.reset === true) {
                viewChanged = true;
                const result = originalFit.apply(cy, [eles, padding]);
                
                // Update user viewport after manual reset
                setTimeout(() => {
                    userViewport = {
                        zoom: cy.zoom(),
                        pan: cy.pan()
                    };
                    viewChanged = false;
                }, 100);
                
                return result;
            }
            return cy;
        };
        
        // Periodically check if view was reset unexpectedly
        viewportIntervalRef.current = window.setInterval(() => {
            // Don't restore during user interactions
            if (cy.nodes().filter(':grabbed').length > 0) return;
            if (document.visibilityState !== 'visible') return;
            
            const currentZoom = cy.zoom();
            const currentPan = cy.pan();
            
            // If viewport changed without user action, restore
            if (!viewChanged && 
                (Math.abs(currentZoom - userViewport.zoom) > 0.01 ||
                 Math.abs(currentPan.x - userViewport.pan.x) > 10 ||
                 Math.abs(currentPan.y - userViewport.pan.y) > 10)) {
                
                cy.viewport({
                    zoom: userViewport.zoom,
                    pan: userViewport.pan
                });
            }
        }, 200);
        
        // For zoom button handlers
        const wheelHandler = (event: any) => {
            // Ensure zoom stays within bounds
            const zoom = cy.zoom();
            if (zoom <= cy.minZoom() || zoom >= cy.maxZoom()) {
                event.preventDefault();
            }
        };
        cy.on('wheelzoom', wheelHandler);

        // Teardown
        return () => {
            if (viewportIntervalRef.current) {
                window.clearInterval(viewportIntervalRef.current);
                viewportIntervalRef.current = null;
            }
            cy.off('wheelzoom', wheelHandler);
        };
    };

    // Memoized graph manipulation functions
    const highlightConnectedNodes = useCallback((node: cytoscape.NodeSingular): void => {
        if (!cyRef.current) return;
        const cy = cyRef.current;
        cy.batch(() => {
            cy.elements().removeClass('highlighted');
            node.addClass('highlighted');
            node.neighborhood().addClass('highlighted');
        });
    }, []);

    const unhighlightAll = useCallback((): void => {
        if (!cyRef.current) return;
        cyRef.current.elements().removeClass('highlighted');
    }, []);

    async function showNodeDetails(nodeId: string): Promise<void> {
        try {
            const nodeData: NodeDetails = await nodesApi.getNode(nodeId);
            setSelectedNode({
                id: nodeId,
                content: nodeData.content || '',
                title: nodeData.title,
                label: nodeData.title || (nodeData.content ? (nodeData.content.length > 50 ? nodeData.content.substring(0, 47) + '...' : nodeData.content) : ''),
                timestamp: nodeData.timestamp || '',
                tags: nodeData.tags || []
            });

            // Process connected memories
            // nodeData.edges contains node IDs of connected memories, not edge IDs
            if (nodeData.edges && nodeData.edges.length > 0 && cyRef.current) {
                const cy = cyRef.current;
                const connectedNodesInfo = nodeData.edges.map(connectedNodeId => {
                    const connectedNode = cy.getElementById(connectedNodeId);
                    if (connectedNode && connectedNode.length > 0) {
                        return {
                            id: connectedNodeId,
                            label: connectedNode.data('label') || 'Untitled'
                        };
                    }
                    return null;
                }).filter((node): node is ConnectedMemory => node !== null);

                setConnectedMemories(connectedNodesInfo);
            } else {
                setConnectedMemories([]);
            }
        } catch (error) {
            console.error('Error loading node details:', error);
            setConnectedMemories([]);
        }
    }

    // Handle opening document mode
    const handleOpenDocumentMode = useCallback((node: DisplayNode, connections: ConnectedMemory[]): void => {
        // Save the node data for document mode
        setDocumentModeNode(node);
        setDocumentModeConnections(connections);
        setIsDocumentMode(true);
        
        // Clear the node details panel
        hideNodeDetails();
    }, [hideNodeDetails]);
    
    // Handle closing document mode
    const handleCloseDocumentMode = useCallback((): void => {
        setIsDocumentMode(false);
        setDocumentModeNode(null);
        setDocumentModeConnections([]);
    }, []);

    const handleConnectedMemoryClick = useCallback((memoryId: string): void => {
        if (!cyRef.current) return;
        
        const cy = cyRef.current;
        const targetNode = cy.getElementById(memoryId);
        
        if (targetNode && targetNode.length > 0) {
            // Center and highlight the connected node
            cy.animate({
                center: { eles: targetNode },
                zoom: 1.5
            }, {
                duration: 300,
                easing: 'ease-out'
            });
            
            // Highlight the connected node and its connections
            highlightConnectedNodes(targetNode);
            
            // Show details for the connected node
            showNodeDetails(memoryId);
        }
    }, []);

    // Handle fullscreen changes - resize cytoscape when entering/exiting fullscreen
    useEffect(() => {
        if (cyRef.current) {
            // Different delays for entering vs exiting fullscreen
            const delay = isFullscreen ? 100 : 500; // Even longer delay for exit to ensure proper restoration
            
            const timer = setTimeout(() => {
                // Enhanced layout restoration for fullscreen exit
                if (!isFullscreen && graphContainerRef.current) {
                    // Reset container dimensions explicitly
                    const container = graphContainerRef.current;
                    container.style.width = '';
                    container.style.height = '';
                    container.style.maxWidth = '';
                    container.style.maxHeight = '';
                    
                    // Force layout recalculation for parent containers
                    const mainContentArea = container.closest('.main-content-area');
                    const dashboardLayout = container.closest('.dashboard-layout-refined');
                    
                    [mainContentArea, dashboardLayout].forEach(parent => {
                        if (parent) {
                            const element = parent as HTMLElement;
                            const display = element.style.display;
                            element.style.display = 'none';
                            element.offsetHeight; // Force reflow
                            element.style.display = display || 'flex';
                        }
                    });
                    
                    // Multiple resize events to ensure all components update
                    setTimeout(() => window.dispatchEvent(new Event('resize')), 0);
                    setTimeout(() => window.dispatchEvent(new Event('resize')), 50);
                    setTimeout(() => window.dispatchEvent(new Event('resize')), 100);
                }
                
                // Resize cytoscape after layout restoration
                cyRef.current?.resize();
                
                // Handle viewport positioning
                if (cyRef.current && cyRef.current.elements().length > 0) {
                    if (isFullscreen) {
                        cyRef.current.fit();
                        cyRef.current.center();
                    } else {
                        // Additional resize after a small delay to ensure proper dimensions
                        setTimeout(() => {
                            cyRef.current?.resize();
                        }, 100);
                    }
                }
            }, delay);

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
            const graphData = await nodesApi.getGraphData() as any;
            
            // Convert new API format to Cytoscape elements format
            const elements: any[] = [];
            
            // Add nodes
            if (graphData.nodes) {
                graphData.nodes.forEach((node: any) => {
                    elements.push({
                        data: {
                            id: node.id,
                            label: node.title || node.content || 'Untitled',
                            content: node.content,
                            title: node.title,
                            timestamp: node.metadata?.created_at || node.metadata?.updated_at,
                            tags: node.tags || []
                        },
                        position: node.position || { x: Math.random() * 500, y: Math.random() * 500 }
                    });
                });
            }
            
            // Add edges
            if (graphData.edges) {
                graphData.edges.forEach((edge: any) => {
                    elements.push({
                        data: {
                            id: edge.id || `${edge.source}-${edge.target}`,
                            source: edge.source,
                            target: edge.target,
                            weight: edge.weight || 1,
                            type: edge.type
                        }
                    });
                });
            }
            
            // Fallback to old format if needed
            if (!graphData.nodes && !graphData.edges && graphData.elements) {
                elements.push(...(graphData.elements || []));
            }
            
            const cy = cyRef.current;
            
            // Check if we actually need to update
            const newCount = elements.length;
            
            // If both current and new are 0, no update needed
            if (currentElementCount === 0 && newCount === 0) {
                return;
            }
            
            // Always clear existing elements (handles transition to empty state)
            cy.batch(() => {
                cy.elements().remove();
                
                // Only add new elements if we have any
                if (elements.length > 0) {
                    const processedElements = preprocessGraphData(elements);
                    cy.add(processedElements as ElementDefinition[]);
                }
            });
            
            // Update our state tracking
            setCurrentElementCount(newCount);
            
            // Only apply layout and effects if we have elements
            if (elements.length > 0) {
                // Apply cola layout with advanced physics parameters
                const layout = cy.layout({
                    name: 'cola',
                    animate: !prefersReducedMotion,
                    refresh: 1,
                    maxSimulationTime: 7000,
                    nodeSpacing: function() { return 50; },
                    edgeLength: function(edge: any) {
                        // Dynamic edge length based on connection strength if available
                        const strength = edge.data('strength') || 0.5;
                        return 80 + (1 - strength) * 150;
                    },
                    // Physics parameters for interactive feel
                    gravity: 0.3,
                    padding: 30,
                    avoidOverlap: true,
                    randomize: false,
                    unconstrIter: 10,
                    userConstIter: 15,
                    allConstIter: 20,
                    // Key physics parameters for dragging
                    handleDisconnected: true,
                    convergenceThreshold: 0.001,
                    flow: {
                        enabled: true,          
                        friction: 0.6
                    },
                    infinite: prefersReducedMotion ? false : true,
                    stop: () => {
                        cy.animate({
                            fit: { eles: cy.elements(), padding: 30 },
                            duration: 300
                        } as any);
                    }
                } as any);

                layout.run();

                // Add background effects
                addBackgroundEffects();
                
                // Setup interactive drag behavior
                setupDragBehavior(cy);
                
                // Continuous node animations handled by effect with cleanup
            }

        } catch (error) {
            console.error('Error loading graph data:', error);
        }
    };

    // Memoized graph preprocessing to avoid recalculating on every render
    const preprocessGraphData = useCallback((elements: any[]): any[] => {
        const nodes = elements.filter(el => el.data && !el.data.source);
        const edges = elements.filter(el => el.data && el.data.source);
        
        // Create a set of all node IDs for quick lookup
        const nodeIdSet = new Set(nodes.map(node => node.data.id));

        // Filter out invalid edges (edges with non-existent source or target nodes)
        const validEdges = edges.filter(edge => {
            const sourceExists = nodeIdSet.has(edge.data.source);
            const targetExists = nodeIdSet.has(edge.data.target);
            if (!sourceExists) {
                console.warn(`Edge ${edge.data.id} has non-existent source node: ${edge.data.source}`);
            }
            if (!targetExists) {
                console.warn(`Edge ${edge.data.id} has non-existent target node: ${edge.data.target}`);
            }
            return sourceExists && targetExists;
        });

        // Create adjacency map for clustering (using only valid edges)
        const adjacency = new Map<string, Set<string>>();
        validEdges.forEach(edge => { // Use validEdges here
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
        
        return [...processedNodes, ...validEdges];
    }, []);

    const setupDragBehavior = useCallback((cy: Core) => {
        let draggedNode: NodeSingular | null = null;
        let connectedNodes: NodeCollection = cy.collection();
        
        cy.on('grab', 'node', function(e) {
            draggedNode = e.target;
            if (!draggedNode) return;
            connectedNodes = draggedNode.neighborhood().nodes();
            
            // Visual feedback
            draggedNode.style({
                'border-width': 3,
                'border-color': 'white'
            });
        });
        
        cy.on('drag', 'node', throttle(function() {
            if (!draggedNode) return;
            
            requestAnimationFrame(() => {
                // Pull connected nodes with diminishing effect
                connectedNodes.forEach(node => {
                    // Skip the dragged node itself
                    if (!draggedNode) return;
                    if (node.id() === draggedNode.id()) return;
                    
                    // Find connection strength
                    const edge = cy.edges().filter(edge => 
                        (edge.source().id() === draggedNode!.id() && edge.target().id() === node.id()) ||
                        (edge.target().id() === draggedNode!.id() && edge.source().id() === node.id())
                    );
                    
                    if (edge.length === 0) return;
                    
                    // Use connection strength for pull effect
                    const strength = edge.data('strength') * 0.02 || 0.005;
                    const nodePos = node.position();
                    const draggedPos = draggedNode.position();
                    
                    // Apply pull effect
                    node.position({
                        x: nodePos.x + (draggedPos.x - nodePos.x) * strength,
                        y: nodePos.y + (draggedPos.y - nodePos.y) * strength
                    });
                });
            });
        }, 30));
        
        cy.on('free', 'node', function() {
            if (!draggedNode) return;
            
            // Reset styles
            draggedNode.style({
                'border-width': 2,
                'border-color': draggedNode.data('color')
            });
            
            // Reset variables
            draggedNode = null;
            connectedNodes = cy.collection();
        });
    }, []);

    const animateNodes = () => {
        if (!cyRef.current) return;
        
        const cy = cyRef.current;
        
        // Apply subtle random movement to keep graph alive
        setInterval(() => {
            // Only apply jitter if graph is not being interacted with
            if (cy.nodes().filter(':grabbed').length === 0) {
                // Add tiny random movements to random nodes
                cy.nodes().forEach(node => {
                    if (Math.random() > 0.7) {  // Only affect ~30% of nodes each time
                        const jitter = (Math.random() - 0.5) * 1;
                        const pos = node.position();
                        node.position({
                            x: pos.x + jitter,
                            y: pos.y + jitter
                        });
                    }
                });
            }
        }, 2000);
        
        // Occasionally add pulse effects to random nodes
        setInterval(() => {
            if (!cyRef.current) return;
            if (cy.nodes().filter(':grabbed').length === 0) {
                const nodes = cy.nodes();
                if (nodes.length === 0) return;
                
                const randomNodeIndex = Math.floor(Math.random() * nodes.length);
                const randomNode = nodes[randomNodeIndex];
                
                // Don't animate if node is selected or being dragged
                if (randomNode.selected() || randomNode.grabbed()) return;
                
                // Add a subtle pulse effect with fixed dimensions to prevent size drift
                const originalWidth = 25;  // Fixed base size from CSS
                const originalHeight = 25; // Fixed base size from CSS
                
                randomNode.animate({
                    style: { 
                        'width': originalWidth * 1.1, 
                        'height': originalHeight * 1.1,
                        'border-width': 7
                    }
                }, {
                    duration: 800,
                    easing: 'ease-in-sine',
                    complete: function() {
                        randomNode.animate({
                            style: { 
                                'width': originalWidth, 
                                'height': originalHeight,
                                'border-width': 2
                            }
                        }, {
                            duration: 800,
                            easing: 'ease-out-sine'
                        });
                    }
                });
            }
        }, 700);
    };

    const addBackgroundEffects = useCallback(() => {
        // Background effects are now handled by the StarField component
        // This function is kept for backward compatibility but does nothing
    }, []);

    // Start/stop subtle animations once, with cleanup and guards
    useEffect(() => {
        if (!cyRef.current) return;
        if (prefersReducedMotion) return;

        const cy = cyRef.current;

        // Apply subtle random movement to keep graph alive
        jitterIntervalRef.current = window.setInterval(() => {
            if (!cyRef.current) return;
            if (document.visibilityState !== 'visible') return;
            if (cy.nodes().filter(':grabbed').length > 0) return;

            cy.nodes().forEach(node => {
                if (Math.random() > 0.7) {
                    const jitter = (Math.random() - 0.5) * 1;
                    const pos = node.position();
                    node.position({ x: pos.x + jitter, y: pos.y + jitter });
                }
            });
        }, 2000);

        // Occasionally add pulse effects to random nodes
        pulseIntervalRef.current = window.setInterval(() => {
            if (!cyRef.current) return;
            if (document.visibilityState !== 'visible') return;
            if (cy.nodes().filter(':grabbed').length > 0) return;

            const nodes = cy.nodes();
            if (nodes.length === 0) return;
            const randomNodeIndex = Math.floor(Math.random() * nodes.length);
            const randomNode = nodes[randomNodeIndex];
            if (randomNode.selected() || randomNode.grabbed()) return;

            const originalWidth = 25;
            const originalHeight = 25;
            randomNode.animate({
                style: { 'width': originalWidth * 1.1, 'height': originalHeight * 1.1, 'border-width': 7 }
            }, {
                duration: 800,
                easing: 'ease-in-sine',
                complete: function() {
                    randomNode.animate({
                        style: { 'width': originalWidth, 'height': originalHeight, 'border-width': 2 }
                    }, { duration: 800, easing: 'ease-out-sine' });
                }
            });
        }, 700);

        return () => {
            if (jitterIntervalRef.current) {
                window.clearInterval(jitterIntervalRef.current);
                jitterIntervalRef.current = null;
            }
            if (pulseIntervalRef.current) {
                window.clearInterval(pulseIntervalRef.current);
                pulseIntervalRef.current = null;
            }
        };
    }, [prefersReducedMotion]);

    return (
        <div className={styles.container} id="graph-container" data-container="graph" ref={graphContainerRef}>
            <GraphControls
                isOverlay={hasOverlayInput}
                isFullscreen={isFullscreen}
                onRefresh={loadGraphData}
                onToggleFullscreen={toggleFullscreen}
                onFitToScreen={() => {
                    if (cyRef.current && cyRef.current.elements().length > 0) {
                        cyRef.current.fit();
                        cyRef.current.center();
                    }
                }}
                onResetZoom={() => {
                    if (cyRef.current) {
                        cyRef.current.zoom(0.8);
                        cyRef.current.center();
                    }
                }}
            />
            
            <div className={styles.content}>
                <StarField
                    className={styles.starfield}
                    width={graphContainerRef.current?.clientWidth}
                    height={graphContainerRef.current?.clientHeight}
                    starCount={200}
                    animate={!prefersReducedMotion}
                />
                <div ref={containerRef} className={styles.cytoscapeContainer}></div>
                
                <NodeDetailsPanel
                    selectedNode={selectedNode}
                    connectedMemories={connectedMemories}
                    onConnectedMemoryClick={handleConnectedMemoryClick}
                    onClose={hideNodeDetails}
                    onOpenDocumentMode={handleOpenDocumentMode}
                />
                
                {/* Document Mode View - Rendered separately from NodeDetailsPanel */}
                {isDocumentMode && documentModeNode && (
                    <DocumentModeView
                        selectedNode={documentModeNode}
                        connectedMemories={documentModeConnections}
                        onConnectedMemoryClick={handleConnectedMemoryClick}
                        onClose={handleCloseDocumentMode}
                    />
                )}
            </div>
        </div>
    );
});

GraphVisualization.displayName = 'GraphVisualization';

// Optimize with React.memo and custom comparison
export default memo(GraphVisualization, (prevProps, nextProps) => {
    // Only re-render if refresh trigger changes
    return prevProps.refreshTrigger === nextProps.refreshTrigger && 
           prevProps.hasOverlayInput === nextProps.hasOverlayInput;
});
