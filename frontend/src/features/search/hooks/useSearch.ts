import { useQuery } from '@tanstack/react-query';
import { searchApi } from '../api/search';
import type { SearchResponse } from '../types/search';

const SEARCH_QUERY_KEY = ['search'] as const;

export function useSearch(query: string, limit = 20, offset = 0) {
    return useQuery<SearchResponse>({
        queryKey: [...SEARCH_QUERY_KEY, query, limit, offset],
        queryFn: () => searchApi.search(query, limit, offset),
        enabled: query.length >= 2,
        staleTime: 30_000,
        placeholderData: (prev) => prev,
    });
}
