import { auth } from './auth';
import { api } from './api';
import { initGraph, refreshGraph } from './graph-viz';
import { MemoryNode } from './types'; // Import our shared type
import { Session } from '@supabase/supabase-js';

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

// App initialization
async function init(): Promise<void> {
    const session: Session | null = await auth.getSession();
    if (session && session.user.email) {
        showApp(session.user.email);
    }

    // Event listeners
    signOutBtn.addEventListener('click', handleSignOut);
    memoryForm.addEventListener('submit', handleMemorySubmit);
    
    if (ENABLE_GRAPH_VISUALIZATION) {
        refreshGraphBtn.addEventListener('click', () => refreshGraph());
        fitGraphBtn.addEventListener('click', () => {
            if (window.cy) {
                window.cy.fit();
            }
        });
    } else {
        // Hide graph controls when disabled
        refreshGraphBtn.style.display = 'none';
        fitGraphBtn.style.display = 'none';
        
        // Hide the graph container as well
        const graphContainer = document.getElementById('cy');
        if (graphContainer) {
            graphContainer.style.display = 'none';
        }
        
        // Hide node details panel
        const nodeDetailsPanel = document.getElementById('node-details');
        if (nodeDetailsPanel) {
            nodeDetailsPanel.style.display = 'none';
        }
    }
}

// Show the main application interface
async function showApp(email: string): Promise<void> {
    authSection.style.display = 'none';
    appSection.style.display = 'block';
    userEmail.textContent = email;

    if (ENABLE_GRAPH_VISUALIZATION) {
        initGraph();
    }

    await loadMemories();
    
    if (ENABLE_GRAPH_VISUALIZATION) {
        await refreshGraph();
    }
}

// Handle user sign-out
async function handleSignOut(): Promise<void> {
    await auth.signOut();
    authSection.style.display = 'flex';
    appSection.style.display = 'none';
    userEmail.textContent = '';
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
        
        if (ENABLE_GRAPH_VISUALIZATION) {
            await refreshGraph();
        }
    } catch (error) {
        showStatus('Failed to save memory. Please try again.', 'error');
        console.error('Error creating memory:', error);
    } finally {
        memoryContent.disabled = false;
        (memoryForm.querySelector('button') as HTMLButtonElement).disabled = false;
        memoryContent.focus();
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

// Render the list of memories to the DOM
function displayMemories(nodes: MemoryNode[]): void {
    if (nodes.length === 0) {
        memoryList.innerHTML = '<p class="empty-state">No memories yet. Create your first memory above!</p>';
        return;
    }

    nodes.sort((a, b) => new Date(b.timestamp).getTime() - new Date(a.timestamp).getTime());

    memoryList.innerHTML = nodes.map(node => `
        <div class="memory-item" data-node-id="${node.nodeId}">
            <div class="memory-item-content">${escapeHtml(node.content)}</div>
            <div class="memory-item-meta">
                ${formatDate(node.timestamp)}
            </div>
            <div class="memory-item-actions">
                <button class="secondary-btn edit-btn">Edit</button>
                <button class="danger-btn delete-btn">Delete</button>
            </div>
        </div>
    `).join('');

    // Add event listeners for the new buttons
    memoryList.querySelectorAll('.memory-item').forEach(item => {
        const nodeId = (item as HTMLElement).dataset.nodeId;
        if (!nodeId) return;

        // Graph interaction for clicking the item
        item.addEventListener('click', (e) => {
            // Prevent event from bubbling up from buttons
            if ((e.target as HTMLElement).tagName === 'BUTTON') return;
            
            if (ENABLE_GRAPH_VISUALIZATION && window.cy) {
                const node = window.cy.getElementById(nodeId);
                if (node?.length) { // Check if node exists
                    node.trigger('tap');
                }
            }
        });

        // Delete button listener
        item.querySelector('.delete-btn')?.addEventListener('click', async () => {
            if (confirm('Are you sure you want to delete this memory? This cannot be undone.')) {
                try {
                    await api.deleteNode(nodeId);
                    showStatus('Memory deleted.', 'success');
                    await loadMemories(); // Reload the list
                    if (ENABLE_GRAPH_VISUALIZATION) {
                        await refreshGraph(); // Refresh the graph
                    }
                } catch (error) {
                    console.error('Failed to delete memory:', error);
                    showStatus('Failed to delete memory.', 'error');
                }
            }
        });

        item.querySelector('.edit-btn')?.addEventListener('click', () => {
            const contentDiv = item.querySelector('.memory-item-content') as HTMLElement;
            const actionsDiv = item.querySelector('.memory-item-actions') as HTMLElement;
            const originalContent = contentDiv.textContent || '';
            
            // Replace content with a textarea for editing
            contentDiv.innerHTML = `<textarea class="edit-textarea">${originalContent}</textarea>`;
            const textarea = contentDiv.querySelector('.edit-textarea') as HTMLTextAreaElement;
            textarea.style.width = '100%';
            textarea.style.minHeight = '80px';
            textarea.focus();

            // Replace actions with Save/Cancel
            actionsDiv.innerHTML = `
                <button class="primary-btn save-btn">Save</button>
                <button class="secondary-btn cancel-btn">Cancel</button>
            `;

            // Save button listener
            actionsDiv.querySelector('.save-btn')?.addEventListener('click', async () => {
                const newContent = textarea.value.trim();
                if (newContent && newContent !== originalContent) {
                    try {
                        await api.updateNode(nodeId, newContent);
                        showStatus('Memory updated!', 'success');
                        await loadMemories(); // Reload to show updated state
                        if (ENABLE_GRAPH_VISUALIZATION) {
                            await refreshGraph();
                        }
                    } catch (error) {
                        console.error('Failed to update memory:', error);
                        showStatus('Failed to update memory.', 'error');
                        // On error, revert UI
                        contentDiv.textContent = originalContent;
                        actionsDiv.innerHTML = `
                            <button class="secondary-btn edit-btn">Edit</button>
                            <button class="danger-btn delete-btn">Delete</button>
                        `;
                    }
                } else {
                    // If no change, just cancel
                    contentDiv.textContent = originalContent;
                    actionsDiv.innerHTML = `
                        <button class="secondary-btn edit-btn">Edit</button>
                        <button class="danger-btn delete-btn">Delete</button>
                    `;
                }
            });

            // Cancel button listener
            actionsDiv.querySelector('.cancel-btn')?.addEventListener('click', () => {
                // Simply reload the memories to revert all editing states
                loadMemories();
            });
        });
    });
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

// Expose showApp to the global scope for auth.ts to call
window.showApp = showApp;

// Initialize on load
document.addEventListener('DOMContentLoaded', init);

