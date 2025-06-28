/**
 * WebSocket Client for Real-Time Graph Updates
 * Manages WebSocket connection to receive real-time updates about graph changes
 */

interface WebSocketMessage {
    action: string;
    nodeId?: string;
}

class WebSocketClient {
    private socket: WebSocket | null = null;
    private isConnecting: boolean = false;
    private reconnectAttempts: number = 0;
    private maxReconnectAttempts: number = 5;
    private reconnectDelay: number = 1000; // Start with 1 second
    private maxReconnectDelay: number = 30000; // Max 30 seconds

    /**
     * Initialize the WebSocket connection
     * @param wsUrl WebSocket API URL from the CDK outputs
     * @param token JWT token for authentication
     */
    async init(wsUrl: string, token: string): Promise<void> {
        if (this.socket?.readyState === WebSocket.OPEN || this.isConnecting) {
            console.log('WebSocket already connected or connecting');
            return;
        }

        this.isConnecting = true;

        try {
            // Construct WebSocket URL with token parameter
            const url = new URL(wsUrl);
            url.searchParams.set('token', encodeURIComponent(token));
            
            console.log('Connecting to WebSocket:', url.toString().replace(/token=[^&]+/, 'token=***'));

            this.socket = new WebSocket(url.toString());
            
            this.socket.onopen = this.handleOpen.bind(this);
            this.socket.onmessage = this.handleMessage.bind(this);
            this.socket.onclose = this.handleClose.bind(this);
            this.socket.onerror = this.handleError.bind(this);

        } catch (error) {
            console.error('Failed to initialize WebSocket:', error);
            this.isConnecting = false;
            throw error;
        }
    }

    /**
     * Close the WebSocket connection
     */
    disconnect(): void {
        if (this.socket) {
            console.log('Disconnecting WebSocket');
            this.socket.close(1000, 'User disconnected');
            this.socket = null;
        }
        this.isConnecting = false;
        this.reconnectAttempts = 0;
    }

    /**
     * Get the current connection status
     */
    isConnected(): boolean {
        return this.socket?.readyState === WebSocket.OPEN;
    }

    /**
     * Handle WebSocket connection opened
     */
    private handleOpen(event: Event): void {
        console.log('WebSocket connected successfully');
        this.isConnecting = false;
        this.reconnectAttempts = 0;
        this.reconnectDelay = 1000; // Reset delay
        
        // Dispatch custom event to notify the app
        const connectEvent = new CustomEvent('websocket-connected');
        window.dispatchEvent(connectEvent);
    }

    /**
     * Handle incoming WebSocket messages
     */
    private handleMessage(event: MessageEvent): void {
        try {
            const message: WebSocketMessage = JSON.parse(event.data);
            console.log('Received WebSocket message:', message);

            switch (message.action) {
                case 'graphUpdated':
                    this.handleGraphUpdated(message.nodeId);
                    break;
                default:
                    console.log('Unknown WebSocket message action:', message.action);
            }
        } catch (error) {
            console.error('Failed to parse WebSocket message:', error);
        }
    }

    /**
     * Handle graph update message
     */
    private handleGraphUpdated(nodeId?: string): void {
        console.log('Graph updated for node:', nodeId);
        
        // Dispatch custom DOM event that the main app can listen to
        const updateEvent = new CustomEvent('graph-update-event', {
            detail: { nodeId }
        });
        window.dispatchEvent(updateEvent);
    }

    /**
     * Handle WebSocket connection closed
     */
    private handleClose(event: CloseEvent): void {
        console.log('WebSocket connection closed:', event.code, event.reason);
        this.socket = null;
        this.isConnecting = false;

        // Dispatch custom event to notify the app
        const disconnectEvent = new CustomEvent('websocket-disconnected');
        window.dispatchEvent(disconnectEvent);

        // Attempt to reconnect if it wasn't a clean close
        if (event.code !== 1000 && this.reconnectAttempts < this.maxReconnectAttempts) {
            this.scheduleReconnect();
        } else if (this.reconnectAttempts >= this.maxReconnectAttempts) {
            console.error('Max reconnection attempts reached. Please refresh the page.');
        }
    }

    /**
     * Handle WebSocket errors
     */
    private handleError(event: Event): void {
        console.error('WebSocket error:', event);
        this.isConnecting = false;
    }

    /**
     * Schedule a reconnection attempt with exponential backoff
     */
    private scheduleReconnect(): void {
        this.reconnectAttempts++;
        
        console.log(`Scheduling WebSocket reconnection attempt ${this.reconnectAttempts}/${this.maxReconnectAttempts} in ${this.reconnectDelay}ms`);
        
        setTimeout(() => {
            if (!this.isConnected() && !this.isConnecting) {
                // Get the stored connection parameters and retry
                // Note: In a real implementation, you'd store these or have a way to retrieve them
                console.log('Attempting WebSocket reconnection...');
                // The app should handle this by calling init() again
                const reconnectEvent = new CustomEvent('websocket-reconnect-needed');
                window.dispatchEvent(reconnectEvent);
            }
        }, this.reconnectDelay);

        // Exponential backoff with jitter
        this.reconnectDelay = Math.min(this.reconnectDelay * 2, this.maxReconnectDelay);
        // Add some jitter to prevent thundering herd
        this.reconnectDelay += Math.random() * 1000;
    }
}

// Create and export a singleton instance
export const webSocketClient = new WebSocketClient();

// Make it available globally for debugging
declare global {
    interface Window {
        webSocketClient: WebSocketClient;
    }
}

window.webSocketClient = webSocketClient;