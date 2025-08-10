import { create } from 'zustand';
import { devtools, persist } from 'zustand/middleware';
import { api } from '../services/apiClient';
import { components } from '../types/generated/generated-types';

// Type definitions for the store
type Node = components['schemas']['Node'];
type NodeDetails = components['schemas']['NodeDetails'];

interface LoadingStates {
  graphLoading: boolean;
  nodeLoading: Set<string>;
  operationLoading: Map<string, 'create' | 'update' | 'delete'>;
  backgroundSync: boolean;
}

interface ErrorStates {
  graphError: Error | null;
  nodeErrors: Map<string, Error>;
  operationErrors: Map<string, Error>;
  connectionError: Error | null;
}

interface CacheState {
  nodeDetails: Map<string, NodeDetails>;
  neighborhoods: Map<string, { data: any; depth: number; timestamp: number }>;
  lastFetched: Map<string, number>;
  pendingOperations: Set<string>;
  optimisticUpdates: Map<string, OptimisticUpdate>;
}

interface OptimisticUpdate {
  type: 'create' | 'update' | 'delete';
  timestamp: number;
  originalData?: any;
}

interface GraphState {
  // Core data
  nodes: Map<string, Node>;
  edges: Map<string, { source: string; target: string }>;
  
  // UI state
  selectedNodes: Set<string>;
  selectedEdges: Set<string>;
  focusedNode: string | null;
  isSidebarOpen: boolean;
  
  // Performance states
  loadingStates: LoadingStates;
  errorStates: ErrorStates;
  cache: CacheState;
  
  // Actions
  loadGraph: (options?: { force?: boolean }) => Promise<void>;
  addNode: (content: string, tags?: string[], optimistic?: boolean) => Promise<Node>;
  updateNode: (nodeId: string, content: string, tags?: string[]) => Promise<void>;
  deleteNodes: (nodeIds: string[]) => Promise<void>;
  deleteNode: (nodeId: string) => Promise<void>;
  selectNode: (nodeId: string, multiSelect?: boolean) => void;
  focusNode: (nodeId: string) => void;
  clearSelection: () => void;
  
  // Advanced actions
  getNodeNeighborhood: (nodeId: string, depth: number) => Promise<any>;
  prefetchNodeDetails: (nodeId: string) => Promise<void>;
  
  // Cache management
  invalidateCache: (keys?: string[]) => void;
  clearCache: () => void;
  
  // Error handling
  clearErrors: () => void;
  retryOperation: (operationId: string) => Promise<void>;
  
  // UI actions
  toggleSidebar: () => void;
}

const createLoadingStates = (): LoadingStates => ({
  graphLoading: false,
  nodeLoading: new Set(),
  operationLoading: new Map(),
  backgroundSync: false,
});

const createErrorStates = (): ErrorStates => ({
  graphError: null,
  nodeErrors: new Map(),
  operationErrors: new Map(),
  connectionError: null,
});

const createCacheState = (): CacheState => ({
  nodeDetails: new Map(),
  neighborhoods: new Map(),
  lastFetched: new Map(),
  pendingOperations: new Set(),
  optimisticUpdates: new Map(),
});

const CACHE_DURATION = 5 * 60 * 1000; // 5 minutes

export const useGraphStore = create<GraphState>()(
  devtools(
    persist(
      (set, get) => ({
        // Initial state
        nodes: new Map(),
        edges: new Map(),
        selectedNodes: new Set(),
        selectedEdges: new Set(),
        focusedNode: null,
        isSidebarOpen: false,
        loadingStates: createLoadingStates(),
        errorStates: createErrorStates(),
        cache: createCacheState(),

        // Core actions
        loadGraph: async (options = {}) => {
          const state = get();
          const cacheKey = 'graph-data';
          const lastFetched = state.cache.lastFetched.get(cacheKey) || 0;
          const now = Date.now();

          // Check cache validity
          if (!options.force && (now - lastFetched) < CACHE_DURATION) {
            return; // Use cached data
          }

          set((state: GraphState) => ({
            loadingStates: {
              ...state.loadingStates,
              graphLoading: true,
            },
            errorStates: {
              ...state.errorStates,
              graphError: null,
            },
          }));

          try {
            const graphData = await api.getGraphData();
            
            // Convert to Maps for better performance
            const nodesMap = new Map();
            const edgesMap = new Map();
            
            // Process elements to separate nodes and edges
            if (graphData.elements) {
              graphData.elements.forEach((element: any) => {
                if (element.data && element.data.id && element.data.label) {
                  // This is a node
                  nodesMap.set(element.data.id, {
                    nodeId: element.data.id,
                    content: element.data.label,
                    tags: [],
                    timestamp: new Date().toISOString(),
                    version: 0,
                  });
                } else if (element.data && element.data.source && element.data.target) {
                  // This is an edge
                  const edgeId = `${element.data.source}-${element.data.target}`;
                  edgesMap.set(edgeId, {
                    source: element.data.source,
                    target: element.data.target,
                  });
                }
              });
            }

            set((state: GraphState) => ({
              nodes: nodesMap,
              edges: edgesMap,
              loadingStates: {
                ...state.loadingStates,
                graphLoading: false,
              },
              cache: {
                ...state.cache,
                lastFetched: new Map(state.cache.lastFetched.set(cacheKey, now)),
              },
            }));
          } catch (error) {
            console.error('Failed to load graph:', error);
            set((state: GraphState) => ({
              loadingStates: {
                ...state.loadingStates,
                graphLoading: false,
              },
              errorStates: {
                ...state.errorStates,
                graphError: error as Error,
              },
            }));
            throw error;
          }
        },

        addNode: async (content: string, tags = [], optimistic = true) => {
          const tempId = `temp-${Date.now()}`;
          let optimisticNode: Node | null = null;

          if (optimistic) {
            // Create optimistic node
            optimisticNode = {
              nodeId: tempId,
              content,
              tags,
              timestamp: new Date().toISOString(),
              version: 0,
            };

            set((state: GraphState) => ({
              nodes: new Map(state.nodes.set(tempId, optimisticNode!)),
              cache: {
                ...state.cache,
                optimisticUpdates: new Map(state.cache.optimisticUpdates.set(tempId, {
                  type: 'create',
                  timestamp: Date.now(),
                })),
              },
            }));
          }

          // Mark operation as loading
          set((state: GraphState) => ({
            loadingStates: {
              ...state.loadingStates,
              operationLoading: new Map(state.loadingStates.operationLoading.set(tempId, 'create')),
            },
          }));

          try {
            const realNode = await api.createNode(content, tags);

            set((state: GraphState) => {
              const newNodes = new Map(state.nodes);
              const newOperationLoading = new Map(state.loadingStates.operationLoading);
              const newOptimisticUpdates = new Map(state.cache.optimisticUpdates);

              // Remove optimistic node if it exists
              if (optimistic) {
                newNodes.delete(tempId);
                newOptimisticUpdates.delete(tempId);
              }

              // Add real node
              newNodes.set(realNode.nodeId, realNode);
              newOperationLoading.delete(tempId);

              return {
                nodes: newNodes,
                loadingStates: {
                  ...state.loadingStates,
                  operationLoading: newOperationLoading,
                },
                cache: {
                  ...state.cache,
                  optimisticUpdates: newOptimisticUpdates,
                },
              };
            });

            return realNode;
          } catch (error) {
            console.error('Failed to create node:', error);
            
            // Rollback optimistic update
            if (optimistic) {
              set((state: GraphState) => {
                const newNodes = new Map(state.nodes);
                newNodes.delete(tempId);

                return {
                  nodes: newNodes,
                  loadingStates: {
                    ...state.loadingStates,
                    operationLoading: (() => {
                      const newMap = new Map(state.loadingStates.operationLoading);
                      newMap.delete(tempId);
                      return newMap;
                    })(),
                  },
                  errorStates: {
                    ...state.errorStates,
                    operationErrors: new Map(state.errorStates.operationErrors.set(tempId, error as Error)),
                  },
                  cache: {
                    ...state.cache,
                    optimisticUpdates: (() => {
                      const newMap = new Map(state.cache.optimisticUpdates);
                      newMap.delete(tempId);
                      return newMap;
                    })(),
                  },
                };
              });
            }
            throw error;
          }
        },

        updateNode: async (nodeId: string, content: string, tags = []) => {
          const state = get();
          const originalNode = state.nodes.get(nodeId);
          
          if (!originalNode) {
            throw new Error('Node not found');
          }

          // Optimistic update
          const updatedNode = { ...originalNode, content, tags };
          set((state: GraphState) => ({
            nodes: new Map(state.nodes.set(nodeId, updatedNode)),
            cache: {
              ...state.cache,
              optimisticUpdates: new Map(state.cache.optimisticUpdates.set(nodeId, {
                type: 'update',
                timestamp: Date.now(),
                originalData: originalNode,
              })),
            },
          }));

          try {
            await api.updateNode(nodeId, content, tags);
            
            // Clear optimistic update on success
            set((state: GraphState) => ({
              cache: {
                ...state.cache,
                optimisticUpdates: (() => {
                  const newMap = new Map(state.cache.optimisticUpdates);
                  newMap.delete(nodeId);
                  return newMap;
                })(),
              },
            }));
          } catch (error) {
            console.error('Failed to update node:', error);
            
            // Rollback optimistic update
            set((state: GraphState) => ({
              nodes: new Map(state.nodes.set(nodeId, originalNode)),
              errorStates: {
                ...state.errorStates,
                operationErrors: new Map(state.errorStates.operationErrors.set(nodeId, error as Error)),
              },
              cache: {
                ...state.cache,
                optimisticUpdates: (() => {
                  const newMap = new Map(state.cache.optimisticUpdates);
                  newMap.delete(nodeId);
                  return newMap;
                })(),
              },
            }));
            throw error;
          }
        },

        deleteNodes: async (nodeIds: string[]) => {
          const state = get();
          const originalNodes = new Map();
          
          // Store original nodes for rollback
          nodeIds.forEach(id => {
            const node = state.nodes.get(id);
            if (node) {
              originalNodes.set(id, node);
            }
          });

          // Optimistic deletion
          set((state: GraphState) => {
            const newNodes = new Map(state.nodes);
            nodeIds.forEach(id => newNodes.delete(id));
            
            return {
              nodes: newNodes,
              cache: {
                ...state.cache,
                optimisticUpdates: new Map(state.cache.optimisticUpdates.set('bulk-delete', {
                  type: 'delete',
                  timestamp: Date.now(),
                  originalData: originalNodes,
                })),
              },
            };
          });

          try {
            await api.bulkDeleteNodes(nodeIds);
            
            // Clear optimistic update on success
            set((state: GraphState) => ({
              cache: {
                ...state.cache,
                optimisticUpdates: (() => {
                  const newMap = new Map(state.cache.optimisticUpdates);
                  newMap.delete('bulk-delete');
                  return newMap;
                })(),
              },
            }));
          } catch (error) {
            console.error('Failed to delete nodes:', error);
            
            // Rollback deletion
            set((state: GraphState) => {
              const newNodes = new Map(state.nodes);
              originalNodes.forEach((node, id) => {
                newNodes.set(id, node);
              });
              
              return {
                nodes: newNodes,
                errorStates: {
                  ...state.errorStates,
                  operationErrors: new Map(state.errorStates.operationErrors.set('bulk-delete', error as Error)),
                },
                cache: {
                  ...state.cache,
                  optimisticUpdates: (() => {
                    const newMap = new Map(state.cache.optimisticUpdates);
                    newMap.delete('bulk-delete');
                    return newMap;
                  })(),
                },
              };
            });
            throw error;
          }
        },

        deleteNode: async (nodeId: string) => {
          return get().deleteNodes([nodeId]);
        },

        // UI actions
        selectNode: (nodeId: string, multiSelect = false) => {
          set((state: GraphState) => {
            const newSelectedNodes = multiSelect 
              ? new Set(state.selectedNodes).add(nodeId)
              : new Set([nodeId]);
            
            return {
              selectedNodes: newSelectedNodes,
              focusedNode: nodeId,
            };
          });
        },

        focusNode: (nodeId: string) => {
          set({ focusedNode: nodeId });
        },

        clearSelection: () => {
          set({
            selectedNodes: new Set(),
            selectedEdges: new Set(),
            focusedNode: null,
          });
        },

        // Advanced actions
        getNodeNeighborhood: async (nodeId: string, depth: number = 2) => {
          const cacheKey = `${nodeId}-${depth}`;
          const state = get();
          const cached = state.cache.neighborhoods.get(cacheKey);
          
          if (cached && (Date.now() - cached.timestamp) < CACHE_DURATION) {
            return cached.data;
          }

          try {
            // TODO: Add neighborhood endpoint to API client when available
            // For now, return the full graph data as a fallback
            const data = await api.getGraphData();
            
            // Cache the result
            set((state: GraphState) => ({
              cache: {
                ...state.cache,
                neighborhoods: new Map(state.cache.neighborhoods.set(cacheKey, {
                  data,
                  depth,
                  timestamp: Date.now(),
                })),
              },
            }));
            
            return data;
          } catch (error) {
            console.error('Failed to get node neighborhood:', error);
            throw error;
          }
        },

        prefetchNodeDetails: async (nodeId: string) => {
          const state = get();
          if (state.cache.nodeDetails.has(nodeId)) {
            return; // Already cached
          }

          try {
            const details = await api.getNode(nodeId);
            set((state: GraphState) => ({
              cache: {
                ...state.cache,
                nodeDetails: new Map(state.cache.nodeDetails.set(nodeId, details)),
              },
            }));
          } catch (error) {
            console.error('Failed to prefetch node details:', error);
          }
        },

        // Cache management
        invalidateCache: (keys?: string[]) => {
          set((state: GraphState) => {
            if (!keys) {
              // Clear all cache
              return {
                cache: createCacheState(),
              };
            }
            
            // Clear specific keys
            const newLastFetched = new Map(state.cache.lastFetched);
            keys.forEach(key => newLastFetched.delete(key));
            
            return {
              cache: {
                ...state.cache,
                lastFetched: newLastFetched,
              },
            };
          });
        },

        clearCache: () => {
          set((state: GraphState) => ({
            cache: createCacheState(),
          }));
        },

        // Error handling
        clearErrors: () => {
          set({
            errorStates: createErrorStates(),
          });
        },

        retryOperation: async (operationId: string) => {
          // Implementation for retrying failed operations
          console.log('Retrying operation:', operationId);
        },

        // UI actions
        toggleSidebar: () => {
          set((state: GraphState) => ({
            isSidebarOpen: !state.isSidebarOpen,
          }));
        },
      }),
      {
        name: 'brain2-graph-storage',
        partialize: (state) => ({
          // Only persist essential data
          nodes: Array.from(state.nodes.entries()),
          edges: Array.from(state.edges.entries()),
          isSidebarOpen: state.isSidebarOpen,
          cache: {
            ...state.cache,
            nodeDetails: Array.from(state.cache.nodeDetails.entries()),
            lastFetched: Array.from(state.cache.lastFetched.entries()),
          },
        }),
        onRehydrateStorage: () => (state) => {
          if (state) {
            // Convert arrays back to Maps
            state.nodes = new Map(state.nodes as any);
            state.edges = new Map(state.edges as any);
            if (state.cache) {
              state.cache.nodeDetails = new Map(state.cache.nodeDetails as any);
              state.cache.lastFetched = new Map(state.cache.lastFetched as any);
              // Recreate non-serializable state
              state.cache.neighborhoods = new Map();
              state.cache.pendingOperations = new Set();
              state.cache.optimisticUpdates = new Map();
            }
            // Recreate non-serializable state
            state.selectedNodes = new Set();
            state.selectedEdges = new Set();
            state.loadingStates = createLoadingStates();
            state.errorStates = createErrorStates();
          }
        },
      }
    )
  )
);