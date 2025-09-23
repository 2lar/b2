import { useInfiniteQuery, useQueryClient } from '@tanstack/react-query';
import { nodesApi } from '../api/nodes';
import type { Node } from '../../../services';

type ListNodesResponse = {
    nodes?: Node[];
    total?: number;
    nextToken?: string;
};

type MemoriesFeed = {
    items: Node[];
    total: number;
    hasNextPage: boolean;
    isInitialLoading: boolean;
    isFetchingNextPage: boolean;
    error: string | null;
    fetchNextPage: () => Promise<void>;
    refetch: () => Promise<void>;
    invalidate: () => Promise<void>;
};

const QUERY_KEY = ['memories'];

export const useMemoriesFeed = (pageSize = 50): MemoriesFeed => {
    const queryClient = useQueryClient();

    const infiniteQuery = useInfiniteQuery<ListNodesResponse, Error>({
        queryKey: [...QUERY_KEY, pageSize],
        queryFn: async ({ pageParam }) => nodesApi.listNodes(pageSize, pageParam as string | undefined),
        getNextPageParam: (lastPage) => lastPage.nextToken ?? undefined,
        initialPageParam: undefined,
    });

    const items = (infiniteQuery.data?.pages.flatMap(page => page.nodes ?? []) ?? []).sort((a, b) => {
        const first = a?.timestamp ? new Date(a.timestamp).getTime() : 0;
        const second = b?.timestamp ? new Date(b.timestamp).getTime() : 0;
        return second - first;
    });
    const total = infiniteQuery.data?.pages[0]?.total ?? items.length;
    const hasNextPage = Boolean(infiniteQuery.hasNextPage);

    const fetchNextPage = async () => {
        if (infiniteQuery.hasNextPage) {
            await infiniteQuery.fetchNextPage();
        }
    };

    const refetch = async () => {
        await infiniteQuery.refetch();
    };

    const invalidate = async () => {
        await queryClient.invalidateQueries({ queryKey: [...QUERY_KEY, pageSize] });
    };

    return {
        items,
        total,
        hasNextPage,
        isInitialLoading: infiniteQuery.isInitialLoading,
        isFetchingNextPage: infiniteQuery.isFetchingNextPage,
        error: infiniteQuery.error ? infiniteQuery.error.message : null,
        fetchNextPage,
        refetch,
        invalidate,
    };
};
