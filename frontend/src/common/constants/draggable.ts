export const DRAGGABLE_CONSTANTS = {
    BOUNDARY_PADDING: 20,
    DEFAULT_PANEL_WIDTH: 400,
    DEFAULT_PANEL_HEIGHT: 500,
    ANIMATION_DURATION: 200,
    DOUBLE_CLICK_DELAY: 300,
    MIN_DRAG_DISTANCE: 5,
    KEYBOARD_MOVE_STEP: 10,
    KEYBOARD_MOVE_STEP_LARGE: 50,
} as const;

export const KEYBOARD_KEYS = {
    ESCAPE: 'Escape',
    ENTER: 'Enter',
    SPACE: ' ',
    ARROW_UP: 'ArrowUp',
    ARROW_DOWN: 'ArrowDown',
    ARROW_LEFT: 'ArrowLeft',
    ARROW_RIGHT: 'ArrowRight',
    TAB: 'Tab',
} as const;

export const ARIA_LABELS = {
    CLOSE_PANEL: 'Close details panel',
    DRAG_HANDLE: 'Drag to move panel',
    PANEL_REGION: 'Memory details panel',
    CONNECTED_MEMORY: 'Navigate to connected memory',
} as const;