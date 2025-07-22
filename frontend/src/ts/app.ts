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
const themeToggle = document.getElementById('theme-toggle') as HTMLButtonElement;
const fullscreenGraphBtn = document.getElementById('fullscreen-graph') as HTMLButtonElement;

// Pagination elements
const memoryCount = document.getElementById('memory-count') as HTMLElement;
const prevPageBtn = document.getElementById('prev-page') as HTMLButtonElement;
const nextPageBtn = document.getElementById('next-page') as HTMLButtonElement;
const pageInfo = document.getElementById('page-info') as HTMLElement;

// Pagination state
let currentPage = 1;
let totalPages = 1;
let totalMemories = 0;
const MEMORIES_PER_PAGE = 50;

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

    // Pagination event listeners
    prevPageBtn.addEventListener('click', () => {
        if (currentPage > 1) {
            currentPage--;
            loadMemories();
        }
    });

    nextPageBtn.addEventListener('click', () => {
        if (currentPage < totalPages) {
            currentPage++;
            loadMemories();
        }
    });

    // Initialize dashboard functionality
    initDashboard();
    
    // Initialize theme and fullscreen functionality
    initTheme();
    initGraphFullscreen();
    
    // Set up graph visualization if enabled
    if (ENABLE_GRAPH_VISUALIZATION) {
        // Set up graph control event listeners
        refreshGraphBtn.addEventListener('click', () => graphViz?.refreshGraph())
        fitGraphBtn.addEventListener('click', () => {
            if (window.cy) {
                window.cy.resize();
                setTimeout(() => {
                    if (window.cy) {
                        window.cy.fit();
                        window.cy.center();
                    }
                }, 100);
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
        const allNodes = data.nodes || [];
        totalMemories = allNodes.length;
        totalPages = Math.ceil(totalMemories / MEMORIES_PER_PAGE);
        
        updateMemoryCount();
        displayMemories(allNodes);
        updatePaginationControls();
    } catch (error) {
        console.error('Error loading memories:', error);
        memoryList.innerHTML = '<p class="error-message">Failed to load memories</p>';
        updateMemoryCount();
        updatePaginationControls();
    }
}

/**
 * Render memories to the DOM with pagination
 * Only handles HTML generation, events handled by delegation
 */
function displayMemories(allNodes: Node[]): void {
    if (allNodes.length === 0) {
        memoryList.innerHTML = '<p class="empty-state">No memories yet. Create your first memory above!</p>';
        return;
    }

    // Sort all nodes by timestamp
    allNodes.sort((a, b) => new Date(b.timestamp || '').getTime() - new Date(a.timestamp || '').getTime());

    // Calculate pagination
    const startIndex = (currentPage - 1) * MEMORIES_PER_PAGE;
    const endIndex = startIndex + MEMORIES_PER_PAGE;
    const nodesForCurrentPage = allNodes.slice(startIndex, endIndex);

    // Create multi-select controls header
    const multiSelectHeader = `
        <div class="memory-list-controls">
            <div class="select-controls">
                <label class="checkbox-container">
                    <input type="checkbox" id="select-all-checkbox" class="select-all-checkbox">
                    <span class="checkmark"></span>
                    Select All (Page)
                </label>
                <span class="selected-count" id="selected-count">0 selected</span>
            </div>
            <div class="bulk-actions">
                <button class="danger-btn bulk-delete-btn" id="bulk-delete-btn" disabled>Delete Selected</button>
            </div>
        </div>
    `;

    const memoriesHtml = nodesForCurrentPage.map(node => `
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
 * Update memory count display
 */
function updateMemoryCount(): void {
    if (memoryCount) {
        memoryCount.textContent = totalMemories === 1 ? '1 memory' : `${totalMemories} memories`;
    }
}

/**
 * Update pagination controls
 */
function updatePaginationControls(): void {
    if (prevPageBtn) {
        prevPageBtn.disabled = currentPage <= 1;
    }
    
    if (nextPageBtn) {
        nextPageBtn.disabled = currentPage >= totalPages;
    }
    
    if (pageInfo) {
        pageInfo.textContent = totalPages > 0 ? `Page ${currentPage} of ${totalPages}` : 'No pages';
    }
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

/**
 * Dashboard Layout Management
 * Handles drag and drop functionality for movable containers
 */
function initDashboard(): void {
    const containers = document.querySelectorAll('.dashboard-container');

    containers.forEach(container => {
        const header = container.querySelector('[data-drag-handle]') as HTMLElement;
        if (!header) return;

        // Make containers draggable
        container.setAttribute('draggable', 'true');

        // Drag start
        container.addEventListener('dragstart', (e: Event) => {
            const dragEvent = e as DragEvent;
            if (!dragEvent.dataTransfer) return;
            
            const containerElement = e.currentTarget as HTMLElement;
            const containerId = containerElement.id;
            
            dragEvent.dataTransfer.setData('text/plain', containerId);
            dragEvent.dataTransfer.effectAllowed = 'move';
            
            containerElement.classList.add('dragging');
            
            // Set drag image to the container
            dragEvent.dataTransfer.setDragImage(containerElement, 0, 0);
        });

        // Drag end
        container.addEventListener('dragend', (e: Event) => {
            const containerElement = e.currentTarget as HTMLElement;
            containerElement.classList.remove('dragging');
            
            // Remove drop target classes from all containers
            containers.forEach(c => c.classList.remove('drop-target'));
        });

        // Drag over (allow drop)
        container.addEventListener('dragover', (e: Event) => {
            const dragEvent = e as DragEvent;
            e.preventDefault();
            if (dragEvent.dataTransfer) {
                dragEvent.dataTransfer.dropEffect = 'move';
            }
        });

        // Drag enter
        container.addEventListener('dragenter', (e: Event) => {
            e.preventDefault();
            const containerElement = e.currentTarget as HTMLElement;
            if (!containerElement.classList.contains('dragging')) {
                containerElement.classList.add('drop-target');
            }
        });

        // Drag leave
        container.addEventListener('dragleave', (e: Event) => {
            const dragEvent = e as DragEvent;
            const containerElement = e.currentTarget as HTMLElement;
            const rect = containerElement.getBoundingClientRect();
            const x = dragEvent.clientX;
            const y = dragEvent.clientY;
            
            // Only remove drop-target if we've actually left the container
            if (x < rect.left || x > rect.right || y < rect.top || y > rect.bottom) {
                containerElement.classList.remove('drop-target');
            }
        });

        // Drop
        container.addEventListener('drop', (e: Event) => {
            const dragEvent = e as DragEvent;
            e.preventDefault();
            if (!dragEvent.dataTransfer) return;

            const draggedId = dragEvent.dataTransfer.getData('text/plain');
            const dropTarget = e.currentTarget as HTMLElement;
            const dropTargetId = dropTarget.id;

            if (draggedId !== dropTargetId) {
                swapContainers(draggedId, dropTargetId);
            }

            dropTarget.classList.remove('drop-target');
        });
    });

    // Initialize resize functionality
    initResizeHandles();
}

function swapContainers(container1Id: string, container2Id: string): void {
    const container1 = document.getElementById(container1Id);
    const container2 = document.getElementById(container2Id);

    if (!container1 || !container2) return;

    // Get the parent containers
    const parent1 = container1.parentElement;
    const parent2 = container2.parentElement;
    
    // Get the next sibling to know where to insert
    const next1 = container1.nextElementSibling;
    const next2 = container2.nextElementSibling;

    // Swap the containers
    if (next1) {
        parent2?.insertBefore(container1, next1);
    } else {
        parent2?.appendChild(container1);
    }

    if (next2) {
        parent1?.insertBefore(container2, next2);
    } else {
        parent1?.appendChild(container2);
    }

    // If graph container was moved, we need to reinitialize the graph
    if (container1Id === 'graph-container' || container2Id === 'graph-container') {
        setTimeout(() => {
            if (ENABLE_GRAPH_VISUALIZATION && graphViz && window.cy) {
                window.cy.resize();
                window.cy.fit();
                window.cy.center();
            }
        }, 150);
    }
}

function initResizeHandles(): void {
    const resizeHandles = document.querySelectorAll('.resize-handle');
    
    resizeHandles.forEach(handle => {
        const handleElement = handle as HTMLElement;
        const resizeType = handleElement.dataset.resize;

        if (resizeType === 'vertical') {
            initVerticalResize(handleElement);
        } else if (resizeType === 'horizontal-left' || resizeType === 'horizontal-right') {
            initHorizontalResize(handleElement, resizeType);
        }
    });
}

function initVerticalResize(handle: HTMLElement): void {
    let isResizing = false;
    let startY = 0;
    let startHeight = 0;
    let topContainer: HTMLElement;
    let bottomContainer: HTMLElement;

    handle.addEventListener('mousedown', (e: Event) => {
        const mouseEvent = e as MouseEvent;
        isResizing = true;
        startY = mouseEvent.clientY;
        
        const rightColumn = handle.parentElement as HTMLElement;
        
        topContainer = rightColumn.querySelector('#input-container') as HTMLElement;
        bottomContainer = rightColumn.querySelector('#list-container') as HTMLElement;
        
        if (topContainer && bottomContainer) {
            startHeight = topContainer.getBoundingClientRect().height;
            
            document.addEventListener('mousemove', handleMouseMove);
            document.addEventListener('mouseup', handleMouseUp);
            
            // Prevent text selection during resize
            document.body.style.userSelect = 'none';
            document.body.style.cursor = 'row-resize';
        }
    });

    function handleMouseMove(e: MouseEvent): void {
        if (!isResizing || !topContainer || !bottomContainer) return;
        
        const deltaY = e.clientY - startY;
        const newHeight = Math.max(100, Math.min(600, startHeight + deltaY));
        
        topContainer.style.flex = `0 0 ${newHeight}px`;
    }

    function handleMouseUp(): void {
        isResizing = false;
        document.removeEventListener('mousemove', handleMouseMove);
        document.removeEventListener('mouseup', handleMouseUp);
        
        // Restore normal cursor and selection
        document.body.style.userSelect = '';
        document.body.style.cursor = '';
    }
}

function initHorizontalResize(handle: HTMLElement, resizeType: string): void {
    let isResizing = false;
    let startX = 0;
    let startWidth = 0;
    let targetElement: HTMLElement;

    handle.addEventListener('mousedown', (e: Event) => {
        const mouseEvent = e as MouseEvent;
        isResizing = true;
        startX = mouseEvent.clientX;
        
        const dashboardLayout = handle.parentElement as HTMLElement;
        
        if (resizeType === 'horizontal-left') {
            targetElement = dashboardLayout.querySelector('.left-sidebar') as HTMLElement;
        } else {
            targetElement = dashboardLayout.querySelector('#graph-container') as HTMLElement;
        }
        
        if (targetElement) {
            startWidth = targetElement.getBoundingClientRect().width;
            
            document.addEventListener('mousemove', handleMouseMove);
            document.addEventListener('mouseup', handleMouseUp);
            
            // Prevent text selection during resize
            document.body.style.userSelect = 'none';
            document.body.style.cursor = 'col-resize';
        }
    });

    function handleMouseMove(e: MouseEvent): void {
        if (!isResizing || !targetElement) return;
        
        const deltaX = e.clientX - startX;
        let newWidth = startWidth + deltaX;
        
        // Get container dimensions for better constraints
        const dashboardLayout = targetElement.parentElement as HTMLElement;
        const totalWidth = dashboardLayout.getBoundingClientRect().width;
        
        // Set min/max constraints based on available space
        if (resizeType === 'horizontal-left') {
            newWidth = Math.max(150, Math.min(400, newWidth));
            dashboardLayout.style.gridTemplateColumns = `${newWidth}px 8px 1fr 8px 1fr`;
        } else {
            const sidebar = dashboardLayout.querySelector('.left-sidebar') as HTMLElement;
            const sidebarWidth = sidebar.getBoundingClientRect().width;
            const rightColumnMinWidth = 300; // Minimum width for right column
            const availableWidth = totalWidth - sidebarWidth - 16; // Subtract sidebar and resize handles
            const maxGraphWidth = availableWidth - rightColumnMinWidth;
            
            // More flexible constraints for graph resizing
            newWidth = Math.max(200, Math.min(maxGraphWidth, newWidth));
            
            // Calculate remaining width for right column
            const rightColumnWidth = availableWidth - newWidth;
            dashboardLayout.style.gridTemplateColumns = `${sidebarWidth}px 8px ${newWidth}px 8px ${rightColumnWidth}px`;
        }
    }

    function handleMouseUp(): void {
        isResizing = false;
        document.removeEventListener('mousemove', handleMouseMove);
        document.removeEventListener('mouseup', handleMouseUp);
        
        // Restore normal cursor and selection
        document.body.style.userSelect = '';
        document.body.style.cursor = '';
        
        // Trigger graph resize if graph was affected
        if (ENABLE_GRAPH_VISUALIZATION && window.cy) {
            setTimeout(() => {
                if (window.cy) {
                    window.cy.resize();
                    window.cy.fit();
                    window.cy.center();
                }
            }, 150);
        }
    }
}

/**
 * Theme Management
 * Handles dark/light mode switching with localStorage persistence
 */
function initTheme(): void {
    // Get saved theme or default to dark
    const savedTheme = localStorage.getItem('theme') || 'dark';
    document.documentElement.setAttribute('data-theme', savedTheme);
    updateThemeButton(savedTheme);

    // Theme toggle event listener
    themeToggle.addEventListener('click', () => {
        const currentTheme = document.documentElement.getAttribute('data-theme');
        const newTheme = currentTheme === 'dark' ? 'light' : 'dark';
        
        document.documentElement.setAttribute('data-theme', newTheme);
        localStorage.setItem('theme', newTheme);
        updateThemeButton(newTheme);
    });
}

function updateThemeButton(theme: string): void {
    if (theme === 'dark') {
        themeToggle.innerHTML = '🌙 Dark';
    } else {
        themeToggle.innerHTML = '☀️ Light';
    }
}

/**
 * Graph Fullscreen Functionality
 * Handles fullscreen mode for graph visualization
 */
let isGraphFullscreen = false;

function initGraphFullscreen(): void {
    fullscreenGraphBtn.addEventListener('click', toggleGraphFullscreen);
    
    // Listen for escape key to exit fullscreen
    document.addEventListener('keydown', (e: KeyboardEvent) => {
        if (e.key === 'Escape' && isGraphFullscreen) {
            exitGraphFullscreen();
        }
    });
}

function toggleGraphFullscreen(): void {
    if (isGraphFullscreen) {
        exitGraphFullscreen();
    } else {
        enterGraphFullscreen();
    }
}

function enterGraphFullscreen(): void {
    const graphContainer = document.getElementById('graph-container');
    if (!graphContainer) return;

    // Add fullscreen class
    graphContainer.classList.add('graph-fullscreen');
    
    // Update button text
    fullscreenGraphBtn.innerHTML = '🗗 Exit Fullscreen';
    
    // Set flag
    isGraphFullscreen = true;
    
    // Resize graph after fullscreen transition
    setTimeout(() => {
        if (window.cy) {
            window.cy.resize();
            window.cy.fit();
            window.cy.center();
        }
    }, 100);
}

function exitGraphFullscreen(): void {
    const graphContainer = document.getElementById('graph-container');
    if (!graphContainer) return;

    // Remove fullscreen class
    graphContainer.classList.remove('graph-fullscreen');
    
    // Update button text
    fullscreenGraphBtn.innerHTML = '⛶ Fullscreen';
    
    // Set flag
    isGraphFullscreen = false;
    
    // Resize graph after fullscreen exit
    setTimeout(() => {
        if (window.cy) {
            window.cy.resize();
            window.cy.fit();
            window.cy.center();
        }
    }, 100);
}

// Expose showApp function to global scope for auth integration
window.showApp = showApp;

// Initialize application when DOM is ready
document.addEventListener('DOMContentLoaded', init);
