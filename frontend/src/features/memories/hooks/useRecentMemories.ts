import { useQuery } from '@tanstack/react-query';
import { nodesApi } from '../api/nodes';
import type { Node } from '../../../services';

const RECENT_MEMORIES_KEY = ['recent-nodes'];

export const useRecentMemories = (limit = 5) => {
    const query = useQuery({
        queryKey: [...RECENT_MEMORIES_KEY, limit],
        queryFn: async () => {
            const response = await nodesApi.listNodes(limit);
            return response.nodes || [];
        },
    });

    return {
        memories: (query.data || []) as Node[],
        isLoading: query.isLoading,
        isFetching: query.isFetching,
        isError: query.isError,
        error: query.error,
        refetch: query.refetch,
    };
};

export const recentMemoriesQueryKey = RECENT_MEMORIES_KEY;
