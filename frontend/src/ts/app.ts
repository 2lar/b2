import { auth } from './authClient';
import { api } from './apiClient';
import { components } from './generated-types'; // Import OpenAPI types
import { webSocketClient } from './webSocketClient';
import { Session } from '@supabase/supabase-js';

// Type alias for easier usage
type Node = components['schemas']['Node'];

// Feature flags
// Set to true to enable graph visualization (requires complete Cytoscape.js setup)
const ENABLE_GRAPH_VISUALIZATION = true;

// DOM Element Assertions for Type Safety
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

// A variable to hold the dynamically imported module
let graphViz: { initGraph: () => void; refreshGraph: () => Promise<void> } | null = null;

// App initialization
async function init(): Promise<void> {
    const session: Session | null = await auth.getSession();
    if (session && session.user.email) {
        showApp(session.user.email);
    }

    // --- EVENT LISTENERS ---
    // All event listeners are attached only once during initialization.

    signOutBtn.addEventListener('click', handleSignOut);
    memoryForm.addEventListener('submit', handleMemorySubmit);

    // Add a single listener to the memory list container to handle all clicks within it.
    // This is the core of the event delegation pattern.
    memoryList.addEventListener('click', handleMemoryListClick);

    // Listen for the custom event dispatched by the WebSocket client
    document.addEventListener('graph-update-event', async () => {
        console.log("Graph update event received in app.ts");
        if (graphViz) {
            showStatus('New connections found! Refreshing graph...', 'success');
            await graphViz.refreshGraph();
        }
    });

    memoryContent.addEventListener('keydown', (e: KeyboardEvent) => {
        if (e.key === 'Enter' && !e.shiftKey) {
            e.preventDefault();
            const submitButton = memoryForm.querySelector('button[type="submit"]') as HTMLButtonElement;
            if (submitButton) {
                submitButton.click();
            }
        }
    });
    
    if (ENABLE_GRAPH_VISUALIZATION) {
        refreshGraphBtn.addEventListener('click', () => graphViz?.refreshGraph())
        fitGraphBtn.addEventListener('click', () => {
            if (window.cy) {
                window.cy.fit();
            }
        });
    } else {
        // Hide graph-related UI elements if the feature is disabled
        const graphSection = document.querySelector('.graph-section') as HTMLElement | null;
        if (graphSection) graphSection.style.display = 'none';
        const nodeDetailsPanel = document.getElementById('node-details');
        if (nodeDetailsPanel) nodeDetailsPanel.style.display = 'none';
    }
}

// Show the main application interface
async function showApp(email: string): Promise<void> {
    authSection.style.display = 'none';
    appSection.style.display = 'block';
    userEmail.textContent = email;

    // Connect the WebSocket client
    webSocketClient.connect();

    await loadMemories();
    
    if (ENABLE_GRAPH_VISUALIZATION) {
        // Dynamically import the graph-viz module here
        graphViz = await import('./graph-viz');
        graphViz.initGraph();
        await graphViz.refreshGraph();
    }
}

// Handle user sign-out
async function handleSignOut(): Promise<void> {
    // Disconnect the WebSocket client
    webSocketClient.disconnect();

    await auth.signOut();
    authSection.style.display = 'flex';
    appSection.style.display = 'none';
    userEmail.textContent = '';
    memoryList.innerHTML = ''; // Clear memories on sign out
}

// Handle the memory form submission
async function handleMemorySubmit(e: Event): Promise<void> {
    e.preventDefault();
    const content = memoryContent.value.trim();
    if (!content) return;

    memoryContent.disabled = true;
    (memoryForm.querySelector('button') as HTMLButtonElement).disabled = true;

    try {
        await api.createNode(content);
        showStatus('Memory saved successfully!', 'success');
        memoryContent.value = '';
        await loadMemories();

        // The graph refresh is now handled by the WebSocket even
        // if (ENABLE_GRAPH_VISUALIZATION && graphViz) {
        //     await graphViz.refreshGraph();
        // }
    } catch (error) {
        showStatus('Failed to save memory. Please try again.', 'error');
        console.error('Error creating memory:', error);
    } finally {
        memoryContent.disabled = false;
        (memoryForm.querySelector('button') as HTMLButtonElement).disabled = false;
        memoryContent.focus();
    }
}

/**
 * Handles all click events inside the #memory-list container.
 * This single function uses event delegation to manage actions for potentially hundreds
 * of memory items without attaching a listener to each one.
 * @param e The mouse click event.
 */
async function handleMemoryListClick(e: MouseEvent): Promise<void> {
    const target = e.target as HTMLElement;

    // Handle select all checkbox
    if (target.matches('#select-all-checkbox')) {
        handleSelectAllToggle();
        return;
    }

    // Handle individual memory checkboxes
    if (target.matches('.memory-checkbox')) {
        handleMemoryCheckboxChange();
        return;
    }

    // Handle bulk delete button
    if (target.matches('#bulk-delete-btn')) {
        await handleBulkDelete();
        return;
    }

    // Find the closest ancestor which is a button or the memory item itself
    const deleteButton = target.closest('.delete-btn');
    const editButton = target.closest('.edit-btn');
    const saveButton = target.closest('.save-btn');
    const cancelButton = target.closest('.cancel-btn');
    const memoryItem = target.closest('.memory-item') as HTMLElement | null;

    if (!memoryItem) return;

    const nodeId = memoryItem.dataset.nodeId;
    if (!nodeId) return;

    // --- Handle DELETE action ---
    if (deleteButton) {
        if (confirm('Are you sure you want to delete this memory? This cannot be undone.')) {
            try {
                await api.deleteNode(nodeId);
                showStatus('Memory deleted.', 'success');
                await loadMemories();
                if (ENABLE_GRAPH_VISUALIZATION && graphViz) await graphViz.refreshGraph();
            } catch (error) {
                console.error('Failed to delete memory:', error);
                showStatus('Failed to delete memory.', 'error');
            }
        }
        return;
    }

    // --- Handle EDIT action ---
    if (editButton) {
        const contentDiv = memoryItem.querySelector('.memory-item-content') as HTMLElement;
        const actionsDiv = memoryItem.querySelector('.memory-item-actions') as HTMLElement;
        const originalContent = contentDiv.textContent || '';
        
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

async function handleBulkDelete(): Promise<void> {
    const selectedCheckboxes = document.querySelectorAll('.memory-checkbox:checked') as NodeListOf<HTMLInputElement>;
    const selectedNodeIds = Array.from(selectedCheckboxes).map(checkbox => checkbox.dataset.nodeId).filter(id => id) as string[];
    
    if (selectedNodeIds.length === 0) {
        return;
    }
    
    const message = selectedNodeIds.length === 1 
        ? 'Are you sure you want to delete this memory? This cannot be undone.'
        : `Are you sure you want to delete ${selectedNodeIds.length} memories? This cannot be undone.`;
    
    if (!confirm(message)) {
        return;
    }
    
    try {
        const response = await api.bulkDeleteNodes(selectedNodeIds);
        
        if (response.deletedCount && response.deletedCount > 0) {
            const successMessage = response.deletedCount === 1 
                ? 'Memory deleted successfully!'
                : `${response.deletedCount} memories deleted successfully!`;
            showStatus(successMessage, 'success');
        }
        
        if (response.failedNodeIds && response.failedNodeIds.length > 0) {
            const failureMessage = response.failedNodeIds.length === 1
                ? 'Failed to delete 1 memory.'
                : `Failed to delete ${response.failedNodeIds.length} memories.`;
            showStatus(failureMessage, 'error');
        }
        
        await loadMemories();
        if (ENABLE_GRAPH_VISUALIZATION && graphViz) await graphViz.refreshGraph();
    } catch (error) {
        console.error('Failed to bulk delete memories:', error);
        showStatus('Failed to delete selected memories.', 'error');
    }
}

// Expose showApp to the global scope for auth.ts to call
window.showApp = showApp;

// Initialize on load
document.addEventListener('DOMContentLoaded', init);
