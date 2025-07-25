/**
 * WebSocket Client - Real-Time Communication for Graph Updates
 * 
 * Implements real-time bidirectional communication using WebSockets.
 * Handles connection management, authentication, and message routing.
 * Enables instant graph updates without page refresh.
 */

import { auth } from './authClient';

// Global WebSocket state management
let socket: WebSocket | null = null;
let reconnectTimer: ReturnType<typeof setTimeout> | null = null;
const WEBSOCKET_URL = import.meta.env.VITE_WEBSOCKET_URL;

/**
 * Establish WebSocket connection with authentication and event handlers
 * Manages connection lifecycle, authentication, and reconnection logic
 */
async function connect() {
    // Check if connection already exists
    if (socket && socket.readyState === WebSocket.OPEN) {
        console.log('WebSocket is already open.');
        return;
    }

    // Validate configuration
    if (!WEBSOCKET_URL) {
        console.error("VITE_WEBSOCKET_URL is not defined in the environment.");
        return;
    }

    // Get authentication token for WebSocket connection
    const token = await auth.getJwtToken();
    if (!token) {
        console.error('Authentication token not found. Cannot connect WebSocket.');
        return;
    }

    // Create WebSocket connection with authentication token
    const urlWithToken = `${WEBSOCKET_URL}?token=${token}`;
    socket = new WebSocket(urlWithToken);

    // Handle successful connection
    socket.onopen = () => {
        console.log('WebSocket connection established.');
        if (reconnectTimer) {
            clearTimeout(reconnectTimer);
            reconnectTimer = null;
        }
    };

    // Handle incoming messages
    socket.onmessage = (event) => {
        try {
            console.log('[WebSocket] Raw message received:', event.data);
            const message = JSON.parse(event.data);
            console.log('[WebSocket] Parsed message:', message);
            
            if (message.action === 'graphUpdated') {
                if (message.nodeId) {
                    console.log('[WebSocket] Dispatching graph-update-event with nodeId:', message.nodeId);
                    document.dispatchEvent(new CustomEvent('graph-update-event', {
                        detail: { nodeId: message.nodeId }
                    }));
                } else {
                    console.warn('[WebSocket] graphUpdated event missing nodeId:', message);
                }
            }
        } catch (err) {
            console.error('[WebSocket] Error parsing message:', err);
        }
    };

    // Handle connection closure and automatic reconnection
    socket.onclose = (event) => {
        console.warn(`WebSocket closed. Code: ${event.code}. Reconnecting in 3 seconds...`);
        if (!reconnectTimer) {
            reconnectTimer = setTimeout(connect, 3000);
        }
    };

    // Handle connection errors
    socket.onerror = (error) => {
        console.error('WebSocket error:', error);
        socket?.close();
    };
}

/**
 * Disconnect WebSocket and clean up resources
 * Prevents automatic reconnection and clears timers
 */
function disconnect() {
    // Cancel any pending reconnection attempts
    if (reconnectTimer) {
        clearTimeout(reconnectTimer);
        reconnectTimer = null;
    }
    
    // Close WebSocket connection gracefully
    if (socket) {
        socket.close(1000, "User logged out");
        socket = null;
    }
}

/**
 * WebSocket client public API
 * Provides connect and disconnect functions for real-time communication
 */
export const webSocketClient = {
    connect,
    disconnect,
};
