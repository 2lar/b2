import { useState, useEffect } from 'react';

interface UseFullscreenReturn {
    isFullscreen: boolean;
    toggleFullscreen: () => Promise<void>;
}

export const useFullscreen = (elementRef: React.RefObject<HTMLElement | null>): UseFullscreenReturn => {
    const [isFullscreen, setIsFullscreen] = useState(false);
    
    const enterFullscreen = async (element: HTMLElement): Promise<void> => {
        try {
            // Add fullscreen class for styling before requesting fullscreen
            element.classList.add('graph-fullscreen');
            
            // Request fullscreen with cross-browser compatibility
            if (element.requestFullscreen) {
                await element.requestFullscreen();
            } else if ((element as any).webkitRequestFullscreen) {
                await (element as any).webkitRequestFullscreen();
            } else if ((element as any).mozRequestFullScreen) {
                await (element as any).mozRequestFullScreen();
            } else if ((element as any).msRequestFullscreen) {
                await (element as any).msRequestFullscreen();
            } else {
                console.warn('Fullscreen API not supported by this browser');
                element.classList.remove('graph-fullscreen');
                return;
            }
        } catch (error) {
            console.error('Error entering fullscreen:', error);
            element.classList.remove('graph-fullscreen');
        }
    };

    const exitFullscreen = async (): Promise<void> => {
        try {
            // Exit fullscreen with cross-browser compatibility
            if (document.exitFullscreen) {
                await document.exitFullscreen();
            } else if ((document as any).webkitExitFullscreen) {
                await (document as any).webkitExitFullscreen();
            } else if ((document as any).mozCancelFullScreen) {
                await (document as any).mozCancelFullScreen();
            } else if ((document as any).msExitFullscreen) {
                await (document as any).msExitFullscreen();
            }
        } catch (error) {
            console.error('Error exiting fullscreen:', error);
        }
    };
    
    const toggleFullscreen = async (): Promise<void> => {
        if (!elementRef.current) {
            console.warn('Element ref is null, cannot toggle fullscreen');
            return;
        }
        
        if (isFullscreen) {
            await exitFullscreen();
        } else {
            await enterFullscreen(elementRef.current);
        }
    };

    const handleFullscreenChange = (): void => {
        const fullscreenElement = document.fullscreenElement || 
                                 (document as any).webkitFullscreenElement || 
                                 (document as any).mozFullScreenElement || 
                                 (document as any).msFullscreenElement;
        
        const isCurrentlyFullscreen = !!fullscreenElement && !!elementRef.current && fullscreenElement === elementRef.current;
        setIsFullscreen(isCurrentlyFullscreen);
        
        // Manage fullscreen class
        if (elementRef.current) {
            if (isCurrentlyFullscreen) {
                elementRef.current.classList.add('graph-fullscreen');
            } else {
                elementRef.current.classList.remove('graph-fullscreen');
            }
        }
    };

    useEffect(() => {
        // Listen for fullscreen change events (handles ESC key and other fullscreen exits)
        document.addEventListener('fullscreenchange', handleFullscreenChange);
        document.addEventListener('webkitfullscreenchange', handleFullscreenChange);
        document.addEventListener('mozfullscreenchange', handleFullscreenChange);
        document.addEventListener('MSFullscreenChange', handleFullscreenChange);

        return () => {
            document.removeEventListener('fullscreenchange', handleFullscreenChange);
            document.removeEventListener('webkitfullscreenchange', handleFullscreenChange);
            document.removeEventListener('mozfullscreenchange', handleFullscreenChange);
            document.removeEventListener('MSFullscreenChange', handleFullscreenChange);
        };
    }, []);

    return {
        isFullscreen,
        toggleFullscreen
    };
};