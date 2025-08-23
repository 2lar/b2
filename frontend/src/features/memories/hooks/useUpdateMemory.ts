import { useMutation, useQueryClient } from '@tanstack/react-query';
import { nodesApi } from '../api/nodes';

interface UpdateMemoryData {
  nodeId: string;
  content: string;
  title?: string;
}

interface GraphData {
  nodes: Array<{
    id: string;
    content: string;
    title?: string;
    label: string;
    isPending?: boolean;
  }>;
  edges: Array<{
    id: string;
    source: string;
    target: string;
  }>;
}

export function useUpdateMemory() {
  const queryClient = useQueryClient();
  
  return useMutation({
    mutationFn: async (data: UpdateMemoryData) => {
      return await nodesApi.updateNode(data.nodeId, data.content, undefined, data.title);
    },
    // Optimistic update
    onMutate: async (variables) => {
      await queryClient.cancelQueries({ queryKey: ['graph'] });
      await queryClient.cancelQueries({ queryKey: ['nodes'] });
      
      const previousGraph = queryClient.getQueryData(['graph']);
      const previousNodes = queryClient.getQueryData(['nodes']);
      
      // Optimistically update graph data
      queryClient.setQueryData(['graph'], (old: GraphData | undefined) => {
        if (!old) return old;
        
        return {
          ...old,
          nodes: old.nodes.map(node => 
            node.id === variables.nodeId 
              ? { 
                  ...node, 
                  content: variables.content,
                  title: variables.title,
                  label: variables.title || variables.content.substring(0, 50),
                  isPending: true 
                }
              : node
          )
        };
      });
      
      // Optimistically update nodes list data
      queryClient.setQueryData(['nodes'], (old: any) => {
        if (!old) return old;
        
        return {
          ...old,
          nodes: old.nodes?.map((node: any) => 
            node.nodeId === variables.nodeId 
              ? { ...node, content: variables.content, title: variables.title, isPending: true }
              : node
          ) || []
        };
      });
      
      return { previousGraph, previousNodes, nodeId: variables.nodeId };
    },
    // Update with real data when response arrives
    onSuccess: (data, variables, context) => {
      // Update graph data with response
      queryClient.setQueryData(['graph'], (old: GraphData | undefined) => {
        if (!old) return old;
        
        return {
          ...old,
          nodes: old.nodes.map(node => 
            node.id === variables.nodeId 
              ? { 
                  ...node, 
                  content: variables.content,
                  title: variables.title,
                  label: variables.title || variables.content.substring(0, 50),
                  isPending: false 
                }
              : node
          )
        };
      });
      
      // Update nodes list data
      queryClient.setQueryData(['nodes'], (old: any) => {
        if (!old) return old;
        
        return {
          ...old,
          nodes: old.nodes?.map((node: any) => 
            node.nodeId === variables.nodeId 
              ? { ...node, content: variables.content, title: variables.title, isPending: false }
              : node
          ) || []
        };
      });
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
      console.error('Failed to update memory:', err);
    },
    onSettled: () => {
      // Refresh data to ensure consistency
      queryClient.invalidateQueries({ queryKey: ['graph'] });
      queryClient.invalidateQueries({ queryKey: ['nodes'] });
    }
  });
}