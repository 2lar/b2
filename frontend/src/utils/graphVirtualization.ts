// Graph virtualization utilities for handling large node counts efficiently
import { useState, useCallback } from 'react';

interface BoundingBox {
  x: number;
  y: number;
  width: number;
  height: number;
}

interface ViewportInfo {
  center: { x: number; y: number };
  zoom: number;
  boundingBox: BoundingBox;
}

interface NodePosition {
  id: string;
  x: number;
  y: number;
  visible: boolean;
}

interface VirtualizationConfig {
  enabled: boolean;
  threshold: number; // Number of nodes above which virtualization kicks in
  bufferSize: number; // Extra nodes to render outside viewport for smooth scrolling
  lodEnabled: boolean; // Level of detail rendering
  maxRenderDistance: number; // Maximum distance from viewport center to render
}

class GraphVirtualizer {
  private config: VirtualizationConfig;
  private nodePositions: Map<string, NodePosition>;
  private viewport: ViewportInfo;
  private visibleNodes: Set<string>;
  private renderQueue: string[];

  constructor(config: Partial<VirtualizationConfig> = {}) {
    this.config = {
      enabled: true,
      threshold: 500,
      bufferSize: 100,
      lodEnabled: true,
      maxRenderDistance: 2000,
      ...config,
    };
    
    this.nodePositions = new Map();
    this.viewport = {
      center: { x: 0, y: 0 },
      zoom: 1,
      boundingBox: { x: 0, y: 0, width: 1000, height: 1000 },
    };
    this.visibleNodes = new Set();
    this.renderQueue = [];
  }

  // Update viewport information
  updateViewport(viewport: ViewportInfo): void {
    this.viewport = { ...viewport };
    this.updateVisibleNodes();
  }

  // Update node positions
  updateNodePositions(positions: Map<string, { x: number; y: number }>): void {
    positions.forEach((pos, id) => {
      this.nodePositions.set(id, {
        id,
        x: pos.x,
        y: pos.y,
        visible: false, // Will be calculated in updateVisibleNodes
      });
    });
    this.updateVisibleNodes();
  }

  // Calculate which nodes should be visible based on viewport
  private updateVisibleNodes(): void {
    const totalNodes = this.nodePositions.size;
    
    // Skip virtualization if below threshold
    if (!this.config.enabled || totalNodes < this.config.threshold) {
      this.visibleNodes = new Set(this.nodePositions.keys());
      this.renderQueue = Array.from(this.visibleNodes);
      return;
    }

    const newVisibleNodes = new Set<string>();
    const { center, zoom, boundingBox } = this.viewport;
    
    // Calculate viewport bounds with buffer
    const viewportBuffer = this.config.bufferSize / zoom;
    const leftBound = center.x - (boundingBox.width / (2 * zoom)) - viewportBuffer;
    const rightBound = center.x + (boundingBox.width / (2 * zoom)) + viewportBuffer;
    const topBound = center.y - (boundingBox.height / (2 * zoom)) - viewportBuffer;
    const bottomBound = center.y + (boundingBox.height / (2 * zoom)) + viewportBuffer;

    // Check each node position
    this.nodePositions.forEach((nodePos, id) => {
      const { x, y } = nodePos;
      
      // Basic viewport culling
      const inViewport = x >= leftBound && x <= rightBound && y >= topBound && y <= bottomBound;
      
      // Distance-based culling for very far nodes
      const distance = Math.sqrt(
        Math.pow(x - center.x, 2) + Math.pow(y - center.y, 2)
      );
      const withinMaxDistance = distance <= this.config.maxRenderDistance / zoom;
      
      if (inViewport || withinMaxDistance) {
        newVisibleNodes.add(id);
        this.nodePositions.set(id, { ...nodePos, visible: true });
      } else {
        this.nodePositions.set(id, { ...nodePos, visible: false });
      }
    });

    this.visibleNodes = newVisibleNodes;
    this.updateRenderQueue();
  }

  // Update render queue with priority based on distance from viewport center
  private updateRenderQueue(): void {
    const nodesWithDistance = Array.from(this.visibleNodes)
      .map(id => {
        const pos = this.nodePositions.get(id);
        if (!pos) return null;
        
        const distance = Math.sqrt(
          Math.pow(pos.x - this.viewport.center.x, 2) + 
          Math.pow(pos.y - this.viewport.center.y, 2)
        );
        
        return { id, distance, position: pos };
      })
      .filter(Boolean)
      .sort((a, b) => a!.distance - b!.distance);

    this.renderQueue = nodesWithDistance.map(node => node!.id);
  }

  // Get nodes that should be rendered
  getVisibleNodes(): Set<string> {
    return new Set(this.visibleNodes);
  }

  // Get render queue (ordered by priority)
  getRenderQueue(): string[] {
    return [...this.renderQueue];
  }

  // Check if virtualization is active
  isVirtualizationActive(): boolean {
    return this.config.enabled && this.nodePositions.size >= this.config.threshold;
  }

  // Get level of detail for a node based on distance and zoom
  getNodeLOD(nodeId: string): 'high' | 'medium' | 'low' | 'minimal' {
    if (!this.config.lodEnabled) return 'high';
    
    const nodePos = this.nodePositions.get(nodeId);
    if (!nodePos) return 'minimal';

    const distance = Math.sqrt(
      Math.pow(nodePos.x - this.viewport.center.x, 2) + 
      Math.pow(nodePos.y - this.viewport.center.y, 2)
    );
    
    const { zoom } = this.viewport;
    const adjustedDistance = distance / zoom;

    if (adjustedDistance < 200) return 'high';
    if (adjustedDistance < 500) return 'medium';
    if (adjustedDistance < 1000) return 'low';
    return 'minimal';
  }

  // Get virtualization statistics
  getStats(): {
    totalNodes: number;
    visibleNodes: number;
    culledNodes: number;
    renderQueue: number;
    virtualizationActive: boolean;
  } {
    return {
      totalNodes: this.nodePositions.size,
      visibleNodes: this.visibleNodes.size,
      culledNodes: this.nodePositions.size - this.visibleNodes.size,
      renderQueue: this.renderQueue.length,
      virtualizationActive: this.isVirtualizationActive(),
    };
  }

  // Update configuration
  updateConfig(newConfig: Partial<VirtualizationConfig>): void {
    this.config = { ...this.config, ...newConfig };
    this.updateVisibleNodes(); // Recalculate with new config
  }

  // Reset virtualization state
  reset(): void {
    this.nodePositions.clear();
    this.visibleNodes.clear();
    this.renderQueue = [];
  }
}

// Hook for using graph virtualization in React components
export function useGraphVirtualization(
  config?: Partial<VirtualizationConfig>
): {
  virtualizer: GraphVirtualizer;
  visibleNodes: Set<string>;
  renderQueue: string[];
  stats: ReturnType<GraphVirtualizer['getStats']>;
  updateViewport: (viewport: ViewportInfo) => void;
  updateNodePositions: (positions: Map<string, { x: number; y: number }>) => void;
} {
  const [virtualizer] = useState(() => new GraphVirtualizer(config));
  const [visibleNodes, setVisibleNodes] = useState<Set<string>>(new Set());
  const [renderQueue, setRenderQueue] = useState<string[]>([]);
  const [stats, setStats] = useState(() => virtualizer.getStats());

  const updateViewport = useCallback((viewport: ViewportInfo) => {
    virtualizer.updateViewport(viewport);
    setVisibleNodes(new Set(virtualizer.getVisibleNodes()));
    setRenderQueue([...virtualizer.getRenderQueue()]);
    setStats(virtualizer.getStats());
  }, [virtualizer]);

  const updateNodePositions = useCallback((positions: Map<string, { x: number; y: number }>) => {
    virtualizer.updateNodePositions(positions);
    setVisibleNodes(new Set(virtualizer.getVisibleNodes()));
    setRenderQueue([...virtualizer.getRenderQueue()]);
    setStats(virtualizer.getStats());
  }, [virtualizer]);

  return {
    virtualizer,
    visibleNodes,
    renderQueue,
    stats,
    updateViewport,
    updateNodePositions,
  };
}

// Utility function to calculate optimal LOD settings based on device capabilities
export function getOptimalVirtualizationConfig(): VirtualizationConfig {
  // Basic device detection
  const isHighEnd = navigator.hardwareConcurrency >= 8 && 
                   'memory' in navigator && 
                   (navigator as any).memory.totalJSHeapSize > 1024 * 1024 * 1024; // 1GB
  
  const isMobile = /Android|iPhone|iPad|iPod|BlackBerry|IEMobile|Opera Mini/i.test(navigator.userAgent);
  
  if (isMobile) {
    return {
      enabled: true,
      threshold: 200,
      bufferSize: 50,
      lodEnabled: true,
      maxRenderDistance: 1000,
    };
  }
  
  if (isHighEnd) {
    return {
      enabled: true,
      threshold: 1000,
      bufferSize: 200,
      lodEnabled: false,
      maxRenderDistance: 5000,
    };
  }
  
  // Default for average desktop
  return {
    enabled: true,
    threshold: 500,
    bufferSize: 100,
    lodEnabled: true,
    maxRenderDistance: 2000,
  };
}

export type { BoundingBox, ViewportInfo, NodePosition, VirtualizationConfig };
export { GraphVirtualizer };