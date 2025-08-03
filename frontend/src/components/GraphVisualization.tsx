import React, { useEffect, useRef, useState, useImperativeHandle, forwardRef } from 'react';
import cytoscape, { Core, ElementDefinition, NodeSingular, NodeCollection } from 'cytoscape';
import cola from 'cytoscape-cola';
import { api, type NodeDetails } from '../services';
import { useFullscreen } from '../hooks/useFullscreen';
import { throttle } from 'lodash-es';

// Register the cola layout
cytoscape.use(cola);

interface GraphVisualizationProps {
    refreshTrigger: number;
}

export interface GraphVisualizationRef {
    selectAndCenterNode: (nodeId: string) => boolean;
}

interface DisplayNode {
    id: string;
    content: string;
    label: string;
    timestamp: string;
    tags?: string[];
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
];

const GraphVisualization = forwardRef<GraphVisualizationRef, GraphVisualizationProps>(({ refreshTrigger }, ref) => {
    const containerRef = useRef<HTMLDivElement>(null);
    const graphContainerRef = useRef<HTMLDivElement>(null);
    const cyRef = useRef<Core | null>(null);
    const [selectedNode, setSelectedNode] = useState<DisplayNode | null>(null);
    
    // Fullscreen functionality
    const { isFullscreen, toggleFullscreen } = useFullscreen(graphContainerRef);

    // Expose methods to parent component via ref
    useImperativeHandle(ref, () => ({
        selectAndCenterNode: (nodeId: string) => {
            if (!cyRef.current) return false;
            
            const cy = cyRef.current;
            const node = cy.getElementById(nodeId);
            
            if (node.length === 0) {
                console.warn(`Node with ID ${nodeId} not found in graph`);
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
        }
    }), []);

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
                        'label': '', // No labels for cleaner look
                        'shadow-blur': 8,
                        'shadow-color': 'data(color)',
                        'shadow-opacity': 0.6,
                        'shadow-offset-x': 0,
                        'shadow-offset-y': 0,
                        'transition-property': 'background-color, border-width, border-color, width, height, opacity, shadow-blur',
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
                        'line-color': 'data(color)',
                        'opacity': 0.7,
                        'curve-style': 'bezier',
                        'shadow-blur': 4,
                        'shadow-color': 'data(color)',
                        'shadow-opacity': 0.4,
                        'shadow-offset-x': 0,
                        'shadow-offset-y': 0,
                        'transition-property': 'width, line-color, opacity, shadow-blur',
                        'transition-duration': 300,
                        'target-arrow-shape': 'none',
                        'z-index': 0
                    }
                },
                {
                    selector: 'node:selected',
                    style: {
                        'border-width': 4,
                        'border-color': '#ffffff',
                        'width': 35,
                        'height': 35,
                        'shadow-blur': 15,
                        'shadow-color': '#ffffff',
                        'shadow-opacity': 0.8,
                        'z-index': 999
                    }
                },
                {
                    selector: 'edge:selected',
                    style: {
                        'width': 5,
                        'opacity': 1,
                        'shadow-blur': 8,
                        'shadow-color': 'data(color)',
                        'shadow-opacity': 0.8,
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

        // Set up event handlers
        setupEventHandlers(cy);

        // Store global reference for legacy compatibility
        (window as any).cy = cy;

        // Prevent unwanted viewport resets
        preventViewportReset(cy);

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
        setInterval(() => {
            // Don't restore during user interactions
            if (cy.nodes().filter(':grabbed').length > 0) return;
            
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
        cy.on('wheelzoom', (event) => {
            // Ensure zoom stays within bounds
            const zoom = cy.zoom();
            if (zoom <= cy.minZoom() || zoom >= cy.maxZoom()) {
                event.preventDefault();
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
                timestamp: nodeData.timestamp || '',
                tags: nodeData.tags || []
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

            // Apply cola layout with advanced physics parameters
            const layout = cy.layout({
                name: 'cola',
                animate: true,
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
                infinite: true, // Keep physics simulation running - CRITICAL for interactive feel
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
            
            // Add continuous node animations
            animateNodes();

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

    const setupDragBehavior = (cy: Core) => {
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
    };

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

    const addBackgroundEffects = () => {
        if (!graphContainerRef.current) return;
        
        // Clear any existing canvas
        const existingCanvas = graphContainerRef.current.querySelector('.star-background');
        if (existingCanvas) {
            existingCanvas.remove();
        }
        
        // Create canvas
        const canvas = document.createElement('canvas');
        canvas.setAttribute('class', 'star-background');
        canvas.style.position = 'absolute';
        canvas.style.top = '0';
        canvas.style.left = '0';
        canvas.style.width = '100%';
        canvas.style.height = '100%';
        canvas.style.pointerEvents = 'none';
        canvas.style.zIndex = '0';
        
        graphContainerRef.current.appendChild(canvas);
        
        // Set canvas size
        canvas.width = graphContainerRef.current.clientWidth;
        canvas.height = graphContainerRef.current.clientHeight;
        
        const ctx = canvas.getContext('2d');
        if (!ctx) return;
        
        // Create enhanced cosmic stars with varied sizes
        const stars: {x: number, y: number, size: number, opacity: number, twinkleSpeed: number, type: 'normal' | 'bright' | 'distant'}[] = [];
        
        for (let i = 0; i < 200; i++) {
            const starType = Math.random();
            let size, opacity, twinkleSpeed, type: 'normal' | 'bright' | 'distant';
            
            if (starType < 0.1) {
                // Bright stars (10%)
                size = Math.random() * 2 + 1.5;
                opacity = Math.random() * 0.4 + 0.6;
                twinkleSpeed = Math.random() * 2000 + 1000;
                type = 'bright';
            } else if (starType < 0.7) {
                // Normal stars (60%)
                size = Math.random() * 1 + 0.5;
                opacity = Math.random() * 0.6 + 0.3;
                twinkleSpeed = Math.random() * 3000 + 2000;
                type = 'normal';
            } else {
                // Distant stars (30%)
                size = Math.random() * 0.5 + 0.2;
                opacity = Math.random() * 0.4 + 0.1;
                twinkleSpeed = Math.random() * 4000 + 3000;
                type = 'distant';
            }
            
            stars.push({
                x: Math.random() * canvas.width,
                y: Math.random() * canvas.height,
                size,
                opacity,
                twinkleSpeed,
                type
            });
        }
        
        // Animation loop
        const animate = () => {
            if (!ctx) return;
            
            ctx.clearRect(0, 0, canvas.width, canvas.height);
            
            // Draw enhanced cosmic stars
            stars.forEach(star => {
                ctx.beginPath();
                
                // Enhanced twinkling effect based on star type
                const time = Date.now();
                const twinkle = Math.sin(time / star.twinkleSpeed) * 0.5 + 0.5;
                const currentOpacity = star.opacity * (0.3 + twinkle * 0.7);
                
                // Different colors for different star types
                let color;
                switch (star.type) {
                    case 'bright':
                        color = `rgba(255, 255, 255, ${currentOpacity})`;
                        // Add subtle glow for bright stars
                        ctx.shadowColor = 'rgba(255, 255, 255, 0.8)';
                        ctx.shadowBlur = star.size * 2;
                        break;
                    case 'normal':
                        color = `rgba(200, 220, 255, ${currentOpacity})`;
                        ctx.shadowColor = 'rgba(200, 220, 255, 0.3)';
                        ctx.shadowBlur = star.size;
                        break;
                    case 'distant':
                        color = `rgba(150, 150, 200, ${currentOpacity})`;
                        ctx.shadowColor = 'transparent';
                        ctx.shadowBlur = 0;
                        break;
                }
                
                ctx.arc(star.x, star.y, star.size, 0, Math.PI * 2);
                ctx.fillStyle = color;
                ctx.fill();
                
                // Reset shadow for next star
                ctx.shadowColor = 'transparent';
                ctx.shadowBlur = 0;
                
                // Slowly move stars (slower for distant stars)
                const moveSpeed = star.type === 'distant' ? 0.02 : star.type === 'bright' ? 0.08 : 0.05;
                star.y -= moveSpeed;
                
                // Reset stars that go off screen
                if (star.y < -star.size) {
                    star.y = canvas.height + star.size;
                    star.x = Math.random() * canvas.width;
                }
            });
            
            requestAnimationFrame(animate);
        };
        
        animate();
    };

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
});

GraphVisualization.displayName = 'GraphVisualization';

export default GraphVisualization;