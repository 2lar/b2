import React, { useEffect, useRef, useState, useImperativeHandle, forwardRef, memo, useCallback } from 'react';
import Sigma from 'sigma';
import Graph from 'graphology';
import FA2Layout from 'graphology-layout-forceatlas2/worker';
import { nodesApi } from '../api/nodes';
import type { NodeDetails } from '../../../services';
import { useFullscreen } from '../../../common/hooks/useFullscreen';
import { buildGraph, getNodesWithinHops } from '../graph';
import type { NodeAttributes, EdgeAttributes, GraphApiData } from '../graph';
import GraphControls from './GraphControls';
import NodeDetailsPanel from './NodeDetailsPanel';
import DocumentModeView from './NodeDetailsPanel/DocumentModeView';
import StarField from './StarField';
import styles from './GraphVisualization.module.css';

interface GraphVisualizationProps {
    refreshTrigger: number;
    hasOverlayInput?: boolean;
}

export interface GraphVisualizationRef {
    selectAndCenterNode: (nodeId: string) => boolean;
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

const DIMMED_NODE_COLOR = 'rgba(100, 100, 100, 0.15)';
const DIMMED_EDGE_COLOR = 'rgba(100, 100, 100, 0.08)';

const GraphVisualization = forwardRef<GraphVisualizationRef, GraphVisualizationProps>(({ refreshTrigger, hasOverlayInput = false }, ref) => {
    const containerRef = useRef<HTMLDivElement>(null);
    const graphContainerRef = useRef<HTMLDivElement>(null);
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    const rendererRef = useRef<any>(null);
    const graphRef = useRef<Graph<NodeAttributes, EdgeAttributes> | null>(null);
    const fa2Ref = useRef<FA2Layout | null>(null);
    const [selectedNode, setSelectedNode] = useState<DisplayNode | null>(null);
    const [connectedMemories, setConnectedMemories] = useState<ConnectedMemory[]>([]);
    const [prefersReducedMotion, setPrefersReducedMotion] = useState(false);

    // Track selected node ID for reducers
    const selectedNodeIdRef = useRef<string | null>(null);
    const selectedNeighborsRef = useRef<Set<string>>(new Set());
    // Depth filter state
    const [depthLevel, setDepthLevel] = useState<number | null>(null);
    const depthVisibleRef = useRef<Set<string> | null>(null);

    // Drag state
    const isDraggingRef = useRef(false);
    const draggedNodeRef = useRef<string | null>(null);

    // Animation refs
    const jitterIntervalRef = useRef<number | null>(null);
    const pulseIntervalRef = useRef<number | null>(null);

    // Document mode state
    const [isDocumentMode, setIsDocumentMode] = useState(false);
    const [documentModeNode, setDocumentModeNode] = useState<DisplayNode | null>(null);
    const [documentModeConnections, setDocumentModeConnections] = useState<ConnectedMemory[]>([]);

    // Fullscreen
    const { isFullscreen, toggleFullscreen } = useFullscreen(graphContainerRef);

    const hideNodeDetails = useCallback((): void => {
        setSelectedNode(null);
        setConnectedMemories([]);
        selectedNodeIdRef.current = null;
        selectedNeighborsRef.current = new Set();
        setDepthLevel(null);
        depthVisibleRef.current = null;
        rendererRef.current?.refresh();
    }, []);

    const highlightNode = useCallback((nodeId: string) => {
        const graph = graphRef.current;
        if (!graph || !graph.hasNode(nodeId)) return;

        selectedNodeIdRef.current = nodeId;
        const neighbors = new Set<string>();
        graph.forEachNeighbor(nodeId, (neighbor) => neighbors.add(neighbor));
        selectedNeighborsRef.current = neighbors;
        rendererRef.current?.refresh();
    }, []);

    const showNodeDetails = useCallback(async (nodeId: string): Promise<void> => {
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

            if (nodeData.edges && nodeData.edges.length > 0 && graphRef.current) {
                const graph = graphRef.current;
                const connectedNodesInfo = nodeData.edges.map(connectedNodeId => {
                    if (graph.hasNode(connectedNodeId)) {
                        return {
                            id: connectedNodeId,
                            label: graph.getNodeAttribute(connectedNodeId, 'label') || 'Untitled'
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
    }, []);

    useImperativeHandle(ref, () => ({
        selectAndCenterNode: (nodeId: string) => {
            const graph = graphRef.current;
            const renderer = rendererRef.current;
            if (!graph || !renderer || !graph.hasNode(nodeId)) return false;

            const x = graph.getNodeAttribute(nodeId, 'x');
            const y = graph.getNodeAttribute(nodeId, 'y');

            renderer.getCamera().animate({ x, y, ratio: 0.3 }, { duration: 500 });
            highlightNode(nodeId);
            showNodeDetails(nodeId);
            return true;
        },
        hideNodeDetails
    }), [hideNodeDetails, highlightNode, showNodeDetails]);

    // Reduced motion preference
    useEffect(() => {
        if (typeof window === 'undefined' || typeof window.matchMedia !== 'function') return;
        const mql = window.matchMedia('(prefers-reduced-motion: reduce)');
        const handler = () => setPrefersReducedMotion(mql.matches);
        handler();
        mql.addEventListener('change', handler);
        return () => mql.removeEventListener('change', handler);
    }, []);

    // Initialize Sigma renderer
    useEffect(() => {
        if (!containerRef.current) return;

        const graph = new Graph<NodeAttributes, EdgeAttributes>();
        graphRef.current = graph;

        const renderer = new Sigma(graph, containerRef.current, {
            renderLabels: false,
            minCameraRatio: 0.1,
            maxCameraRatio: 10,
            labelRenderedSizeThreshold: 999, // effectively hide labels
            nodeReducer: (node, data) => {
                const res: Record<string, any> = { ...data };

                // Depth filter
                if (depthVisibleRef.current && !depthVisibleRef.current.has(node)) {
                    res.hidden = true;
                    return res;
                }

                // Selection highlighting
                if (selectedNodeIdRef.current) {
                    if (node === selectedNodeIdRef.current) {
                        res.size = (data.size || 8) * 1.4;
                        res.zIndex = 2;
                    } else if (selectedNeighborsRef.current.has(node)) {
                        res.zIndex = 1;
                    } else {
                        res.color = DIMMED_NODE_COLOR;
                        res.zIndex = 0;
                    }
                }

                return res;
            },
            edgeReducer: (edge, data) => {
                const res: Record<string, any> = { ...data };

                // Depth filter — hide edges whose endpoints are hidden
                if (depthVisibleRef.current) {
                    const [source, target] = graph.extremities(edge);
                    if (!depthVisibleRef.current.has(source) || !depthVisibleRef.current.has(target)) {
                        res.hidden = true;
                        return res;
                    }
                }

                // Selection highlighting
                if (selectedNodeIdRef.current) {
                    const [source, target] = graph.extremities(edge);
                    const sel = selectedNodeIdRef.current;
                    if (source !== sel && target !== sel) {
                        res.color = DIMMED_EDGE_COLOR;
                    }
                }

                return res;
            }
        });

        rendererRef.current = renderer;

        // --- Event handlers ---

        // Click node
        renderer.on('clickNode', ({ node }) => {
            highlightNode(node);
            showNodeDetails(node);
        });

        // Click stage (background)
        renderer.on('clickStage', () => {
            hideNodeDetails();
        });

        // Drag: downNode + mousemovebody + mouseup
        renderer.on('downNode', (e) => {
            isDraggingRef.current = true;
            draggedNodeRef.current = e.node;
            graph.setNodeAttribute(e.node, 'fixed', true);
            // Disable camera panning during drag
            renderer.getCamera().disable();
        });

        renderer.getMouseCaptor().on('mousemovebody', (e) => {
            if (!isDraggingRef.current || !draggedNodeRef.current) return;

            // Convert viewport coords to graph coords
            const pos = renderer.viewportToGraph(e);
            graph.setNodeAttribute(draggedNodeRef.current, 'x', pos.x);
            graph.setNodeAttribute(draggedNodeRef.current, 'y', pos.y);

            // Pull connected nodes with diminishing effect
            graph.forEachNeighbor(draggedNodeRef.current, (neighbor) => {
                const nx = graph.getNodeAttribute(neighbor, 'x');
                const ny = graph.getNodeAttribute(neighbor, 'y');
                const pull = 0.01;
                graph.setNodeAttribute(neighbor, 'x', nx + (pos.x - nx) * pull);
                graph.setNodeAttribute(neighbor, 'y', ny + (pos.y - ny) * pull);
            });
        });

        renderer.getMouseCaptor().on('mouseup', () => {
            if (draggedNodeRef.current) {
                graph.setNodeAttribute(draggedNodeRef.current, 'fixed', false);
            }
            isDraggingRef.current = false;
            draggedNodeRef.current = null;
            renderer.getCamera().enable();
        });

        return () => {
            // Kill FA2 if running
            if (fa2Ref.current) {
                fa2Ref.current.kill();
                fa2Ref.current = null;
            }
            // Kill renderer
            if (rendererRef.current) {
                rendererRef.current.kill();
                rendererRef.current = null;
            }
            graphRef.current = null;
            // Clear intervals
            if (jitterIntervalRef.current) {
                window.clearInterval(jitterIntervalRef.current);
                jitterIntervalRef.current = null;
            }
            if (pulseIntervalRef.current) {
                window.clearInterval(pulseIntervalRef.current);
                pulseIntervalRef.current = null;
            }
        };
    }, []);

    const loadGraphData = useCallback(async () => {
        const graph = graphRef.current;
        const renderer = rendererRef.current;
        if (!graph || !renderer) return;

        try {
            const apiData = await nodesApi.getGraphData() as GraphApiData;

            // Stop existing FA2
            if (fa2Ref.current) {
                fa2Ref.current.kill();
                fa2Ref.current = null;
            }

            // Clear and rebuild
            graph.clear();
            const newGraph = buildGraph(apiData);

            // Merge into our live graph
            newGraph.forEachNode((node, attrs) => {
                graph.addNode(node, attrs);
            });
            newGraph.forEachEdge((edge, attrs, source, target) => {
                graph.addEdgeWithKey(edge, source, target, attrs);
            });

            if (graph.order > 0) {
                // Start ForceAtlas2 layout in a web worker
                const fa2 = new FA2Layout(graph, {
                    settings: {
                        gravity: 0.3,
                        barnesHutOptimize: true,
                        adjustSizes: true,
                        slowDown: 2,
                        scalingRatio: 4,
                    }
                });
                fa2Ref.current = fa2;
                fa2.start();

                // Auto-stop after 7 seconds
                setTimeout(() => {
                    if (fa2Ref.current === fa2 && fa2.isRunning()) {
                        fa2.stop();
                    }
                }, 7000);
            }

            // Reset selection state
            selectedNodeIdRef.current = null;
            selectedNeighborsRef.current = new Set();
            depthVisibleRef.current = null;
            setDepthLevel(null);
            setSelectedNode(null);
            setConnectedMemories([]);

            renderer.refresh();
        } catch (error) {
            console.error('Error loading graph data:', error);
        }
    }, []);

    // Load graph data when refreshTrigger changes
    useEffect(() => {
        loadGraphData();
    }, [refreshTrigger, loadGraphData]);

    // Subtle animations (jitter + pulse)
    useEffect(() => {
        if (!graphRef.current || prefersReducedMotion) return;

        const graph = graphRef.current;

        jitterIntervalRef.current = window.setInterval(() => {
            if (!graphRef.current || document.visibilityState !== 'visible') return;
            if (isDraggingRef.current) return;
            if (fa2Ref.current?.isRunning()) return;

            graph.forEachNode((node) => {
                if (Math.random() > 0.7) {
                    const jx = (Math.random() - 0.5) * 0.5;
                    const jy = (Math.random() - 0.5) * 0.5;
                    graph.setNodeAttribute(node, 'x', graph.getNodeAttribute(node, 'x') + jx);
                    graph.setNodeAttribute(node, 'y', graph.getNodeAttribute(node, 'y') + jy);
                }
            });
        }, 2000);

        pulseIntervalRef.current = window.setInterval(() => {
            if (!graphRef.current || document.visibilityState !== 'visible') return;
            if (isDraggingRef.current) return;
            if (fa2Ref.current?.isRunning()) return;

            const nodes = graph.nodes();
            if (nodes.length === 0) return;
            const nodeId = nodes[Math.floor(Math.random() * nodes.length)];
            if (nodeId === selectedNodeIdRef.current) return;

            const origSize = graph.getNodeAttribute(nodeId, 'size');
            graph.setNodeAttribute(nodeId, 'size', origSize * 1.3);
            setTimeout(() => {
                if (graphRef.current?.hasNode(nodeId)) {
                    graph.setNodeAttribute(nodeId, 'size', origSize);
                }
            }, 700);
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

    // Handle fullscreen resize
    useEffect(() => {
        if (!rendererRef.current) return;
        const timer = setTimeout(() => {
            rendererRef.current?.refresh();
        }, isFullscreen ? 100 : 300);
        return () => clearTimeout(timer);
    }, [isFullscreen]);

    // Depth filter handler
    const handleDepthFilter = useCallback((hops: number | null) => {
        setDepthLevel(hops);
        if (!hops || !selectedNodeIdRef.current || !graphRef.current) {
            depthVisibleRef.current = null;
        } else {
            depthVisibleRef.current = getNodesWithinHops(graphRef.current, selectedNodeIdRef.current, hops);
        }
        rendererRef.current?.refresh();
    }, []);

    // Document mode handlers
    const handleOpenDocumentMode = useCallback((node: DisplayNode, connections: ConnectedMemory[]): void => {
        setDocumentModeNode(node);
        setDocumentModeConnections(connections);
        setIsDocumentMode(true);
        hideNodeDetails();
    }, [hideNodeDetails]);

    const handleCloseDocumentMode = useCallback((): void => {
        setIsDocumentMode(false);
        setDocumentModeNode(null);
        setDocumentModeConnections([]);
    }, []);

    const handleConnectedMemoryClick = useCallback((memoryId: string): void => {
        const graph = graphRef.current;
        const renderer = rendererRef.current;
        if (!graph || !renderer || !graph.hasNode(memoryId)) return;

        const x = graph.getNodeAttribute(memoryId, 'x');
        const y = graph.getNodeAttribute(memoryId, 'y');
        renderer.getCamera().animate({ x, y, ratio: 0.3 }, { duration: 300 });

        highlightNode(memoryId);
        showNodeDetails(memoryId);
    }, [highlightNode, showNodeDetails]);

    return (
        <div className={styles.container} id="graph-container" data-container="graph" ref={graphContainerRef}>
            <GraphControls
                isOverlay={hasOverlayInput}
                isFullscreen={isFullscreen}
                onRefresh={loadGraphData}
                onToggleFullscreen={toggleFullscreen}
                onFitToScreen={() => {
                    rendererRef.current?.getCamera().animatedReset({ duration: 300 });
                }}
                onResetZoom={() => {
                    rendererRef.current?.getCamera().animate({ ratio: 1.25 }, { duration: 300 });
                }}
                depthLevel={depthLevel}
                onDepthFilter={selectedNodeIdRef.current ? handleDepthFilter : undefined}
            />

            <div className={styles.content}>
                <StarField
                    className={styles.starfield}
                    width={graphContainerRef.current?.clientWidth}
                    height={graphContainerRef.current?.clientHeight}
                    starCount={200}
                    animate={!prefersReducedMotion}
                />
                <div ref={containerRef} className={styles.sigmaContainer}></div>

                <NodeDetailsPanel
                    selectedNode={selectedNode}
                    connectedMemories={connectedMemories}
                    onConnectedMemoryClick={handleConnectedMemoryClick}
                    onClose={hideNodeDetails}
                    onOpenDocumentMode={handleOpenDocumentMode}
                />

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

export default memo(GraphVisualization, (prevProps, nextProps) => {
    return prevProps.refreshTrigger === nextProps.refreshTrigger &&
           prevProps.hasOverlayInput === nextProps.hasOverlayInput;
});
