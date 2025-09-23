import { useQuery } from '@tanstack/react-query';
import { categoriesApi } from '../api/categories';
import type { Category } from '../../../services';

const CATEGORIES_KEY = ['categories'];

export const useCategories = () => {
    const query = useQuery({
        queryKey: CATEGORIES_KEY,
        queryFn: async () => {
            const response = await categoriesApi.listCategories();
            const categories = response.categories || [];
            return categories.sort((a, b) => a.title.localeCompare(b.title));
        },
    });

    return {
        categories: (query.data || []) as Category[],
        isLoading: query.isLoading,
        isFetching: query.isFetching,
        isError: query.isError,
        error: query.error,
        refetch: query.refetch,
    };
};

export const categoriesQueryKey = CATEGORIES_KEY;
