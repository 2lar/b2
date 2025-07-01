/**
 * Brain2 Frontend Application - Main Controller
 * 
 * This is the main application controller that orchestrates the entire frontend experience.
 * It serves as the central hub connecting authentication, API communication, real-time updates,
 * and graph visualization.
 * 
 * KEY ARCHITECTURAL CONCEPTS:
 * 1. Event Delegation Pattern: Uses a single event listener on containers to handle multiple child events
 * 2. Modular Imports: Dynamically imports heavy modules (graph-viz) only when needed
 * 3. State Management: Manages UI state transitions between authentication and main app
 * 4. Real-time Communication: Integrates WebSocket for live graph updates
 * 5. Type Safety: Uses OpenAPI-generated types for consistent data contracts
 * 
 * LEARNING OBJECTIVES:
 * - Understanding modern frontend architecture patterns
 * - Event delegation for performance optimization
 * - Dynamic imports for code splitting
 * - WebSocket integration for real-time features
 * - Type-safe API communication
 */

// Import our custom authentication wrapper around Supabase
import { auth } from './authClient';
// Import our API client that handles all backend communication
import { api } from './apiClient';
// Import OpenAPI-generated TypeScript types for type safety across frontend/backend
import { components } from './generated-types';
// Import WebSocket client for real-time graph updates
import { webSocketClient } from './webSocketClient';
// Import Supabase types for authentication session handling
import { Session } from '@supabase/supabase-js';

/**
 * Type alias for the Node schema from our OpenAPI specification.
 * This ensures we use the same data structure that our backend expects,
 * providing compile-time type checking and better developer experience.
 */
type Node = components['schemas']['Node'];

/**
 * Feature Flags for Progressive Enhancement
 * 
 * This allows us to easily enable/disable features during development or deployment.
 * Graph visualization is resource-intensive, so we can disable it for:
 * - Performance testing
 * - Mobile devices
 * - Users with slow connections
 * - Debugging other features
 */
const ENABLE_GRAPH_VISUALIZATION = true;

/**
 * DOM Element References with Type Safety
 * 
 * TypeScript doesn't know about our HTML structure, so we explicitly cast elements
 * to their specific types. This provides:
 * 1. Compile-time type checking (e.g., can't call .submit() on a div)
 * 2. Better IDE autocomplete and error detection
 * 3. Runtime safety (will throw if element doesn't exist)
 * 
 * WHY THIS PATTERN:
 * - Fails fast if HTML structure changes
 * - Provides type safety without runtime overhead
 * - Single source of truth for all DOM references
 */
const authSection = document.getElementById('auth-section') as HTMLElement;
const appSection = document.getElementById('app-section') as HTMLElement;
const userEmail = document.getElementById('user-email') as HTMLElement;
const signOutBtn = document.getElementById('sign-out-btn') as HTMLButtonElement;
const memoryForm = document.getElementById('memory-form') as HTMLFormElement;
const memoryContent = document.getElementById('memory-content') as HTMLTextAreaElement;
const memoryStatus = document.getElementById('memory-status') as HTMLElement;
const memoryList = document.getElementById('memory-list') as HTMLElement;
const refreshGraphBtn = document.getElementById('refresh-graph') as HTMLButtonElement;
const fitGraphBtn = document.getElementById('fit-graph') as HTMLButtonElement;

/**
 * Dynamic Module Loading for Performance Optimization
 * 
 * Instead of importing graph-viz at the top level, we import it dynamically when needed.
 * This technique is called "code splitting" and provides several benefits:
 * 
 * 1. FASTER INITIAL LOAD: The main bundle is smaller, so the app loads faster
 * 2. CONDITIONAL LOADING: Only load graph features if user actually needs them
 * 3. LAZY LOADING: Import heavy dependencies (Cytoscape.js) only when required
 * 4. BANDWIDTH SAVINGS: Users who don't use graphs don't download graph code
 * 
 * The type annotation ensures we maintain type safety even with dynamic imports.
 */
let graphViz: { initGraph: () => void; refreshGraph: () => Promise<void> } | null = null;

/**
 * Application Initialization - The Foundation of Our App
 * 
 * This function runs once when the page loads and sets up the entire application.
 * It demonstrates several important patterns:
 * 
 * 1. AUTHENTICATION-FIRST APPROACH: Check if user is already signed in
 * 2. EVENT DELEGATION: Set up a few strategic event listeners instead of many
 * 3. LAZY INITIALIZATION: Only set up features that are actually needed
 * 4. SINGLE SETUP: All event listeners are attached once to prevent duplicates
 * 
 * WHY ASYNC/AWAIT:
 * - Authentication check requires network request to Supabase
 * - Modern browsers handle async initialization gracefully
 * - Provides clean error handling for setup failures
 */
async function init(): Promise<void> {
    // Check if user is already authenticated (persistent session)
    const session: Session | null = await auth.getSession();
    if (session && session.user.email) {
        // User is already signed in, skip authentication screen
        showApp(session.user.email);
    }

    /**
     * EVENT LISTENER SETUP - The Event Delegation Pattern
     * 
     * Instead of attaching listeners to every button and form element,
     * we use event delegation to handle events efficiently:
     * 
     * BENEFITS:
     * - Performance: Fewer event listeners = less memory usage
     * - Dynamic content: Works with elements added later via JavaScript
     * - Maintainability: Centralized event handling logic
     * - Prevents memory leaks: No need to remove listeners when elements change
     */
    // All event listeners are attached only once during initialization.

    signOutBtn.addEventListener('click', handleSignOut);
    memoryForm.addEventListener('submit', handleMemorySubmit);

    /**
     * Event Delegation - The Core Performance Pattern
     * 
     * Instead of adding click listeners to every memory item (could be hundreds),
     * we add ONE listener to the container and determine what was clicked.
     * 
     * HOW IT WORKS:
     * 1. User clicks anywhere in the memory list
     * 2. Browser "bubbles" the event up to our container listener
     * 3. We check event.target to see what was actually clicked
     * 4. We route to the appropriate handler based on the clicked element
     * 
     * WHY THIS IS POWERFUL:
     * - Scales to thousands of memory items without performance issues
     * - Works with dynamically added content (no need to re-attach listeners)
     * - Centralizes all memory interaction logic in one place
     * - Prevents memory leaks from orphaned event listeners
     */
    memoryList.addEventListener('click', handleMemoryListClick);

    /**
     * Real-Time Graph Updates via Custom Events
     * 
     * This demonstrates a clean separation of concerns pattern:
     * 
     * FLOW:
     * 1. User creates/updates a memory
     * 2. Backend processes the memory and finds new connections
     * 3. Backend sends WebSocket message about new connections
     * 4. WebSocketClient receives message and dispatches custom DOM event
     * 5. This listener catches the event and refreshes the graph visualization
     * 
     * WHY CUSTOM EVENTS:
     * - Loose coupling: WebSocket client doesn't need to know about graph visualization
     * - Testability: Easy to simulate events for testing
     * - Extensibility: Other components can listen for the same event
     * - Browser native: Uses standard DOM event system
     */
    document.addEventListener('graph-update-event', async () => {
        console.log("Graph update event received in app.ts");
        if (graphViz) {
            showStatus('New connections found! Refreshing graph...', 'success');
            await graphViz.refreshGraph();
        }
    });

    /**
     * Enhanced UX: Submit Memory with Enter Key
     * 
     * This adds a common UX pattern where Enter submits the form,
     * but Shift+Enter allows multi-line input.
     * 
     * UX CONSIDERATIONS:
     * - Enter = Quick submission for short thoughts
     * - Shift+Enter = Multi-line editing for longer content
     * - Prevents accidental submissions while typing
     * - Matches behavior users expect from chat applications
     */
    memoryContent.addEventListener('keydown', (e: KeyboardEvent) => {
        if (e.key === 'Enter' && !e.shiftKey) {
            e.preventDefault();
            const submitButton = memoryForm.querySelector('button[type="submit"]') as HTMLButtonElement;
            if (submitButton) {
                submitButton.click();
            }
        }
    });
    
    /**
     * Conditional Feature Setup - Progressive Enhancement Pattern
     * 
     * This demonstrates how to build applications that gracefully handle
     * optional features. Graph visualization is complex and resource-intensive,
     * so we make it optional.
     * 
     * PROGRESSIVE ENHANCEMENT PRINCIPLES:
     * 1. Core functionality works without advanced features
     * 2. Advanced features enhance but don't break basic experience
     * 3. Graceful degradation for older browsers or limited resources
     * 4. Clear feature boundaries for testing and debugging
     */
    if (ENABLE_GRAPH_VISUALIZATION) {
        // Set up graph control event listeners
        // Optional chaining (?.) safely handles cases where graphViz isn't loaded yet
        refreshGraphBtn.addEventListener('click', () => graphViz?.refreshGraph())
        fitGraphBtn.addEventListener('click', () => {
            // window.cy is the global Cytoscape instance created by graph-viz module
            if (window.cy) {
                window.cy.fit(); // Zoom to fit all nodes in the viewport
            }
        });
    } else {
        /**
         * Feature Flag Cleanup
         * 
         * When features are disabled, we hide related UI elements to avoid
         * confusing users with non-functional buttons.
         */
        const graphSection = document.querySelector('.graph-section') as HTMLElement | null;
        if (graphSection) graphSection.style.display = 'none';
        const nodeDetailsPanel = document.getElementById('node-details');
        if (nodeDetailsPanel) nodeDetailsPanel.style.display = 'none';
    }
}

/**
 * Application State Transition: Authentication → Main App
 * 
 * This function handles the critical transition from the authentication screen
 * to the main application interface. It demonstrates several key patterns:
 * 
 * 1. UI STATE MANAGEMENT: Clean transitions between app states
 * 2. LAZY LOADING: Only load heavy features when user is authenticated
 * 3. REAL-TIME SETUP: Establish WebSocket connection for live updates
 * 4. PROGRESSIVE ENHANCEMENT: Optional features loaded conditionally
 * 
 * EXECUTION ORDER MATTERS:
 * - Hide auth UI first (immediate visual feedback)
 * - Show main UI (user sees app loading)
 * - Connect WebSocket (background real-time setup)
 * - Load memories (populate UI with data)
 * - Load graph visualization (heavy lifting happens last)
 */
async function showApp(email: string): Promise<void> {
    // Immediate UI state transition - user sees the change instantly
    authSection.style.display = 'none';
    appSection.style.display = 'block';
    userEmail.textContent = email;

    /**
     * WebSocket Connection Setup
     * 
     * We connect the WebSocket early so we can receive real-time updates
     * while the rest of the app loads. This ensures users get live updates
     * as soon as possible.
     */
    webSocketClient.connect();

    // Load user's existing memories from the backend
    await loadMemories();
    
    /**
     * Conditional Graph Visualization Loading
     * 
     * This is where the magic of dynamic imports happens:
     * 1. Only users who need graphs pay the performance cost
     * 2. Cytoscape.js is ~500KB - only loaded when needed
     * 3. If loading fails, core app functionality still works
     * 4. Perfect for A/B testing different visualization libraries
     */
    if (ENABLE_GRAPH_VISUALIZATION) {
        // Dynamic import - this triggers a separate network request
        graphViz = await import('./graph-viz');
        graphViz.initGraph();        // Initialize the graph container
        await graphViz.refreshGraph(); // Load and display user's graph data
    }
}

/**
 * Clean Application Shutdown - Security and Performance
 * 
 * Sign-out is more than just hiding the UI - it's about security and cleanup:
 * 
 * 1. SECURITY: Clear sensitive data from memory and UI
 * 2. PERFORMANCE: Close connections to prevent resource leaks
 * 3. PRIVACY: Ensure next user can't see previous user's data
 * 4. UX: Smooth transition back to authentication state
 * 
 * ORDER OF OPERATIONS:
 * - Disconnect WebSocket first (stops incoming data)
 * - Sign out from auth provider (invalidates session)
 * - Clear UI state (remove sensitive data)
 * - Reset to authentication screen
 */
async function handleSignOut(): Promise<void> {
    /**
     * WebSocket Cleanup
     * 
     * Critical for preventing memory leaks and ensuring security.
     * If we don't disconnect, the WebSocket might continue receiving
     * messages for the signed-out user.
     */
    webSocketClient.disconnect();

    // Sign out from Supabase - this invalidates the JWT token
    await auth.signOut();
    
    // UI state reset - show auth screen, hide main app
    authSection.style.display = 'flex';
    appSection.style.display = 'none';
    userEmail.textContent = '';
    
    /**
     * Data Cleanup
     * 
     * Clear any sensitive data from the DOM to prevent the next user
     * from seeing the previous user's memories.
     */
    memoryList.innerHTML = '';
}

/**
 * Memory Creation - The Core User Interaction
 * 
 * This function handles the primary user action: creating new memories.
 * It demonstrates several important patterns for form handling:
 * 
 * 1. FORM VALIDATION: Basic client-side validation before API call
 * 2. UI FEEDBACK: Immediate visual feedback during async operations
 * 3. ERROR HANDLING: Graceful handling of network and server errors
 * 4. UX OPTIMIZATION: Disable form during submission to prevent double-submission
 * 5. REAL-TIME UPDATES: Integration with WebSocket for live graph updates
 * 
 * THE ASYNC FLOW:
 * User types → Form submission → API call → UI update → Real-time notification
 */
async function handleMemorySubmit(e: Event): Promise<void> {
    // Prevent default form submission (which would reload the page)
    e.preventDefault();
    
    // Basic client-side validation
    const content = memoryContent.value.trim();
    if (!content) return; // Don't submit empty memories

    /**
     * UI State Management During Async Operations
     * 
     * We disable the form elements to provide immediate feedback
     * and prevent users from submitting multiple times.
     */
    memoryContent.disabled = true;
    (memoryForm.querySelector('button') as HTMLButtonElement).disabled = true;

    try {
        // API call to create new memory node
        await api.createNode(content);
        
        // Success feedback
        showStatus('Memory saved successfully!', 'success');
        memoryContent.value = ''; // Clear the form
        await loadMemories();     // Refresh the memory list

        /**
         * Real-Time Graph Updates
         * 
         * Note: We used to manually refresh the graph here, but now it's handled
         * automatically via WebSocket events. This is better because:
         * - Graph only updates when there are actual new connections
         * - Other users' memories can trigger updates too
         * - Reduces unnecessary API calls
         * - Provides consistent real-time experience
         */
        // The graph refresh is now handled by the WebSocket event
        // if (ENABLE_GRAPH_VISUALIZATION && graphViz) {
        //     await graphViz.refreshGraph();
        // }
    } catch (error) {
        /**
         * Error Handling Best Practices
         * 
         * 1. User-friendly message (no technical details)
         * 2. Console logging for debugging
         * 3. Don't clear the form on error (user can retry)
         */
        showStatus('Failed to save memory. Please try again.', 'error');
        console.error('Error creating memory:', error);
    } finally {
        /**
         * Cleanup - Always Runs
         * 
         * The finally block ensures the form is re-enabled regardless
         * of success or failure. This prevents the UI from getting stuck
         * in a disabled state.
         */
        memoryContent.disabled = false;
        (memoryForm.querySelector('button') as HTMLButtonElement).disabled = false;
        memoryContent.focus(); // Return focus for better UX
    }
}

/**
 * Event Delegation Masterclass - Single Function, Multiple Responsibilities
 * 
 * This function is the heart of our event delegation pattern. It handles ALL
 * click events within the memory list, routing them to appropriate actions.
 * 
 * WHY THIS APPROACH IS POWERFUL:
 * 
 * 1. PERFORMANCE: One event listener instead of hundreds
 *    - With 1000 memories: 1 listener vs 3000+ listeners (edit, delete, checkbox per item)
 *    - Massive reduction in memory usage and event registration overhead
 * 
 * 2. DYNAMIC CONTENT: Works with memories added/removed after page load
 *    - No need to re-attach listeners when content changes
 *    - Automatically handles new memories from real-time updates
 * 
 * 3. MAINTAINABILITY: All memory interactions in one place
 *    - Easy to add new actions (just add another condition)
 *    - Centralized logic for memory item interactions
 * 
 * 4. FLEXIBILITY: Can handle complex interaction patterns
 *    - Multi-select with checkboxes
 *    - Bulk operations on selected items
 *    - Graph integration (clicking memory highlights node)
 * 
 * THE ROUTING PATTERN:
 * Click event → Identify target → Route to appropriate handler → Execute action
 * 
 * @param e The mouse click event bubbled up from child elements
 */
async function handleMemoryListClick(e: MouseEvent): Promise<void> {
    /**
     * Event Target Identification
     * 
     * event.target is the actual element that was clicked, even if it's
     * deeply nested inside our memory list container.
     */
    const target = e.target as HTMLElement;

    /**
     * Multi-Select Functionality - Bulk Operations
     * 
     * These handlers manage the checkbox-based multi-select system:
     * - Select all: Toggle all memory checkboxes at once
     * - Individual checkboxes: Update selection state and UI
     * - Bulk delete: Operate on all selected memories
     * 
     * UX PATTERN: Gmail-style bulk operations
     */
    
    // Handle the "select all" checkbox in the header
    if (target.matches('#select-all-checkbox')) {
        handleSelectAllToggle();
        return;
    }

    // Handle individual memory item checkboxes
    if (target.matches('.memory-checkbox')) {
        handleMemoryCheckboxChange();
        return;
    }

    // Handle bulk delete button (operates on all selected memories)
    if (target.matches('#bulk-delete-btn')) {
        await handleBulkDelete();
        return;
    }

    /**
     * DOM Traversal for Action Identification
     * 
     * Since users might click on text inside buttons or icons within buttons,
     * we use .closest() to find the actual actionable element.
     * 
     * HOW .closest() WORKS:
     * - Starts with the clicked element
     * - Walks up the DOM tree until it finds a matching element
     * - Returns null if no match found
     * 
     * EXAMPLE: User clicks on delete icon → .closest('.delete-btn') finds button
     */
    const deleteButton = target.closest('.delete-btn');
    const editButton = target.closest('.edit-btn');
    const saveButton = target.closest('.save-btn');
    const cancelButton = target.closest('.cancel-btn');
    const memoryItem = target.closest('.memory-item') as HTMLElement | null;

    // If click wasn't inside a memory item, ignore it
    if (!memoryItem) return;

    /**
     * Data Attributes for State Management
     * 
     * We store the nodeId in a data attribute on each memory item.
     * This allows us to identify which memory to operate on without
     * complex DOM traversal or maintaining a separate data structure.
     */
    const nodeId = memoryItem.dataset.nodeId;
    if (!nodeId) return;

    /**
     * DELETE Action - Destructive Operation with Confirmation
     * 
     * Deletion is irreversible, so we implement multiple safety measures:
     * 1. User confirmation dialog
     * 2. Try-catch error handling
     * 3. User feedback for success/failure
     * 4. Graph refresh to maintain consistency
     * 
     * UX CONSIDERATIONS:
     * - Clear warning about irreversibility
     * - Immediate feedback after action
     * - Graceful error handling
     */
    if (deleteButton) {
        // Safety confirmation - critical for destructive actions
        if (confirm('Are you sure you want to delete this memory? This cannot be undone.')) {
            try {
                // API call to delete the memory
                await api.deleteNode(nodeId);
                showStatus('Memory deleted.', 'success');
                
                // Update UI to reflect the change
                await loadMemories();
                
                // Update graph visualization if enabled
                if (ENABLE_GRAPH_VISUALIZATION && graphViz) {
                    await graphViz.refreshGraph();
                }
            } catch (error) {
                console.error('Failed to delete memory:', error);
                showStatus('Failed to delete memory.', 'error');
            }
        }
        return;
    }

    /**
     * EDIT Action - Inline Editing Pattern
     * 
     * This implements a common UX pattern where users can edit content
     * directly in place without navigating to a separate edit page.
     * 
     * THE TRANSFORMATION:
     * 1. Replace display content with editable textarea
     * 2. Replace action buttons with save/cancel options
     * 3. Focus the textarea for immediate editing
     * 4. Preserve original content for cancellation
     * 
     * UX BENEFITS:
     * - No page navigation required
     * - Context is preserved (user sees other memories)
     * - Quick editing for small changes
     * - Clear save/cancel options
     */
    if (editButton) {
        // Find the content and actions containers
        const contentDiv = memoryItem.querySelector('.memory-item-content') as HTMLElement;
        const actionsDiv = memoryItem.querySelector('.memory-item-actions') as HTMLElement;
        
        // Preserve the original content for potential cancellation
        const originalContent = contentDiv.textContent || '';
        
        /**
         * DOM Transformation: Display → Edit Mode
         * 
         * We replace the static content with an editable textarea
         * and style it to fit naturally within the memory item.
         */
        contentDiv.innerHTML = `<textarea class="edit-textarea">${originalContent}</textarea>`;
        const textarea = contentDiv.querySelector('.edit-textarea') as HTMLTextAreaElement;
        
        // Style the textarea to match the container
        textarea.style.width = '100%';
        textarea.style.minHeight = '80px';
        textarea.focus(); // Immediate focus for better UX

        /**
         * Action Button Transformation
         * 
         * Replace edit/delete buttons with save/cancel options.
         * Note: These new buttons will be handled by the same event
         * delegation system when clicked.
         */
        actionsDiv.innerHTML = `
            <button class="primary-btn save-btn">Save</button>
            <button class="secondary-btn cancel-btn">Cancel</button>
        `;
        return;
    }

    // --- Handle SAVE action (after editing) ---
    if (saveButton) {
        const textarea = memoryItem.querySelector('.edit-textarea') as HTMLTextAreaElement;
        const newContent = textarea.value.trim();

        if (newContent) {
            try {
                await api.updateNode(nodeId, newContent);
                showStatus('Memory updated!', 'success');
                // No need to call loadMemories() here, it will be handled by the cancel logic
            } catch (error) {
                console.error('Failed to update memory:', error);
                showStatus('Failed to update memory.', 'error');
            }
        }
        // Whether successful or not, restore the original view
        await loadMemories();
        if (ENABLE_GRAPH_VISUALIZATION && graphViz) await graphViz.refreshGraph();
        return;
    }

    // --- Handle CANCEL action (after editing) ---
    if (cancelButton) {
        await loadMemories(); // Simply reload to discard changes
        return;
    }
    
    // --- Handle clicking the memory item itself for graph interaction ---
    // Only trigger if not clicking on a checkbox or its container
    if (!target.closest('.checkbox-container') && 
        !target.matches('.memory-checkbox') && 
        ENABLE_GRAPH_VISUALIZATION && window.cy) {
        const node = window.cy.getElementById(nodeId);
        if (node?.length) {
            node.trigger('tap');
            window.cy.animate({
                center: { eles: node },
                zoom: 1.2,
                duration: 400
            });
        }
    }
}


// Load and display all memories
async function loadMemories(): Promise<void> {
    try {
        const data = await api.listNodes();
        displayMemories(data.nodes || []);
    } catch (error) {
        console.error('Error loading memories:', error);
        memoryList.innerHTML = '<p class="error-message">Failed to load memories</p>';
    }
}

// Render the list of memories to the DOM.
// This function is now ONLY responsible for rendering HTML. It does not handle events.
function displayMemories(nodes: Node[]): void {
    if (nodes.length === 0) {
        memoryList.innerHTML = '<p class="empty-state">No memories yet. Create your first memory above!</p>';
        return;
    }

    nodes.sort((a, b) => new Date(b.timestamp || '').getTime() - new Date(a.timestamp || '').getTime());

    // Create multi-select controls header
    const multiSelectHeader = `
        <div class="memory-list-controls">
            <div class="select-controls">
                <label class="checkbox-container">
                    <input type="checkbox" id="select-all-checkbox" class="select-all-checkbox">
                    <span class="checkmark"></span>
                    Select All
                </label>
                <span class="selected-count" id="selected-count">0 selected</span>
            </div>
            <div class="bulk-actions">
                <button class="danger-btn bulk-delete-btn" id="bulk-delete-btn" disabled>Delete Selected</button>
            </div>
        </div>
    `;

    const memoriesHtml = nodes.map(node => `
        <div class="memory-item" data-node-id="${node.nodeId || ''}">
            <div class="memory-item-header">
                <label class="checkbox-container">
                    <input type="checkbox" class="memory-checkbox" data-node-id="${node.nodeId || ''}">
                    <span class="checkmark"></span>
                </label>
                <div class="memory-item-content">${escapeHtml(node.content || '')}</div>
            </div>
            <div class="memory-item-meta">
                ${formatDate(node.timestamp || '')}
            </div>
            <div class="memory-item-actions">
                <button class="secondary-btn edit-btn">Edit</button>
                <button class="danger-btn delete-btn">Delete</button>
            </div>
        </div>
    `).join('');

    memoryList.innerHTML = multiSelectHeader + memoriesHtml;
}

// Show a temporary status message
function showStatus(message: string, type: 'success' | 'error'): void {
    memoryStatus.textContent = message;
    memoryStatus.className = `status-message ${type}`;

    setTimeout(() => {
        memoryStatus.textContent = '';
        memoryStatus.className = 'status-message';
    }, 3000);
}

// Utility functions
function escapeHtml(text: string): string {
    const div = document.createElement('div');
    div.textContent = text;
    return div.innerHTML;
}

function formatDate(dateString: string): string {
    const date = new Date(dateString);
    const now = new Date();
    const diffMs = now.getTime() - date.getTime();
    const diffMins = Math.round(diffMs / 60000);

    if (diffMins < 1) return 'Just now';
    if (diffMins < 60) return `${diffMins} minute${diffMins > 1 ? 's' : ''} ago`;
    const diffHours = Math.round(diffMins / 60);
    if (diffHours < 24) return `${diffHours} hour${diffHours > 1 ? 's' : ''} ago`;
    const diffDays = Math.round(diffHours / 24);
    if (diffDays < 7) return `${diffDays} day${diffDays > 1 ? 's' : ''} ago`;
    
    return date.toLocaleDateString();
}

// Multi-select functionality
function handleSelectAllToggle(): void {
    const selectAllCheckbox = document.getElementById('select-all-checkbox') as HTMLInputElement;
    const memoryCheckboxes = document.querySelectorAll('.memory-checkbox') as NodeListOf<HTMLInputElement>;
    
    const isChecked = selectAllCheckbox.checked;
    
    memoryCheckboxes.forEach(checkbox => {
        checkbox.checked = isChecked;
    });
    
    updateSelectedCount();
    updateBulkDeleteButton();
}

function handleMemoryCheckboxChange(): void {
    updateSelectedCount();
    updateBulkDeleteButton();
    updateSelectAllCheckbox();
}

function updateSelectedCount(): void {
    const selectedCheckboxes = document.querySelectorAll('.memory-checkbox:checked');
    const selectedCountElement = document.getElementById('selected-count');
    if (selectedCountElement) {
        selectedCountElement.textContent = `${selectedCheckboxes.length} selected`;
    }
}

function updateBulkDeleteButton(): void {
    const selectedCheckboxes = document.querySelectorAll('.memory-checkbox:checked');
    const bulkDeleteButton = document.getElementById('bulk-delete-btn') as HTMLButtonElement;
    if (bulkDeleteButton) {
        bulkDeleteButton.disabled = selectedCheckboxes.length === 0;
    }
}

function updateSelectAllCheckbox(): void {
    const memoryCheckboxes = document.querySelectorAll('.memory-checkbox') as NodeListOf<HTMLInputElement>;
    const selectedCheckboxes = document.querySelectorAll('.memory-checkbox:checked');
    const selectAllCheckbox = document.getElementById('select-all-checkbox') as HTMLInputElement;
    
    if (selectAllCheckbox) {
        if (selectedCheckboxes.length === 0) {
            selectAllCheckbox.checked = false;
            selectAllCheckbox.indeterminate = false;
        } else if (selectedCheckboxes.length === memoryCheckboxes.length) {
            selectAllCheckbox.checked = true;
            selectAllCheckbox.indeterminate = false;
        } else {
            selectAllCheckbox.checked = false;
            selectAllCheckbox.indeterminate = true;
        }
    }
}

/**
 * Bulk Delete Operation - Complex Multi-Step Process
 * 
 * This function handles deleting multiple memories at once. It demonstrates:
 * 1. Data collection from UI state
 * 2. User confirmation for destructive actions
 * 3. Batch API operations
 * 4. Partial success/failure handling
 * 5. UI state updates after operations
 * 
 * COMPLEXITY FACTORS:
 * - Some deletions might succeed while others fail
 * - Network errors during batch operations
 * - User feedback for partial success scenarios
 * - Graph visualization synchronization
 */
async function handleBulkDelete(): Promise<void> {
    /**
     * Data Collection Phase
     * 
     * Extract the IDs of all selected memories from the UI.
     * We filter out any undefined values to ensure data integrity.
     */
    const selectedCheckboxes = document.querySelectorAll('.memory-checkbox:checked') as NodeListOf<HTMLInputElement>;
    const selectedNodeIds = Array.from(selectedCheckboxes)
        .map(checkbox => checkbox.dataset.nodeId)
        .filter(id => id) as string[];
    
    // Safety check - don't proceed if nothing is selected
    if (selectedNodeIds.length === 0) {
        return;
    }
    
    /**
     * User Confirmation - Context-Aware Messaging
     * 
     * The confirmation message adapts based on the number of items:
     * - Singular: "delete this memory"
     * - Plural: "delete X memories"
     * 
     * This makes the action clearer and less likely to be clicked accidentally.
     */
    const message = selectedNodeIds.length === 1 
        ? 'Are you sure you want to delete this memory? This cannot be undone.'
        : `Are you sure you want to delete ${selectedNodeIds.length} memories? This cannot be undone.`;
    
    if (!confirm(message)) {
        return; // User cancelled the operation
    }
    
    /**
     * Batch Operation Execution
     * 
     * The backend bulk delete API can handle partial failures,
     * so we need to handle different response scenarios.
     */
    try {
        const response = await api.bulkDeleteNodes(selectedNodeIds);
        
        /**
         * Success Feedback
         * 
         * Provide specific feedback about how many items were successfully deleted.
         * This is important for user confidence and understanding.
         */
        if (response.deletedCount && response.deletedCount > 0) {
            const successMessage = response.deletedCount === 1 
                ? 'Memory deleted successfully!'
                : `${response.deletedCount} memories deleted successfully!`;
            showStatus(successMessage, 'success');
        }
        
        /**
         * Partial Failure Handling
         * 
         * Some memories might fail to delete due to:
         * - Concurrent modifications by other users
         * - Permission changes
         * - Network issues during batch processing
         * 
         * We inform the user about failures without alarming them.
         */
        if (response.failedNodeIds && response.failedNodeIds.length > 0) {
            const failureMessage = response.failedNodeIds.length === 1
                ? 'Failed to delete 1 memory.'
                : `Failed to delete ${response.failedNodeIds.length} memories.`;
            showStatus(failureMessage, 'error');
        }
        
        /**
         * UI Synchronization
         * 
         * After any bulk operation, we need to:
         * 1. Refresh the memory list (remove deleted items)
         * 2. Update the graph visualization (remove deleted nodes)
         * 3. Reset selection state (handled automatically by loadMemories)
         */
        await loadMemories();
        if (ENABLE_GRAPH_VISUALIZATION && graphViz) {
            await graphViz.refreshGraph();
        }
    } catch (error) {
        /**
         * Complete Failure Handling
         * 
         * If the entire batch operation fails (network error, server error),
         * we provide a generic error message and log details for debugging.
         */
        console.error('Failed to bulk delete memories:', error);
        showStatus('Failed to delete selected memories.', 'error');
    }
}

/**
 * ======================
 * GLOBAL INTEGRATION & INITIALIZATION
 * ======================
 */

/**
 * Global API Exposure
 * 
 * We expose the showApp function to the global scope so that the authentication
 * module (auth.ts) can call it after successful login. This creates a clean
 * separation between authentication and application logic.
 * 
 * WHY GLOBAL SCOPE:
 * - Auth module needs to trigger app display after login
 * - Avoids circular dependencies between modules
 * - Simple integration point between separate concerns
 * 
 * ALTERNATIVES WE COULD USE:
 * - Custom events (more decoupled but more complex)
 * - Module imports (creates circular dependency)
 * - Callback passing (requires more complex initialization)
 */
window.showApp = showApp;

/**
 * Application Bootstrap
 * 
 * This event listener starts the entire application when the DOM is ready.
 * DOMContentLoaded is the right event because:
 * 
 * 1. Fires after HTML is parsed (our elements exist)
 * 2. Doesn't wait for images/stylesheets (faster startup)
 * 3. Works consistently across all modern browsers
 * 4. Runs before the 'load' event (which waits for all resources)
 * 
 * This single line of code kicks off the entire application lifecycle!
 */
document.addEventListener('DOMContentLoaded', init);
