import { create } from 'zustand';

const isDesktopViewport = () => {
    if (typeof window === 'undefined') {
        return true;
    }
    return window.innerWidth >= 1024;
};

const isTabletViewport = () => {
    if (typeof window === 'undefined') {
        return true;
    }
    return window.innerWidth >= 768;
};

interface LayoutState {
    isAppSidebarOpen: boolean;
    isLeftPanelOpen: boolean;
    setAppSidebarOpen: (open: boolean) => void;
    toggleAppSidebar: () => void;
    setLeftPanelOpen: (open: boolean) => void;
    toggleLeftPanel: () => void;
    initializeFromViewport: () => void;
}

export const useLayoutStore = create<LayoutState>((set) => ({
    isAppSidebarOpen: isDesktopViewport(),
    isLeftPanelOpen: isTabletViewport(),
    setAppSidebarOpen: (open: boolean) => set({ isAppSidebarOpen: open }),
    toggleAppSidebar: () => set((state) => ({ isAppSidebarOpen: !state.isAppSidebarOpen })),
    setLeftPanelOpen: (open: boolean) => set({ isLeftPanelOpen: open }),
    toggleLeftPanel: () => set((state) => ({ isLeftPanelOpen: !state.isLeftPanelOpen })),
    initializeFromViewport: () => set({
        isAppSidebarOpen: isDesktopViewport(),
        isLeftPanelOpen: isTabletViewport(),
    }),
}));
