import React from 'react';
import { nodesApi } from '../../memories';
import { categoriesApi } from '../../categories';
import type { Category, Node } from '../../../services';

const DEFAULT_PAGE_SIZE = 50;

type PaginationState = {
    currentPage: number;
    totalPages: number;
    totalMemories: number;
    nextToken?: string;
};

type DashboardData = {
    memories: Node[];
    categories: Category[];
    pagination: PaginationState;
    isLoading: boolean;
    error: string | null;
    graphRefreshKey: number;
    sidebarRefreshKey: number;
    onPageChange: (page: number) => void;
    onMemoryCreated: () => Promise<void>;
    onMemoryDeleted: () => Promise<void>;
    onMemoryUpdated: () => Promise<void>;
    clearError: () => void;
};

const sortByTimestamp = (nodes: Node[]): Node[] => {
    return [...nodes].sort((a, b) => {
        const first = a.timestamp ? new Date(a.timestamp).getTime() : 0;
        const second = b.timestamp ? new Date(b.timestamp).getTime() : 0;
        return second - first;
    });
};

export const useDashboardData = (pageSize: number = DEFAULT_PAGE_SIZE): DashboardData => {
    const [memories, setMemories] = React.useState<Node[]>([]);
    const [categories, setCategories] = React.useState<Category[]>([]);
    const [pagination, setPagination] = React.useState<PaginationState>({
        currentPage: 1,
        totalPages: 1,
        totalMemories: 0,
        nextToken: undefined,
    });
    const [pageTokens, setPageTokens] = React.useState<Map<number, string>>(new Map());
    const [isLoading, setIsLoading] = React.useState(false);
    const [error, setError] = React.useState<string | null>(null);
    const [graphRefreshKey, setGraphRefreshKey] = React.useState(0);
    const [sidebarRefreshKey, setSidebarRefreshKey] = React.useState(0);

    const updatePagination = React.useCallback((page: number, total: number, nextToken?: string) => {
        setPagination({
            currentPage: page,
            totalPages: Math.max(1, Math.ceil(total / pageSize)),
            totalMemories: total,
            nextToken,
        });
    }, [pageSize]);

    const loadCategories = React.useCallback(async () => {
        try {
            const data = await categoriesApi.listCategories();
            setCategories(data.categories || []);
        } catch (loadError) {
            console.error('Error loading categories:', loadError);
            setError('Unable to load categories. Please try again later.');
        }
    }, []);

    const loadMemories = React.useCallback(async (page: number, token?: string) => {
        setIsLoading(true);
        try {
            const data = await nodesApi.listNodes(pageSize, token);
            const nodes = sortByTimestamp(data.nodes || []);
            const total = data.total ?? nodes.length;

            setMemories(nodes);
            updatePagination(page, total, data.nextToken);
            setError(null);

            setPageTokens(prevTokens => {
                const nextTokens = new Map(prevTokens);
                if (data.nextToken) {
                    nextTokens.set(page + 1, data.nextToken);
                } else {
                    nextTokens.delete(page + 1);
                }
                return nextTokens;
            });
        } catch (loadError) {
            console.error('Error loading memories:', loadError);
            setError((loadError as Error).message || 'Unable to load memories.');
        } finally {
            setIsLoading(false);
        }
    }, [pageSize, updatePagination]);

    const loadPageFromBeginning = React.useCallback(async (targetPage: number) => {
        setIsLoading(true);
        try {
            let currentToken: string | undefined;
            const nextTokens = new Map<number, string>();

            for (let page = 1; page <= targetPage; page += 1) {
                const response = await nodesApi.listNodes(pageSize, currentToken);

                if (page === targetPage) {
                    const nodes = sortByTimestamp(response.nodes || []);
                    const total = response.total ?? nodes.length;
                    setMemories(nodes);
                    updatePagination(page, total, response.nextToken);
                }

                if (response.nextToken) {
                    nextTokens.set(page + 1, response.nextToken);
                }
                currentToken = response.nextToken;
            }

            setPageTokens(nextTokens);
            setError(null);
        } catch (loadError) {
            console.error('Error rebuilding pagination sequence:', loadError);
            setError((loadError as Error).message || 'Unable to load that page.');
        } finally {
            setIsLoading(false);
        }
    }, [pageSize, updatePagination]);

    const refreshDataConsumers = React.useCallback(() => {
        setGraphRefreshKey(prev => prev + 1);
        setSidebarRefreshKey(prev => prev + 1);
    }, []);

    const handleMemoryCreated = React.useCallback(async () => {
        setPageTokens(new Map());
        await loadMemories(1);
        await loadCategories();
        refreshDataConsumers();
    }, [loadCategories, loadMemories, refreshDataConsumers]);

    const handleMemoryDeleted = React.useCallback(async () => {
        if (pagination.currentPage === 1) {
            await loadMemories(1);
        } else {
            await loadPageFromBeginning(pagination.currentPage);
        }
        await loadCategories();
        refreshDataConsumers();
    }, [loadCategories, loadMemories, loadPageFromBeginning, pagination.currentPage, refreshDataConsumers]);

    const handleMemoryUpdated = React.useCallback(async () => {
        if (pagination.currentPage === 1) {
            await loadMemories(1);
        } else {
            await loadPageFromBeginning(pagination.currentPage);
        }
        await loadCategories();
        refreshDataConsumers();
    }, [loadCategories, loadMemories, loadPageFromBeginning, pagination.currentPage, refreshDataConsumers]);

    const handlePageChange = React.useCallback((page: number) => {
        if (page === 1) {
            void loadMemories(1);
            return;
        }

        if (page > pagination.currentPage) {
            const tokenForPage = pageTokens.get(page);
            void loadMemories(page, tokenForPage);
            return;
        }

        void loadPageFromBeginning(page);
    }, [loadMemories, loadPageFromBeginning, pageTokens, pagination.currentPage]);

    React.useEffect(() => {
        void Promise.all([
            loadMemories(1),
            loadCategories(),
        ]);
    }, [loadCategories, loadMemories]);

    return {
        memories,
        categories,
        pagination,
        isLoading,
        error,
        graphRefreshKey,
        sidebarRefreshKey,
        onPageChange: handlePageChange,
        onMemoryCreated: handleMemoryCreated,
        onMemoryDeleted: handleMemoryDeleted,
        onMemoryUpdated: handleMemoryUpdated,
        clearError: () => setError(null),
    };
};
