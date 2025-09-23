import { useQuery } from '@tanstack/react-query';
import { categoriesApi } from '../api/categories';
import type { components } from '../../../types/generated/generated-types';

const CATEGORY_NODES_KEY = ['category-nodes'];

type ApiNode = components['schemas']['Node'];

type CategoryNode = {
    nodeId: string;
    title?: string;
    content: string;
    timestamp?: string;
};

export const useCategoryNodes = (categoryId: string | null, limit = 5) => {
    const query = useQuery({
        queryKey: [...CATEGORY_NODES_KEY, categoryId, limit],
        enabled: Boolean(categoryId),
        queryFn: async () => {
            if (!categoryId) {
                return [] as CategoryNode[];
            }
            const response = await categoriesApi.getNodesInCategory?.(categoryId);
            const payload = (response as any) || {};
            const nodes: ApiNode[] = payload.memories || payload.nodes || [];
            return nodes.slice(0, limit).map(node => ({
                nodeId: (node as any).id || (node as any).nodeId,
                title: node.title,
                content: node.content || '',
                timestamp: (node as any).createdAt || node.timestamp,
            }));
        },
    });

    return {
        nodes: query.data || [],
        isLoading: query.isLoading,
        isFetching: query.isFetching,
        isError: query.isError,
        error: query.error,
        refetch: query.refetch,
    };
};

export const categoryNodesQueryKey = CATEGORY_NODES_KEY;
