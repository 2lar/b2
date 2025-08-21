import { useQuery } from '@tanstack/react-query';
import { categoriesApi } from '../api/categories';

/**
 * Centralized Categories Query Hook
 * 
 * Purpose:
 * Provides centralized category data fetching with optimized caching.
 * Categories change less frequently than memories, so longer cache times are used.
 * 
 * Key Features:
 * - Long cache duration (categories are relatively stable)
 * - Background refetching for data freshness
 * - Error handling for authentication issues
 * - Automatic retry with exponential backoff
 */

interface CategoriesQueryOptions {
    /** Whether to enable the query (default: true) */
    enabled?: boolean;
    /** Custom stale time in milliseconds */
    staleTime?: number;
    /** Whether to refetch on window focus */
    refetchOnWindowFocus?: boolean;
}

export function useCategoriesQuery(options?: CategoriesQueryOptions) {
    return useQuery({
        queryKey: ['categories'],
        queryFn: () => categoriesApi.listCategories(),
        staleTime: options?.staleTime ?? 10 * 60 * 1000, // 10 minutes (longer than memories)
        gcTime: 15 * 60 * 1000, // 15 minutes
        refetchOnWindowFocus: options?.refetchOnWindowFocus ?? false,
        refetchOnReconnect: 'always',
        enabled: options?.enabled ?? true,
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
        retryDelay: (attemptIndex) => Math.min(1000 * 2 ** attemptIndex, 30000),
    });
}

/**
 * Single Category Detail Query Hook
 * 
 * Purpose:
 * Fetches detailed information for a specific category.
 * Used for category detail views and editing interfaces.
 */
export function useCategoryQuery(categoryId: string | null, options?: { enabled?: boolean }) {
    return useQuery({
        queryKey: ['categories', categoryId],
        queryFn: () => categoryId ? categoriesApi.getCategory(categoryId) : null,
        enabled: Boolean(categoryId) && (options?.enabled ?? true),
        staleTime: 10 * 60 * 1000, // 10 minutes
        gcTime: 15 * 60 * 1000, // 15 minutes
        retry: (failureCount, error) => {
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