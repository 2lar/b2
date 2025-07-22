/**
 * Graph data for visualization
 */
export interface GraphData {
  nodes: GraphNode[];
  edges: GraphEdge[];
}

export interface GraphNode {
  id: string;
  label: string;
  content: string;
  createdAt: string;
}

export interface GraphEdge {
  id: string;
  source: string;
  target: string;
  strength: number;
  type: string;
}

import React, { useEffect, useRef, useState } from 'react';
import cytoscape, { Core, NodeCollection, NodeSingular } from 'cytoscape';
import cola from 'cytoscape-cola';
import { GraphData } from '@b2/shared';
import { throttle } from 'lodash-es';
import './GraphVisualization.css';

// Register the cola layout
cytoscape.use(cola);

interface GraphVisualizationProps {
  data: GraphData;
}

interface DisplayNode {
  id: string;
  content: string;
  label: string;
  createdAt: string;
}

// Color spectrum for the visualization - ordered like a rainbow/spectrum
const NODE_COLORS = [
  '#ffcb05', // yellow
  '#ff9500', // orange
  '#ff4800', // orange-red
  '#ff0000', // red
  '#d10f7c', // magenta
  '#9b00d9', // purple
  '#5215e8', // indigo
  '#2e58e8', // blue
  '#00c2dd', // cyan
  '#0dea88', // green
];

const GraphVisualization: React.FC<GraphVisualizationProps> = ({ data }) => {
  const containerRef = useRef<HTMLDivElement>(null);
  const cyRef = useRef<Core | null>(null);
  const [selectedNode, setSelectedNode] = useState<DisplayNode | null>(null);
  const [isSidebarPinned, setIsSidebarPinned] = useState<boolean>(false);
  const [hoveredNode, setHoveredNode] = useState<DisplayNode | null>(null);

  // Initialize cytoscape once on component mount
  useEffect(() => {
    console.log("GraphData received:", data);
    console.log("Nodes:", data.nodes.length, "Edges:", data.edges.length);
    
    if (!containerRef.current) {
      console.error("Container ref is null");
      return;
    }
    if (!containerRef.current) return;

    // Initialize cytoscape
    const cy = cytoscape({
      container: containerRef.current,
      elements: [],
      style: [
        {
          selector: 'node',
          style: {
            'background-color': 'data(color)',
            'width': 25, // Larger nodes as requested
            'height': 25, // Larger nodes as requested
            'border-width': 2,
            'border-color': 'data(borderColor)',
            'border-opacity': 1,
            'label': '', // No labels for cleaner look
            'transition-property': 'background-color, border-width, border-color, width, height, opacity',
            'transition-duration': 300
          }
        },
        {
          selector: 'edge',
          style: {
            'width': (ele: { data: (arg0: string) => number; }) => {
              // Scale width based on edge strength
              const strength = ele.data('strength') || 0.5;
              return Math.max(1.5, Math.min(strength * 7, 12));
            },
            'line-color': 'data(color)',
            'opacity': 0.8, // Fixed high opacity for all edges
            'curve-style': 'bezier',
            'transition-property': 'width, line-color, opacity',
            'transition-duration': 300,
            'target-arrow-shape': 'none', // No arrows for cleaner look
            'z-index': 0 // Edges below nodes
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
      layout: {
        name: 'grid' // Initial layout, will be replaced
      },
      // Set min/max zoom levels
      zoomingEnabled: true,
      userZoomingEnabled: true,
      minZoom: 0.1,
      maxZoom: 10,
      wheelSensitivity: 0.2,
      motionBlur: false // Disable motion blur for better performance
    });

    // Store reference to cytoscape instance
    cyRef.current = cy;

    preventViewportReset(cy);
    
    return () => {
      // Clean up only when component unmounts
      if (cyRef.current) {
        cyRef.current.destroy();
        cyRef.current = null;
      }
    };
  }, []); // Empty dependency array means this only runs once on mount

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
      if (arguments[2] === true || arguments[0]?.reset === true) {
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

  // Set up event handlers separately to avoid recreating on every state change
  useEffect(() => {
    if (!cyRef.current) return;
    console.log("Graph data received:", data);
    const cy = cyRef.current;
    
    const handleNodeTap = (event: cytoscape.EventObject) => {
      const node = event.target as NodeSingular;
      
      if (selectedNode && selectedNode.id === node.id()) {
        // If clicking the same node, toggle pin state
        setIsSidebarPinned(!isSidebarPinned);
      } else {
        // If clicking a new node, select it and pin sidebar
        setSelectedNode({
          id: node.id(),
          content: node.data('content'),
          label: node.data('label'),
          createdAt: node.data('createdAt')
        });
        setIsSidebarPinned(true);
      }
    };
    
    const handleNodeMouseOver = (event: cytoscape.EventObject) => {
      const node = event.target as NodeSingular;
      if (!isSidebarPinned) {
        setHoveredNode({
          id: node.id(),
          content: node.data('content'),
          label: node.data('label'),
          createdAt: node.data('createdAt')
        });
      }
      
      // Highlight connected edges
      const connectedEdges = node.connectedEdges();
      connectedEdges.addClass('highlighted');
    };
    
    const handleNodeMouseOut = () => {
      if (!isSidebarPinned) {
        setHoveredNode(null);
      }
      
      // Remove highlights
      cy.edges().removeClass('highlighted');
    };
    
    const handleBackgroundTap = (event: cytoscape.EventObject) => {
      // If clicking the background (not a node)
      if (event.target === cy) {
        // Unpin sidebar if it's pinned
        if (isSidebarPinned) {
          setIsSidebarPinned(false);
          setSelectedNode(null);
        }
      }
    };
    
    // Add event listeners
    cy.on('tap', 'node', handleNodeTap);
    cy.on('mouseover', 'node', handleNodeMouseOver);
    cy.on('mouseout', 'node', handleNodeMouseOut);
    cy.on('tap', handleBackgroundTap);
    
    // Clean up event listeners when component updates
    return () => {
      cy.removeListener('tap', 'node', handleNodeTap);
      cy.removeListener('mouseover', 'node', handleNodeMouseOver);
      cy.removeListener('mouseout', 'node', handleNodeMouseOut);
      cy.removeListener('tap', handleBackgroundTap);
    };
  }, [selectedNode, isSidebarPinned]); // Only re-run when these states change

  // Update elements when data changes
  useEffect(() => {
    if (!cyRef.current || !data) return;
    
    const cy = cyRef.current;
    
    // Check if we have actual data to show
    if (!data.nodes || !data.edges || data.nodes.length === 0) return;
    
    // Remove all existing elements
    cy.elements().remove();
  
    // First, assign node clusters based on connectivity
    const clusters = assignClusters(data);
    
    // Add nodes with color data
    data.nodes.forEach(node => {
      const clusterInfo = clusters.get(node.id) || { clusterId: node.id, clusterSize: 1, colorIndex: 0 };
      const color = NODE_COLORS[clusterInfo.colorIndex];
      
      cy.add({
        group: 'nodes',
        data: {
          id: node.id,
          label: node.label,
          content: node.content,
          createdAt: node.createdAt,
          color: color,
          borderColor: color,
          clusterId: clusterInfo.clusterId
        }
      });
    });
    
    // Add edges with color data
    data.edges.forEach(edge => {
      // Get source node's color
      const sourceNode = cy.getElementById(edge.source);
      const sourceColor = sourceNode.data('color');
      
      cy.add({
        group: 'edges',
        data: {
          id: edge.id,
          source: edge.source,
          target: edge.target,
          strength: edge.strength,
          color: sourceColor
        }
      });
    });
    
    // Apply cola layout with physics for interactive force-directed behavior
    const layout = cy.layout({
      name: 'cola',
      animate: true,
      refresh: 1,
      maxSimulationTime: 7000,
      nodeSpacing: function() { return 50; },
      edgeLength: function(edge: any) {
        // Stronger connections = shorter edges
        const strength = edge.data('strength') || 0.5;
        return 80 + (1 - strength) * 150;
      },
      // Group nodes by cluster
      alignment: function(node: any) {
        // Nodes in the same cluster tend to align together
        const clusterId = node.data('clusterId');
        if (clusterId) {
          // Create a predictable but varied position for each cluster
          const hash = clusterId.split('').reduce((acc: number, char: string) => acc + char.charCodeAt(0), 0);
          return {
            x: Math.cos(hash % 360 * Math.PI / 180),
            y: Math.sin(hash % 360 * Math.PI / 180)
          };
        }
        return { x: 0, y: 0 };
      },
      // *** Physics parameters - key for interactive feel ***
      gravity: 0.3,              // Lower = more spread out
      padding: 30,               // Padding around nodes
      avoidOverlap: true,        // Prevent node overlap
      randomize: false,          // Start with deterministic layout
      unconstrIter: 10,          // Iterations of unconstrained algorithm
      userConstIter: 15,         // Iterations of user-constrained algorithm
      allConstIter: 20,          // Iterations of all-constrained algorithm
      
      // *** Key physics parameters for dragging ***
      handleDisconnected: true,  // Handle disconnected nodes
      convergenceThreshold: 0.001, // When to stop the simulation
      flow: {                    // Use flow field effect for cleaner layouts
        enabled: true,          
        friction: 0.6           // Lower = more movement after dragging
      },
      infinite: true             // Keep physics simulation running - CRITICAL for interactive feel
    } as any);
    
    layout.run();
    
    // Add background effects
    addBackgroundEffects();
    
    // Add subtle animation to nodes
    animateNodes();
    
    // Setup grab and drag physics behavior
    setupDragBehavior(cy);
    
  }, [data]); // Only re-run when data changes
  
  // Function to assign clusters and colors to nodes based on connectivity
  const assignClusters = (graphData: GraphData) => {
    // Simplified clustering that focuses on strongest connections
    const nodeClusters = new Map();
    const clusterSizes = new Map();
    
    // Initialize each node in its own cluster
    graphData.nodes.forEach(node => {
      nodeClusters.set(node.id, { 
        clusterId: node.id, 
        colorIndex: 0 
      });
      clusterSizes.set(node.id, 1);
    });
    
    // Sort edges by strength (strongest first)
    const sortedEdges = [...graphData.edges].sort((a, b) => b.strength - a.strength);
    
    // Merge clusters based on strong connections
    sortedEdges.forEach(edge => {
      const sourceCluster = nodeClusters.get(edge.source).clusterId;
      const targetCluster = nodeClusters.get(edge.target).clusterId;
      
      // Skip if already in same cluster
      if (sourceCluster === targetCluster) return;
      
      // Only merge if connection is strong enough
      if (edge.strength < 0.2) return;
      
      // Determine which cluster to keep (the larger one)
      const sourceSize = clusterSizes.get(sourceCluster);
      const targetSize = clusterSizes.get(targetCluster);
      
      const keepCluster = sourceSize >= targetSize ? sourceCluster : targetCluster;
      const mergeCluster = sourceSize >= targetSize ? targetCluster : sourceCluster;
      
      // Update all nodes in the merged cluster
      graphData.nodes.forEach(node => {
        if (nodeClusters.get(node.id).clusterId === mergeCluster) {
          nodeClusters.set(node.id, { 
            clusterId: keepCluster, 
            colorIndex: nodeClusters.get(node.id).colorIndex 
          });
        }
      });
      
      // Update cluster size
      clusterSizes.set(keepCluster, sourceSize + targetSize);
      clusterSizes.delete(mergeCluster);
    });
    
    // Assign colors to clusters - using array methods instead of iterators
    const uniqueClusters = Array.from(new Set(
      Array.from(nodeClusters.values()).map(v => v.clusterId)
    ));
    const clusterColors = new Map();
    
    uniqueClusters.forEach((clusterId, index) => {
      clusterColors.set(clusterId, index % NODE_COLORS.length);
    });
    
    // Update color indices
    nodeClusters.forEach((value, nodeId) => {
      const colorIndex = clusterColors.get(value.clusterId);
      nodeClusters.set(nodeId, { ...value, colorIndex });
    });
    
    return nodeClusters;
  };
  
  // Helper to merge a node into a cluster
  const mergeIntoClusters = (
    graphData: GraphData,
    nodeClusters: Map<string, { clusterId: string, clusterSize: number, colorIndex: number }>,
    clusterSizes: Map<string, number>,
    nodeId: string,
    newClusterId: string
  ) => {
    // Skip if already in this cluster
    if (nodeClusters.get(nodeId)?.clusterId === newClusterId) return;
    
    const oldClusterId = nodeClusters.get(nodeId)?.clusterId || nodeId;
    
    // Update the node's cluster
    nodeClusters.set(nodeId, { 
      ...nodeClusters.get(nodeId)!,
      clusterId: newClusterId 
    });
    
    // Update cluster sizes
    const oldSize = clusterSizes.get(oldClusterId) || 1;
    const newSize = clusterSizes.get(newClusterId) || 1;
    
    clusterSizes.set(oldClusterId, oldSize - 1);
    clusterSizes.set(newClusterId, newSize + 1);
    
    // Recursively merge connected nodes
    graphData.edges.forEach(edge => {
      if (edge.source === nodeId && nodeClusters.get(edge.target)?.clusterId === oldClusterId) {
        mergeIntoClusters(graphData, nodeClusters, clusterSizes, edge.target, newClusterId);
      } else if (edge.target === nodeId && nodeClusters.get(edge.source)?.clusterId === oldClusterId) {
        mergeIntoClusters(graphData, nodeClusters, clusterSizes, edge.source, newClusterId);
      }
    });
  };
  
  // Create background star effect
  const addBackgroundEffects = () => {
    if (!containerRef.current) return;
    
    // Clear any existing canvas
    const existingCanvas = containerRef.current.querySelector('.star-background');
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
    
    containerRef.current.appendChild(canvas);
    
    // Set canvas size
    canvas.width = containerRef.current.clientWidth;
    canvas.height = containerRef.current.clientHeight;
    
    const ctx = canvas.getContext('2d');
    if (!ctx) return;
    
    // Create stars
    const stars: {x: number, y: number, size: number, opacity: number}[] = [];
    
    for (let i = 0; i < 150; i++) {
      stars.push({
        x: Math.random() * canvas.width,
        y: Math.random() * canvas.height,
        size: Math.random() * 1.5,
        opacity: Math.random() * 0.8 + 0.2
      });
    }
    
    // Animation loop
    const animate = () => {
      if (!ctx) return;
      
      ctx.clearRect(0, 0, canvas.width, canvas.height);
      
      // Draw stars
      stars.forEach(star => {
        ctx.beginPath();
        ctx.arc(star.x, star.y, star.size, 0, Math.PI * 2);
        ctx.fillStyle = `rgba(255, 255, 255, ${star.opacity * (0.5 + Math.sin(Date.now() / 1000) * 0.5)})`;
        ctx.fill();
        
        // Slowly move stars
        star.y -= 0.05;
        
        // Reset stars that go off screen
        if (star.y < 0) {
          star.y = canvas.height;
          star.x = Math.random() * canvas.width;
        }
      });
      
      requestAnimationFrame(animate);
    };
    
    animate();
  };

  const setupDragBehavior = (cy: Core) => {
    let draggedNode: NodeSingular | null = null;
    let connectedNodes: cytoscape.NodeCollection = cy.collection();
    
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
    
    cy.on('drag', 'node', throttle(function(e) {
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
     })
    }, 30));
    
    cy.on('free', 'node', function(e) {
      if (!draggedNode) return;
      
      // Reset styles
      draggedNode.style({
        'border-width': 2,
        'border-color': draggedNode.data('borderColor')
      });
      
      // Reset variables
      draggedNode = null;
      connectedNodes = cy.collection();
    });
  };


  // Animate nodes with subtle movement
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
        
        // Add a subtle pulse effect
        randomNode.animate({
          style: { 
            'width': randomNode.width() * 1.1, 
            'height': randomNode.height() * 1.1,
            'border-width': 4
          }
        }, {
          duration: 800,
          easing: 'ease-in-sine',
          complete: function() {
            randomNode.animate({
              style: { 
                'width': randomNode.width() / 1.1, 
                'height': randomNode.height() / 1.1,
                'border-width': 2
              }
            }, {
              duration: 800,
              easing: 'ease-out-sine'
            });
          }
        });
      }
    }, 500);
  };

  // Subtle animation for nodes
  const animateNode = (node: any) => {
    if (!cyRef.current || !node) return;
    
    // Don't animate if node is being dragged or is locked
    if (node.grabbed() || node.locked()) return;
    
    // Original position
    const originalPos = node.position();
    
    // Small random movement (different for each node)
    const offsetX = (Math.random() - 0.5) * 15;
    const offsetY = (Math.random() - 0.5) * 15;
    const duration = 2000 + Math.random() * 4000; // Random duration for more natural movement
  
    // First animation - move away from original position
    node.animate({
      position: { x: originalPos.x + offsetX, y: originalPos.y + offsetY },
      style: { opacity: 1 }
    }, {
      duration: duration / 2,
      easing: 'ease-in-out-sine',
      complete: function() {
        // Second animation - move back toward original position but not exactly
        // This creates a perpetual "floating" effect
        const returnOffsetX = (Math.random() - 0.5) * 5;
        const returnOffsetY = (Math.random() - 0.5) * 5;
        
        node.animate({
          position: { x: originalPos.x + returnOffsetX, y: originalPos.y + returnOffsetY }
        }, {
          duration: duration / 2,
          easing: 'ease-in-out-sine',
          complete: function() {
            // Continue animation with a short delay
            setTimeout(() => {
              // Recursive call ensures animation continues indefinitely
              animateNode(node);
            }, 100 + Math.random() * 500);
          }
        });
      }
    });
  };

  // Format date for display
  const formatDate = (dateString: string): string => {
    if (!dateString) return '';
    
    const options: Intl.DateTimeFormatOptions = { 
      year: 'numeric', 
      month: 'short', 
      day: 'numeric',
      hour: '2-digit', 
      minute: '2-digit'
    };
    
    try {
      const date = new Date(dateString);
      return date.toLocaleDateString(undefined, options);
    } catch (e) {
      return dateString;
    }
  };
  

  // Update reset button to use this flag
  const handleResetView = () => {
    if (cyRef.current) {
      // Pass a flag in a way TypeScript accepts
      const options = { reset: true };
      cyRef.current.fit(options as any, 50);
    }
  };
  

  // Determine if sidebar should be visible
  const displayNode = selectedNode || (!isSidebarPinned && hoveredNode);
  const sidebarClass = `graph-sidebar ${displayNode ? 'visible' : ''} ${isSidebarPinned ? 'pinned' : ''}`;

  return (
    <div className="graph-container">
      <div className="graph-tools">
        <button className="reset-view-button" onClick={handleResetView}>
          Reset View
        </button>
      </div>
      
      <div ref={containerRef} className="graph-visualization"></div>
      
      <div className={sidebarClass}>
        {displayNode && (
          <>
            <div className="sidebar-header">
              <h3>{displayNode.label}</h3>
              {isSidebarPinned && (
                <button 
                  className="unpin-button"
                  onClick={() => {
                    setIsSidebarPinned(false);
                    setSelectedNode(null);
                  }}
                  aria-label="Close sidebar"
                >
                  ×
                </button>
              )}
            </div>
            <div className="sidebar-date">{formatDate(displayNode.createdAt)}</div>
            <div className="sidebar-content">{displayNode.content}</div>
            <div className="sidebar-footer">
              {!isSidebarPinned && (
                <div className="sidebar-instruction">Click to pin this note</div>
              )}
            </div>
          </>
        )}
      </div>
    </div>
  );
};

export default GraphVisualization;