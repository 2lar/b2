import { useMutation, useQueryClient } from '@tanstack/react-query';
import { nodesApi } from '../api/nodes';

interface CreateMemoryData {
  content: string;
  tags?: string[];
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

export function useCreateMemory() {
  const queryClient = useQueryClient();
  
  return useMutation({
    mutationFn: async (data: CreateMemoryData) => {
      return await nodesApi.createNode(data.content, data.tags);
    },
    // Optimistic update
    onMutate: async (variables) => {
      await queryClient.cancelQueries({ queryKey: ['graph'] });
      
      const previousGraph = queryClient.getQueryData(['graph']);
      
      // Create temporary node with pending status
      const tempNode = {
        id: `temp-${Date.now()}`,
        content: variables.content,
        label: variables.content.substring(0, 50),
        isPending: true
      };
      
      queryClient.setQueryData(['graph'], (old: GraphData | undefined) => {
        if (!old) {
          return {
            nodes: [tempNode],
            edges: []
          };
        }
        
        return {
          ...old,
          nodes: [...old.nodes, tempNode]
        };
      });
      
      return { previousGraph, tempNodeId: tempNode.id };
    },
    // Replace with real data when response arrives
    onSuccess: (data, variables, context) => {
      queryClient.setQueryData(['graph'], (old: GraphData | undefined) => {
        if (!old) return old;
        
        // Remove temporary node and add real node
        const filtered = old.nodes.filter(n => n.id !== context?.tempNodeId);
        
        // Create the real node
        const realNode = {
          id: data.nodeId,
          content: data.content,
          label: data.content.substring(0, 50)
        };
        
        return {
          nodes: [...filtered, realNode],
          edges: old.edges // Edges will be updated via WebSocket
        };
      });
      
      // Invalidate related queries to ensure data consistency
      queryClient.invalidateQueries({ queryKey: ['nodes'] });
      queryClient.invalidateQueries({ queryKey: ['graph'] });
    },
    // Rollback on error
    onError: (err, variables, context) => {
      if (context?.previousGraph) {
        queryClient.setQueryData(['graph'], context.previousGraph);
      }
    }
  });
}