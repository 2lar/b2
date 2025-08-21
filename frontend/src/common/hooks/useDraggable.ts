/**
 * useDraggable Hook - Draggable Element Functionality
 * 
 * Purpose:
 * Provides draggable functionality for UI elements with position persistence
 * and boundary constraints. Works on all screen sizes with viewport boundary
 * protection.
 * 
 * Key Features:
 * - Mouse and touch-based dragging with smooth movement
 * - Keyboard navigation support
 * - Position maintained during session (resets on page reload)
 * - Viewport boundary constraints
 * - Works on all screen sizes
 * - Reset functionality for default positioning
 * - Smooth transitions and visual feedback
 * - Debounced resize handling
 * 
 * Usage:
 * - Call hook with element ref and storage key
 * - Apply returned position to element transform
 * - Attach drag handlers to drag handle element
 * - Use isDragging for visual feedback
 */

import { useState, useEffect, useCallback, useRef, RefObject, useMemo } from 'react';
import { DRAGGABLE_CONSTANTS, KEYBOARD_KEYS } from '../constants/draggable';

interface Position {
    x: number;
    y: number;
}

interface DraggableOptions {
    /** Storage key for sessionStorage persistence */
    storageKey: string;
    /** Default position when no stored position exists */
    defaultPosition: Position;
    /** Whether dragging is enabled (default: true) */
    enabled?: boolean;
    /** Minimum distance from viewport edges */
    boundaryPadding?: number;
    /** Enable keyboard navigation (default: true) */
    enableKeyboard?: boolean;
    /** Enable touch support (default: true) */
    enableTouch?: boolean;
    /** Callback when position changes */
    onPositionChange?: (position: Position) => void;
}

interface DraggableResult {
    /** Current position of the element */
    position: Position;
    /** Whether the element is currently being dragged */
    isDragging: boolean;
    /** Handler for mouse down event on drag handle */
    onMouseDown: (event: React.MouseEvent) => void;
    /** Handler for touch start event on drag handle */
    onTouchStart: (event: React.TouchEvent) => void;
    /** Handler for keyboard events */
    onKeyDown: (event: React.KeyboardEvent) => void;
    /** Reset position to default */
    resetPosition: () => void;
    /** Manually set position */
    setPosition: (position: Position) => void;
}

export function useDraggable(
    elementRef: RefObject<HTMLElement | null>,
    options: DraggableOptions
): DraggableResult {
    const {
        storageKey,
        defaultPosition,
        enabled = true,
        boundaryPadding = DRAGGABLE_CONSTANTS.BOUNDARY_PADDING,
        enableKeyboard = true,
        enableTouch = true,
        onPositionChange
    } = options;

    // Always start with default position on page load
    const [position, setPositionState] = useState<Position>(defaultPosition);
    const [isDragging, setIsDragging] = useState(false);
    
    // Use refs to avoid stale closures
    const dragStateRef = useRef<{
        startX: number;
        startY: number;
        startElementX: number;
        startElementY: number;
        elementWidth: number;
        elementHeight: number;
        dragType: 'mouse' | 'touch';
    } | null>(null);
    
    const rafIdRef = useRef<number | null>(null);
    const currentPointRef = useRef<{ x: number; y: number }>({ x: 0, y: 0 });
    const resizeTimeoutRef = useRef<NodeJS.Timeout | null>(null);

    // Clear sessionStorage on mount to ensure fresh start on page reload
    useEffect(() => {
        try {
            sessionStorage.removeItem(storageKey);
        } catch (error) {
            console.warn('Failed to clear sessionStorage:', error);
        }
    }, []); // Empty deps - only run once on mount

    // Save position to sessionStorage and notify callback
    const savePosition = useCallback((pos: Position) => {
        try {
            sessionStorage.setItem(storageKey, JSON.stringify(pos));
            onPositionChange?.(pos);
        } catch (error) {
            console.warn('Failed to save position to sessionStorage:', error);
        }
    }, [storageKey, onPositionChange]);

    // Constrain position to viewport boundaries
    const constrainPosition = useCallback((pos: Position, elementWidth: number, elementHeight: number): Position => {
        const viewportWidth = window.innerWidth;
        const viewportHeight = window.innerHeight;

        const minX = boundaryPadding;
        const minY = boundaryPadding;
        const maxX = viewportWidth - elementWidth - boundaryPadding;
        const maxY = viewportHeight - elementHeight - boundaryPadding;

        return {
            x: Math.max(minX, Math.min(maxX, pos.x)),
            y: Math.max(minY, Math.min(maxY, pos.y))
        };
    }, [boundaryPadding]);

    // Update position during drag using RAF
    const updateDragPosition = useCallback(() => {
        if (!isDragging || !dragStateRef.current) return;

        const drag = dragStateRef.current;
        const point = currentPointRef.current;

        const deltaX = point.x - drag.startX;
        const deltaY = point.y - drag.startY;

        const newPosition = {
            x: drag.startElementX + deltaX,
            y: drag.startElementY + deltaY
        };

        const constrainedPosition = constrainPosition(newPosition, drag.elementWidth, drag.elementHeight);
        setPositionState(constrainedPosition);

        rafIdRef.current = requestAnimationFrame(updateDragPosition);
    }, [isDragging, constrainPosition]);

    // Handle pointer move during drag (mouse or touch)
    const handlePointerMove = useCallback((event: MouseEvent | TouchEvent) => {
        if (!isDragging) return;
        
        const point = 'touches' in event ? event.touches[0] : event;
        currentPointRef.current = {
            x: point.clientX,
            y: point.clientY
        };

        // Start RAF loop if not already running
        if (!rafIdRef.current) {
            rafIdRef.current = requestAnimationFrame(updateDragPosition);
        }
    }, [isDragging, updateDragPosition]);

    // Handle pointer up (end drag)
    const handlePointerUp = useCallback(() => {
        if (!isDragging) return;

        setIsDragging(false);
        dragStateRef.current = null;
        
        // Cancel any pending RAF
        if (rafIdRef.current) {
            cancelAnimationFrame(rafIdRef.current);
            rafIdRef.current = null;
        }

        // Save final position
        savePosition(position);
    }, [isDragging, position, savePosition]);

    // Handle mouse down on drag handle
    const onMouseDown = useCallback((event: React.MouseEvent) => {
        if (!enabled || !elementRef.current) return;

        event.preventDefault();
        event.stopPropagation();

        const element = elementRef.current;
        const rect = element.getBoundingClientRect();
        
        // Get the actual current position from the element's transform
        // This ensures we start dragging from where the element actually is
        let currentX = position.x;
        let currentY = position.y;
        
        try {
            const computedStyle = window.getComputedStyle(element);
            const transform = computedStyle.transform;
            
            if (transform && transform !== 'none') {
                const matrix = new DOMMatrix(transform);
                currentX = matrix.m41;
                currentY = matrix.m42;
            }
        } catch (e) {
            // Fallback to stored position if transform parsing fails
            console.warn('Failed to parse transform, using stored position', e);
        }

        // Store drag state with actual current position
        dragStateRef.current = {
            startX: event.clientX,
            startY: event.clientY,
            startElementX: currentX,
            startElementY: currentY,
            elementWidth: rect.width,
            elementHeight: rect.height,
            dragType: 'mouse'
        };

        currentPointRef.current = {
            x: event.clientX,
            y: event.clientY
        };

        setIsDragging(true);
    }, [enabled, position]);

    // Reset to default position
    const resetPosition = useCallback(() => {
        setPositionState(defaultPosition);
        savePosition(defaultPosition);
    }, [defaultPosition, savePosition]);

    // Handle touch start
    const onTouchStart = useCallback((event: React.TouchEvent) => {
        if (!enabled || !enableTouch || !elementRef.current) return;

        event.preventDefault();
        event.stopPropagation();

        const element = elementRef.current;
        const rect = element.getBoundingClientRect();
        const touch = event.touches[0];
        
        let currentX = position.x;
        let currentY = position.y;
        
        try {
            const computedStyle = window.getComputedStyle(element);
            const transform = computedStyle.transform;
            
            if (transform && transform !== 'none') {
                const matrix = new DOMMatrix(transform);
                currentX = matrix.m41;
                currentY = matrix.m42;
            }
        } catch (e) {
            console.warn('Failed to parse transform, using stored position', e);
        }

        dragStateRef.current = {
            startX: touch.clientX,
            startY: touch.clientY,
            startElementX: currentX,
            startElementY: currentY,
            elementWidth: rect.width,
            elementHeight: rect.height,
            dragType: 'touch'
        };

        currentPointRef.current = {
            x: touch.clientX,
            y: touch.clientY
        };

        setIsDragging(true);
    }, [enabled, enableTouch, position]);

    // Handle keyboard navigation
    const onKeyDown = useCallback((event: React.KeyboardEvent) => {
        if (!enabled || !enableKeyboard || !elementRef.current) return;

        const step = event.shiftKey ? DRAGGABLE_CONSTANTS.KEYBOARD_MOVE_STEP_LARGE : DRAGGABLE_CONSTANTS.KEYBOARD_MOVE_STEP;
        let newPosition = { ...position };
        let shouldUpdate = false;

        switch (event.key) {
            case KEYBOARD_KEYS.ARROW_UP:
                newPosition.y -= step;
                shouldUpdate = true;
                break;
            case KEYBOARD_KEYS.ARROW_DOWN:
                newPosition.y += step;
                shouldUpdate = true;
                break;
            case KEYBOARD_KEYS.ARROW_LEFT:
                newPosition.x -= step;
                shouldUpdate = true;
                break;
            case KEYBOARD_KEYS.ARROW_RIGHT:
                newPosition.x += step;
                shouldUpdate = true;
                break;
            case KEYBOARD_KEYS.ESCAPE:
                resetPosition();
                return;
            default:
                return;
        }

        if (shouldUpdate) {
            event.preventDefault();
            const element = elementRef.current;
            const rect = element.getBoundingClientRect();
            const constrainedPosition = constrainPosition(newPosition, rect.width, rect.height);
            setPositionState(constrainedPosition);
            savePosition(constrainedPosition);
        }
    }, [enabled, enableKeyboard, position, constrainPosition, savePosition, resetPosition]);

    // Set up global event listeners during drag
    useEffect(() => {
        if (!enabled || !isDragging || !dragStateRef.current) return;

        const dragType = dragStateRef.current.dragType;

        if (dragType === 'mouse') {
            document.addEventListener('mousemove', handlePointerMove);
            document.addEventListener('mouseup', handlePointerUp);
            document.addEventListener('mouseleave', handlePointerUp);
        } else if (dragType === 'touch') {
            document.addEventListener('touchmove', handlePointerMove, { passive: false });
            document.addEventListener('touchend', handlePointerUp);
            document.addEventListener('touchcancel', handlePointerUp);
        }

        // Add cursor style to body during drag
        document.body.style.cursor = 'move';
        document.body.style.userSelect = 'none';
        document.body.style.webkitUserSelect = 'none';

        return () => {
            if (dragType === 'mouse') {
                document.removeEventListener('mousemove', handlePointerMove);
                document.removeEventListener('mouseup', handlePointerUp);
                document.removeEventListener('mouseleave', handlePointerUp);
            } else if (dragType === 'touch') {
                document.removeEventListener('touchmove', handlePointerMove);
                document.removeEventListener('touchend', handlePointerUp);
                document.removeEventListener('touchcancel', handlePointerUp);
            }
            
            // Cancel any pending RAF
            if (rafIdRef.current) {
                cancelAnimationFrame(rafIdRef.current);
                rafIdRef.current = null;
            }
            
            // Restore cursor and user select
            document.body.style.cursor = '';
            document.body.style.userSelect = '';
            document.body.style.webkitUserSelect = '';
        };
    }, [enabled, isDragging, handlePointerMove, handlePointerUp]);

    // Set position with constraint checking and persistence
    const setPosition = useCallback((newPosition: Position) => {
        if (!elementRef.current) {
            setPositionState(newPosition);
            savePosition(newPosition);
            return;
        }

        const element = elementRef.current;
        const rect = element.getBoundingClientRect();
        const constrainedPosition = constrainPosition(newPosition, rect.width, rect.height);
        setPositionState(constrainedPosition);
        savePosition(constrainedPosition);
    }, [constrainPosition, savePosition]);

    // Debounced resize handler
    const debouncedHandleResize = useMemo(() => {
        return () => {
            if (resizeTimeoutRef.current) {
                clearTimeout(resizeTimeoutRef.current);
            }

            resizeTimeoutRef.current = setTimeout(() => {
                if (!elementRef.current) return;
                
                const element = elementRef.current;
                const rect = element.getBoundingClientRect();
                
                setPositionState(currentPos => {
                    const constrainedPosition = constrainPosition(currentPos, rect.width, rect.height);
                    
                    if (constrainedPosition.x !== currentPos.x || constrainedPosition.y !== currentPos.y) {
                        savePosition(constrainedPosition);
                        return constrainedPosition;
                    }
                    
                    return currentPos;
                });
            }, 150);
        };
    }, [constrainPosition, savePosition]);

    // Handle window resize to re-constrain position
    useEffect(() => {
        if (!enabled) return;

        window.addEventListener('resize', debouncedHandleResize);
        return () => {
            window.removeEventListener('resize', debouncedHandleResize);
            if (resizeTimeoutRef.current) {
                clearTimeout(resizeTimeoutRef.current);
            }
        };
    }, [enabled, debouncedHandleResize]);

    return {
        position,
        isDragging,
        onMouseDown,
        onTouchStart,
        onKeyDown,
        resetPosition,
        setPosition
    };
}