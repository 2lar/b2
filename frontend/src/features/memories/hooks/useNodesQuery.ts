import { useInfiniteQuery, useQuery } from '@tanstack/react-query';
import { nodesApi } from '../api/nodes';

/**
 * Centralized Nodes List Query Hook
 * 
 * Purpose:
 * Provides centralized node/memory list fetching with pagination support.
 * Optimized for both traditional pagination and infinite scroll scenarios.
 * 
 * Key Features:
 * - Token-based pagination support
 * - Automatic cache invalidation
 * - Optimistic updates integration
 * - Error handling and retry logic
 */

interface NodesQueryOptions {
    /** Number of items per page */
    pageSize?: number;
    /** Initial token for pagination */
    token?: string;
    /** Whether to enable the query */
    enabled?: boolean;
    /** Custom stale time */
    staleTime?: number;
}

export function useNodesQuery(options?: NodesQueryOptions) {
    const { pageSize = 50, token, enabled = true, staleTime = 2 * 60 * 1000 } = options || {};

    return useQuery({
        queryKey: ['nodes', { pageSize, token }],
        queryFn: () => nodesApi.listNodes(pageSize, token),
        staleTime,
        gcTime: 5 * 60 * 1000, // 5 minutes
        refetchOnWindowFocus: false,
        enabled,
        retry: (failureCount, error) => {
            // Don't retry on authentication errors
            if (error && typeof error === 'object' && 'message' in error) {
                const message = (error as any).message;
                if (message?.includes('Authentication') || message?.includes('expired')) {
                    return false;
                }
            }
            return failureCount < 3;
        },
    });
}

/**
 * Infinite Scroll Query Hook for Memories
 * 
 * Purpose:
 * Provides infinite scrolling capability for memory lists.
 * Automatically handles pagination tokens and page concatenation.
 */
export function useInfiniteNodesQuery(pageSize: number = 50) {
    return useInfiniteQuery({
        queryKey: ['nodes', 'infinite', pageSize],
        queryFn: ({ pageParam }: { pageParam?: string }) => nodesApi.listNodes(pageSize, pageParam),
        initialPageParam: undefined,
        getNextPageParam: (lastPage: any) => {
            // Return the next token if available, undefined if no more pages
            return lastPage?.nextToken || undefined;
        },
        staleTime: 2 * 60 * 1000, // 2 minutes
        gcTime: 5 * 60 * 1000, // 5 minutes
        refetchOnWindowFocus: false,
        retry: (failureCount, error) => {
            if (error && typeof error === 'object' && 'message' in error) {
                const message = (error as any).message;
                if (message?.includes('Authentication') || message?.includes('expired')) {
                    return false;
                }
            }
            return failureCount < 3;
        },
    });
}

/**
 * Single Node Detail Query Hook
 * 
 * Purpose:
 * Fetches detailed information for a specific node/memory.
 * Used for node details panels and editing interfaces.
 */
export function useNodeQuery(nodeId: string | null, options?: { enabled?: boolean }) {
    return useQuery({
        queryKey: ['nodes', nodeId],
        queryFn: () => nodeId ? nodesApi.getNode(nodeId) : null,
        enabled: Boolean(nodeId) && (options?.enabled ?? true),
        staleTime: 5 * 60 * 1000, // 5 minutes
        gcTime: 10 * 60 * 1000, // 10 minutes
        retry: (failureCount, error) => {
            // Don't retry on 404 errors
            if (error && typeof error === 'object' && 'status' in error) {
                const status = (error as any).status;
                if (status === 404) {
                    return false;
                }
            }
            return failureCount < 2;
        },
    });
}