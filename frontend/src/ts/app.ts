/**
 * Brain2 Frontend Application - Main Controller
 * * Main application controller that orchestrates the frontend experience.
 * Connects authentication, API communication, real-time updates, and graph visualization.
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

// Type alias for the Node schema from OpenAPI specification
type Node = components['schemas']['Node'];
type NodeDetails = components['schemas']['NodeDetails'];

// Feature flag for graph visualization
const ENABLE_GRAPH_VISUALIZATION = true;

// DOM element references with type safety
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

// Dynamic module loading for graph visualization (code splitting)
let graphViz: {
    initGraph: () => void; 
    refreshGraph: () => Promise<void>;
    destroyGraph: () => void;
    addNodeAndAnimate: (nodeDetails: NodeDetails) => Promise<void>; 
} | null = null;

/**
 * Initialize the application
 * Sets up event listeners and checks for existing authentication session
 */
async function init(): Promise<void> {
    // Check if user is already authenticated (persistent session)
    const session: Session | null = await auth.getSession();
    if (session && session.user.email) {
        // User is already signed in, skip authentication screen
        showApp(session.user.email);
    }

    // Set up event listeners using event delegation pattern

    signOutBtn.addEventListener('click', handleSignOut);
    memoryForm.addEventListener('submit', handleMemorySubmit);

    // Event delegation for memory list interactions
    memoryList.addEventListener('click', handleMemoryListClick);

    // Listen for real-time graph updates via WebSocket
    document.addEventListener('graph-update-event', async (event: Event) => {
        const customEvent = event as CustomEvent;
        console.log("[Graph] Update event received in app.ts", customEvent.detail);
        
        if (!graphViz) {
            console.error("[Graph] graphViz is null - visualization not initialized");
            return;
        }
        
        if (!customEvent.detail?.nodeId) {
            console.error("[Graph] No nodeId in event detail", customEvent.detail);
            return;
        }

        showStatus('New connections found! Updating graph...', 'success');
        console.log("[Graph] Fetching details for node:", customEvent.detail.nodeId);
        
        const nodeDetails = await fetchNodeWithRetries(customEvent.detail.nodeId);

        if (nodeDetails) {
            console.log("[Graph] Node details fetched successfully:", nodeDetails);
            try {
                await graphViz.addNodeAndAnimate(nodeDetails);
                console.log("[Graph] Animation completed for node:", nodeDetails.nodeId);
            } catch (error) {
                console.error("[Graph] Error during animation:", error);
                await graphViz.refreshGraph();
            }
        } else {
            console.error(`[Graph] Failed to fetch details for node ${customEvent.detail.nodeId}`);
            await graphViz.refreshGraph();
        }
    });

    // Enhanced UX: Submit memory with Enter key (Shift+Enter for new line)
    memoryContent.addEventListener('keydown', (e: KeyboardEvent) => {
        if (e.key === 'Enter' && !e.shiftKey) {
            e.preventDefault();
            const submitButton = memoryForm.querySelector('button[type="submit"]') as HTMLButtonElement;
            if (submitButton) {
                submitButton.click();
            }
        }
    });
    
    // Set up graph visualization if enabled
    if (ENABLE_GRAPH_VISUALIZATION) {
        // Set up graph control event listeners
        refreshGraphBtn.addEventListener('click', () => graphViz?.refreshGraph())
        fitGraphBtn.addEventListener('click', () => {
            if (window.cy) {
                window.cy.fit();
            }
        });
    } else {
        // Hide graph UI when feature is disabled
        const graphSection = document.querySelector('.graph-section') as HTMLElement | null;
        if (graphSection) graphSection.style.display = 'none';
        const nodeDetailsPanel = document.getElementById('node-details');
        if (nodeDetailsPanel) nodeDetailsPanel.style.display = 'none';
    }
}

/**
 * Fetches node details with a retry mechanism to handle database consistency delays.
 * @param nodeId The ID of the node to fetch.
 * @returns The node details or null if not found after retries.
 */
async function fetchNodeWithRetries(nodeId: string): Promise<NodeDetails | null> {
    const maxRetries = 4;
    const retryDelay = 750; // ms

    for (let i = 0; i < maxRetries; i++) {
        try {
            console.log(`[Animation] Attempt ${i + 1} to fetch node ${nodeId}...`);
            const details = await api.getNode(nodeId);
            if (details && details.nodeId) {
                console.log(`[Animation] Successfully fetched node ${nodeId}.`);
                return details; // Success
            }
        } catch (error) {
            console.warn(`[Animation] Attempt ${i + 1} failed. Retrying in ${retryDelay}ms...`);
            if (i < maxRetries - 1) {
                await new Promise(resolve => setTimeout(resolve, retryDelay));
            }
        }
    }
    console.error(`[Animation] Failed to fetch details for node ${nodeId} after ${maxRetries} attempts.`);
    return null; // Failed after all retries
}


/**
 * Transition from authentication to main app
 * Handles UI state changes and initializes features
 */
async function showApp(email: string): Promise<void> {
    // UI state transition
    authSection.style.display = 'none';
    appSection.style.display = 'block';
    userEmail.textContent = email;

    // Connect WebSocket for real-time updates
    webSocketClient.connect();

    // Load user's existing memories
    await loadMemories();
    
    // Load graph visualization if enabled
    if (ENABLE_GRAPH_VISUALIZATION) {
        graphViz = await import('./graph-viz');
        graphViz.initGraph();
        await graphViz.refreshGraph();
    }
}

/**
 * Handle user sign out
 * Cleans up connections and resets UI state
 */
async function handleSignOut(): Promise<void> {
    // Disconnect WebSocket to prevent memory leaks
    webSocketClient.disconnect();

    // Destroy the graph instance to prevent data persistence between sessions
    if (ENABLE_GRAPH_VISUALIZATION && graphViz) {
        graphViz.destroyGraph();
    }

    // Sign out from Supabase
    await auth.signOut();
    
    // Reset UI state
    authSection.style.display = 'flex';
    appSection.style.display = 'none';
    userEmail.textContent = '';
    
    // Clear sensitive data from DOM
    memoryList.innerHTML = '';
    
    authSection.style.display = 'flex';
}

/**
 * Handle memory form submission
 * Creates new memory and updates UI
 */
async function handleMemorySubmit(e: Event): Promise<void> {
    e.preventDefault();
    
    const content = memoryContent.value.trim();
    if (!content) return;

    // Disable form during submission
    memoryContent.disabled = true;
    (memoryForm.querySelector('button') as HTMLButtonElement).disabled = true;

    try {
        await api.createNode(content);
        
        showStatus('Memory saved successfully!', 'success');
        memoryContent.value = '';
        await loadMemories();

        // Graph refresh is handled automatically via WebSocket events
    } catch (error) {
        showStatus('Failed to save memory. Please try again.', 'error');
        console.error('Error creating memory:', error);
    } finally {
        // Re-enable form elements
        memoryContent.disabled = false;
        (memoryForm.querySelector('button') as HTMLButtonElement).disabled = false;
        memoryContent.focus();
    }
}

/**
 * Handle all click events in memory list using event delegation
 * Routes clicks to appropriate handlers based on target element
 */
async function handleMemoryListClick(e: MouseEvent): Promise<void> {
    const target = e.target as HTMLElement;

    // Handle multi-select functionality
    
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

    // Find the actionable element using DOM traversal
    const deleteButton = target.closest('.delete-btn');
    const editButton = target.closest('.edit-btn');
    const saveButton = target.closest('.save-btn');
    const cancelButton = target.closest('.cancel-btn');
    const memoryItem = target.closest('.memory-item') as HTMLElement | null;

    // If click wasn't inside a memory item, ignore it
    if (!memoryItem) return;

    // Get nodeId from data attribute
    const nodeId = memoryItem.dataset.nodeId;
    if (!nodeId) return;

    // Handle delete action with confirmation
    if (deleteButton) {
        if (confirm('Are you sure you want to delete this memory? This cannot be undone.')) {
            try {
                await api.deleteNode(nodeId);
                showStatus('Memory deleted.', 'success');
                await loadMemories();
                
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

    // Handle edit action - inline editing
    if (editButton) {
        const contentDiv = memoryItem.querySelector('.memory-item-content') as HTMLElement;
        const actionsDiv = memoryItem.querySelector('.memory-item-actions') as HTMLElement;
        const originalContent = contentDiv.textContent || '';
        
        // Transform to edit mode
        contentDiv.innerHTML = `<textarea class="edit-textarea">${originalContent}</textarea>`;
        const textarea = contentDiv.querySelector('.edit-textarea') as HTMLTextAreaElement;
        
        textarea.style.width = '100%';
        textarea.style.minHeight = '80px';
        textarea.focus();
        actionsDiv.innerHTML = `
            <button class="primary-btn save-btn">Save</button>
            <button class="secondary-btn cancel-btn">Cancel</button>
        `;
        return;
    }

    // Handle save action
    if (saveButton) {
        const textarea = memoryItem.querySelector('.edit-textarea') as HTMLTextAreaElement;
        const newContent = textarea.value.trim();

        if (newContent) {
            try {
                await api.updateNode(nodeId, newContent);
                showStatus('Memory updated!', 'success');
                // Reload memories to show updated content
            } catch (error) {
                console.error('Failed to update memory:', error);
                showStatus('Failed to update memory.', 'error');
            }
        }
        await loadMemories();
        if (ENABLE_GRAPH_VISUALIZATION && graphViz) await graphViz.refreshGraph();
        return;
    }

    // Handle cancel action
    if (cancelButton) {
        await loadMemories();
        return;
    }
    
    // Handle memory item click for graph interaction
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


/**
 * Load and display all memories from the API
 */
async function loadMemories(): Promise<void> {
    try {
        const data = await api.listNodes();
        displayMemories(data.nodes || []);
    } catch (error) {
        console.error('Error loading memories:', error);
        memoryList.innerHTML = '<p class="error-message">Failed to load memories</p>';
    }
}

/**
 * Render memories to the DOM
 * Only handles HTML generation, events handled by delegation
 */
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

/**
 * Show temporary status message to user
 */
function showStatus(message: string, type: 'success' | 'error'): void {
    memoryStatus.textContent = message;
    memoryStatus.className = `status-message ${type}`;

    setTimeout(() => {
        memoryStatus.textContent = '';
        memoryStatus.className = 'status-message';
    }, 3000);
}

/**
 * Utility functions for HTML escaping and date formatting
 */
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

/**
 * Multi-select functionality for bulk operations
 */
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
 * Handle bulk delete operation
 * Deletes multiple selected memories with user confirmation
 */
async function handleBulkDelete(): Promise<void> {
    // Get selected memory IDs
    const selectedCheckboxes = document.querySelectorAll('.memory-checkbox:checked') as NodeListOf<HTMLInputElement>;
    const selectedNodeIds = Array.from(selectedCheckboxes)
        .map(checkbox => checkbox.dataset.nodeId)
        .filter(id => id) as string[];
    
    if (selectedNodeIds.length === 0) {
        return;
    }
    
    // Confirm deletion with context-aware message
    const message = selectedNodeIds.length === 1 
        ? 'Are you sure you want to delete this memory? This cannot be undone.'
        : `Are you sure you want to delete ${selectedNodeIds.length} memories? This cannot be undone.`;
    
    if (!confirm(message)) {
        return; // User cancelled the operation
    }
    
    // Execute bulk delete operation
    try {
        const response = await api.bulkDeleteNodes(selectedNodeIds);
        
        // Provide success feedback
        if (response.deletedCount && response.deletedCount > 0) {
            const successMessage = response.deletedCount === 1 
                ? 'Memory deleted successfully!'
                : `${response.deletedCount} memories deleted successfully!`;
            showStatus(successMessage, 'success');
        }
        
        // Handle partial failures
        if (response.failedNodeIds && response.failedNodeIds.length > 0) {
            const failureMessage = response.failedNodeIds.length === 1
                ? 'Failed to delete 1 memory.'
                : `Failed to delete ${response.failedNodeIds.length} memories.`;
            showStatus(failureMessage, 'error');
        }
        
        // Update UI after deletion
        await loadMemories();
        if (ENABLE_GRAPH_VISUALIZATION && graphViz) {
            await graphViz.refreshGraph();
        }
    } catch (error) {
        // Handle complete failure
        console.error('Failed to bulk delete memories:', error);
        showStatus('Failed to delete selected memories.', 'error');
    }
}

// Expose showApp function to global scope for auth integration
window.showApp = showApp;

// Initialize application when DOM is ready
document.addEventListener('DOMContentLoaded', init);
