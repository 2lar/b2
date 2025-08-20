import { useMutation, useQueryClient } from '@tanstack/react-query';
import { nodesApi } from '../api/nodes';

interface DeleteMemoryData {
  nodeId: string;
}

interface GraphData {
  nodes: Array<{
    id: string;
    content: string;
    label: string;
    isPending?: boolean;
  }>;
  edges: Array<{
    id: string;
    source: string;
    target: string;
  }>;
}

export function useDeleteMemory() {
  const queryClient = useQueryClient();
  
  return useMutation({
    mutationFn: async (data: DeleteMemoryData) => {
      return await nodesApi.deleteNode(data.nodeId);
    },
    // Optimistic update
    onMutate: async (variables) => {
      await queryClient.cancelQueries({ queryKey: ['graph'] });
      await queryClient.cancelQueries({ queryKey: ['nodes'] });
      
      const previousGraph = queryClient.getQueryData(['graph']);
      const previousNodes = queryClient.getQueryData(['nodes']);
      
      // Optimistically remove from graph data
      queryClient.setQueryData(['graph'], (old: GraphData | undefined) => {
        if (!old) return old;
        
        return {
          nodes: old.nodes.filter(node => node.id !== variables.nodeId),
          edges: old.edges.filter(edge => 
            edge.source !== variables.nodeId && edge.target !== variables.nodeId
          )
        };
      });
      
      // Optimistically remove from nodes list data
      queryClient.setQueryData(['nodes'], (old: any) => {
        if (!old) return old;
        
        return {
          ...old,
          nodes: old.nodes?.filter((node: any) => node.nodeId !== variables.nodeId) || [],
          total: Math.max(0, (old.total || 1) - 1)
        };
      });
      
      return { previousGraph, previousNodes, nodeId: variables.nodeId };
    },
    // Success handling
    onSuccess: (data, variables, context) => {
      // The optimistic update was correct, data is already updated
      console.log('Memory deleted successfully');
    },
    // Rollback on error
    onError: (err, variables, context) => {
      if (context?.previousGraph) {
        queryClient.setQueryData(['graph'], context.previousGraph);
      }
      if (context?.previousNodes) {
        queryClient.setQueryData(['nodes'], context.previousNodes);
      }
      
      // Show error feedback
      console.error('Failed to delete memory:', err);
    },
    onSettled: () => {
      // Refresh data to ensure consistency
      queryClient.invalidateQueries({ queryKey: ['graph'] });
      queryClient.invalidateQueries({ queryKey: ['nodes'] });
      queryClient.invalidateQueries({ queryKey: ['categories'] });
    }
  });
}

export function useBulkDeleteMemories() {
  const queryClient = useQueryClient();
  
  return useMutation({
    mutationFn: async (nodeIds: string[]) => {
      return await nodesApi.bulkDeleteNodes(nodeIds);
    },
    // Optimistic update
    onMutate: async (variables) => {
      await queryClient.cancelQueries({ queryKey: ['graph'] });
      await queryClient.cancelQueries({ queryKey: ['nodes'] });
      
      const previousGraph = queryClient.getQueryData(['graph']);
      const previousNodes = queryClient.getQueryData(['nodes']);
      
      const nodeIds = new Set(variables);
      
      // Optimistically remove from graph data
      queryClient.setQueryData(['graph'], (old: GraphData | undefined) => {
        if (!old) return old;
        
        return {
          nodes: old.nodes.filter(node => !nodeIds.has(node.id)),
          edges: old.edges.filter(edge => 
            !nodeIds.has(edge.source) && !nodeIds.has(edge.target)
          )
        };
      });
      
      // Optimistically remove from nodes list data
      queryClient.setQueryData(['nodes'], (old: any) => {
        if (!old) return old;
        
        return {
          ...old,
          nodes: old.nodes?.filter((node: any) => !nodeIds.has(node.nodeId)) || [],
          total: Math.max(0, (old.total || variables.length) - variables.length)
        };
      });
      
      return { previousGraph, previousNodes, nodeIds: variables };
    },
    // Success handling
    onSuccess: (data, variables, context) => {
      console.log(`${variables.length} memories deleted successfully`);
    },
    // Rollback on error
    onError: (err, variables, context) => {
      if (context?.previousGraph) {
        queryClient.setQueryData(['graph'], context.previousGraph);
      }
      if (context?.previousNodes) {
        queryClient.setQueryData(['nodes'], context.previousNodes);
      }
      
      console.error('Failed to bulk delete memories:', err);
    },
    onSettled: () => {
      // Refresh data to ensure consistency
      queryClient.invalidateQueries({ queryKey: ['graph'] });
      queryClient.invalidateQueries({ queryKey: ['nodes'] });
      queryClient.invalidateQueries({ queryKey: ['categories'] });
    }
  });
}