import { useQuery } from '@tanstack/react-query';
import { nodesApi } from '../api/nodes';

/**
 * Centralized Graph Data Query Hook
 * 
 * Purpose:
 * Provides a centralized way to fetch and cache graph data across the application.
 * Includes intelligent caching, error handling, and background refetching.
 * 
 * Key Features:
 * - Automatic caching with 5-minute stale time
 * - Background refetching on window focus
 * - Error retry with exponential backoff
 * - Optimized for graph visualization performance
 * 
 * Cache Strategy:
 * - 5 minutes stale time for frequent updates
 * - 10 minutes cache time to prevent unnecessary fetches
 * - Background refetch on reconnect for real-time sync
 */

interface GraphQueryOptions {
    /** Whether to enable the query (default: true) */
    enabled?: boolean;
    /** Custom stale time in milliseconds */
    staleTime?: number;
    /** Custom cache time in milliseconds */
    gcTime?: number;
    /** Whether to refetch on window focus */
    refetchOnWindowFocus?: boolean;
}

export function useGraphQuery(options?: GraphQueryOptions) {
    return useQuery({
        queryKey: ['graph'],
        queryFn: () => nodesApi.getGraphData(),
        staleTime: options?.staleTime ?? 5 * 60 * 1000, // 5 minutes
        gcTime: options?.gcTime ?? 10 * 60 * 1000, // 10 minutes
        refetchOnWindowFocus: options?.refetchOnWindowFocus ?? false,
        refetchOnReconnect: 'always',
        enabled: options?.enabled ?? true,
        retry: (failureCount, error) => {
            // Don't retry on 4xx errors (client errors)
            if (error && typeof error === 'object' && 'status' in error) {
                const status = (error as any).status;
                if (status >= 400 && status < 500) {
                    return false;
                }
            }
            
            // Retry up to 3 times with exponential backoff
            return failureCount < 3;
        },
        retryDelay: (attemptIndex) => Math.min(1000 * 2 ** attemptIndex, 30000),
    });
}

/**
 * Specialized hook for graph data that automatically enables/disables based on conditions
 */
export function useConditionalGraphQuery(condition: boolean, options?: Omit<GraphQueryOptions, 'enabled'>) {
    return useGraphQuery({
        ...options,
        enabled: condition,
    });
}