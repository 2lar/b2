import React from 'react';
import type { Node } from '../../../services';
import { useMemoriesFeed } from '../../memories';

type DashboardData = {
    memories: Node[];
    totalMemories: number;
    isLoading: boolean;
    isFetchingMore: boolean;
    hasMore: boolean;
    graphRefreshKey: number;
    sidebarRefreshKey: number;
    error: string | null;
    loadMore: () => Promise<void>;
    onMemoryCreated: () => Promise<void>;
    onMemoryDeleted: () => Promise<void>;
    onMemoryUpdated: () => Promise<void>;
    clearError: () => void;
};

export const useDashboardData = (pageSize = 50): DashboardData => {
    const [graphRefreshKey, setGraphRefreshKey] = React.useState(0);
    const [sidebarRefreshKey, setSidebarRefreshKey] = React.useState(0);
    const [error, setError] = React.useState<string | null>(null);

    const memoriesFeed = useMemoriesFeed(pageSize);

    const refreshDataConsumers = React.useCallback(() => {
        setGraphRefreshKey(prev => prev + 1);
        setSidebarRefreshKey(prev => prev + 1);
    }, []);

    const handleMemoryCreated = React.useCallback(async () => {
        try {
            await memoriesFeed.invalidate();
            await memoriesFeed.refetch();
            refreshDataConsumers();
            setError(null);
        } catch (feedError) {
            setError((feedError as Error).message || 'Unable to refresh memories.');
        }
    }, [memoriesFeed, refreshDataConsumers]);

    const handleMemoryChanged = React.useCallback(async () => {
        try {
            await memoriesFeed.refetch();
            refreshDataConsumers();
            setError(null);
        } catch (feedError) {
            setError((feedError as Error).message || 'Unable to refresh memories.');
        }
    }, [memoriesFeed, refreshDataConsumers]);

    React.useEffect(() => {
        if (memoriesFeed.error) {
            setError(memoriesFeed.error);
        }
    }, [memoriesFeed.error]);

    return {
        memories: memoriesFeed.items,
        totalMemories: memoriesFeed.total,
        isLoading: memoriesFeed.isInitialLoading,
        isFetchingMore: memoriesFeed.isFetchingNextPage,
        hasMore: memoriesFeed.hasNextPage,
        graphRefreshKey,
        sidebarRefreshKey,
        error,
        loadMore: memoriesFeed.fetchNextPage,
        onMemoryCreated: handleMemoryCreated,
        onMemoryDeleted: handleMemoryChanged,
        onMemoryUpdated: handleMemoryChanged,
        clearError: () => setError(null),
    };
};
