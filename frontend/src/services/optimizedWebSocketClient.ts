/**
 * Optimized WebSocket Client - Enhanced Real-Time Communication
 * 
 * Purpose:
 * Provides an enhanced WebSocket client with advanced features for better performance
 * and reliability. Includes message batching, reconnection strategies, and queue management.
 * 
 * Key Features:
 * - Message batching for reduced network overhead
 * - Exponential backoff reconnection strategy
 * - Message queuing during disconnections
 * - Heartbeat mechanism for connection health
 * - Event deduplication and throttling
 * - Connection state management
 * - Error recovery mechanisms
 */

import { auth } from './authClient';

interface QueuedMessage {
    id: string;
    type: string;
    data: any;
    timestamp: number;
    retries: number;
}

interface WebSocketEvent {
    type: string;
    data: any;
    timestamp: number;
}

interface ConnectionMetrics {
    connectTime: number;
    lastMessageTime: number;
    messageCount: number;
    reconnectCount: number;
    errorCount: number;
}

class OptimizedWebSocketClient {
    private socket: WebSocket | null = null;
    private reconnectTimer: ReturnType<typeof setTimeout> | null = null;
    private heartbeatTimer: ReturnType<typeof setInterval> | null = null;
    private messageQueue: QueuedMessage[] = [];
    private pendingMessages: Map<string, QueuedMessage> = new Map();
    private eventHistory: Map<string, number> = new Map();
    private metrics: ConnectionMetrics;
    
    // Configuration
    private readonly WEBSOCKET_URL = import.meta.env.VITE_WEBSOCKET_URL;
    private readonly MAX_RECONNECT_ATTEMPTS = 10;
    private readonly INITIAL_RECONNECT_DELAY = 1000; // 1 second
    private readonly MAX_RECONNECT_DELAY = 30000; // 30 seconds
    private readonly HEARTBEAT_INTERVAL = 30000; // 30 seconds
    private readonly MESSAGE_BATCH_SIZE = 10;
    private readonly MESSAGE_BATCH_DELAY = 100; // ms
    private readonly EVENT_DEDUPE_WINDOW = 1000; // ms
    private readonly MAX_QUEUE_SIZE = 100;
    
    // State
    private reconnectAttempts = 0;
    private isConnecting = false;
    private connectionState: 'disconnected' | 'connecting' | 'connected' | 'reconnecting' = 'disconnected';
    private batchTimer: ReturnType<typeof setTimeout> | null = null;
    private messageBuffer: WebSocketEvent[] = [];

    constructor() {
        this.metrics = {
            connectTime: 0,
            lastMessageTime: 0,
            messageCount: 0,
            reconnectCount: 0,
            errorCount: 0
        };
    }

    /**
     * Establish WebSocket connection with enhanced error handling and authentication
     */
    async connect(): Promise<void> {
        if (this.socket && this.socket.readyState === WebSocket.OPEN) {
            return;
        }

        if (this.isConnecting) {
            return;
        }

        if (!this.WEBSOCKET_URL) {
            console.error("VITE_WEBSOCKET_URL is not defined in the environment.");
            return;
        }

        try {
            this.isConnecting = true;
            this.connectionState = this.reconnectAttempts > 0 ? 'reconnecting' : 'connecting';
            
            const token = await auth.getJwtToken();
            if (!token) {
                console.error('Authentication token not found. Cannot connect WebSocket.');
                this.isConnecting = false;
                return;
            }

            const urlWithToken = `${this.WEBSOCKET_URL}?token=${token}`;
            this.socket = new WebSocket(urlWithToken);

            this.socket.onopen = this.handleOpen.bind(this);
            this.socket.onmessage = this.handleMessage.bind(this);
            this.socket.onclose = this.handleClose.bind(this);
            this.socket.onerror = this.handleError.bind(this);

        } catch (error) {
            console.error('Error establishing WebSocket connection:', error);
            this.isConnecting = false;
            this.scheduleReconnect();
        }
    }

    /**
     * Handle successful connection
     */
    private handleOpen(): void {
        console.log('WebSocket connected successfully');
        this.isConnecting = false;
        this.connectionState = 'connected';
        this.reconnectAttempts = 0;
        this.metrics.connectTime = Date.now();
        
        // Clear reconnect timer
        if (this.reconnectTimer) {
            clearTimeout(this.reconnectTimer);
            this.reconnectTimer = null;
        }

        // Start heartbeat
        this.startHeartbeat();

        // Process queued messages
        this.processMessageQueue();

        // Emit connection event
        this.emitEvent('connection', { state: 'connected', timestamp: Date.now() });
    }

    /**
     * Handle incoming messages with batching and deduplication
     */
    private handleMessage(event: MessageEvent): void {
        try {
            const message = JSON.parse(event.data);
            this.metrics.lastMessageTime = Date.now();
            this.metrics.messageCount++;

            // Handle heartbeat response
            if (message.type === 'pong') {
                return;
            }

            // Check for duplicate events
            if (this.isDuplicateEvent(message)) {
                return;
            }

            // Add to buffer for batching
            this.addToMessageBuffer({
                type: message.type,
                data: message,
                timestamp: Date.now()
            });

        } catch (err) {
            console.error('WebSocket message parsing error:', err);
            this.metrics.errorCount++;
        }
    }

    /**
     * Handle connection closure with reconnection logic
     */
    private handleClose(event: CloseEvent): void {
        console.warn(`WebSocket closed: ${event.code} - ${event.reason}`);
        this.connectionState = 'disconnected';
        this.stopHeartbeat();

        // Don't reconnect if it was a deliberate closure
        if (event.code === 1000) {
            return;
        }

        this.scheduleReconnect();
    }

    /**
     * Handle connection errors
     */
    private handleError(error: Event): void {
        console.error('WebSocket error:', error);
        this.metrics.errorCount++;
        this.socket?.close();
    }

    /**
     * Schedule reconnection with exponential backoff
     */
    private scheduleReconnect(): void {
        if (this.reconnectAttempts >= this.MAX_RECONNECT_ATTEMPTS) {
            console.error('Max reconnection attempts reached. Giving up.');
            this.emitEvent('connection', { 
                state: 'failed', 
                reason: 'max_attempts_reached',
                timestamp: Date.now() 
            });
            return;
        }

        const delay = Math.min(
            this.INITIAL_RECONNECT_DELAY * Math.pow(2, this.reconnectAttempts),
            this.MAX_RECONNECT_DELAY
        );

        console.log(`Scheduling reconnection attempt ${this.reconnectAttempts + 1} in ${delay}ms`);

        this.reconnectTimer = setTimeout(() => {
            this.reconnectAttempts++;
            this.metrics.reconnectCount++;
            this.connect();
        }, delay);
    }

    /**
     * Start heartbeat mechanism
     */
    private startHeartbeat(): void {
        this.heartbeatTimer = setInterval(() => {
            if (this.socket && this.socket.readyState === WebSocket.OPEN) {
                this.socket.send(JSON.stringify({ type: 'ping', timestamp: Date.now() }));
            }
        }, this.HEARTBEAT_INTERVAL);
    }

    /**
     * Stop heartbeat mechanism
     */
    private stopHeartbeat(): void {
        if (this.heartbeatTimer) {
            clearInterval(this.heartbeatTimer);
            this.heartbeatTimer = null;
        }
    }

    /**
     * Add message to buffer for batched processing
     */
    private addToMessageBuffer(event: WebSocketEvent): void {
        this.messageBuffer.push(event);

        // Process immediately if buffer is full
        if (this.messageBuffer.length >= this.MESSAGE_BATCH_SIZE) {
            this.processBatchedMessages();
            return;
        }

        // Otherwise schedule processing
        if (!this.batchTimer) {
            this.batchTimer = setTimeout(() => {
                this.processBatchedMessages();
            }, this.MESSAGE_BATCH_DELAY);
        }
    }

    /**
     * Process batched messages
     */
    private processBatchedMessages(): void {
        if (this.batchTimer) {
            clearTimeout(this.batchTimer);
            this.batchTimer = null;
        }

        if (this.messageBuffer.length === 0) {
            return;
        }

        const batch = [...this.messageBuffer];
        this.messageBuffer = [];

        // Process batch in next tick to avoid blocking
        requestAnimationFrame(() => {
            batch.forEach(event => {
                this.processMessage(event);
            });
        });
    }

    /**
     * Process individual message
     */
    private processMessage(event: WebSocketEvent): void {
        if (event.data.type === 'nodeCreated') {
            document.dispatchEvent(new CustomEvent('graph-update-event', {
                detail: {
                    type: 'nodeCreated',
                    node: {
                        id: event.data.nodeId,
                        content: event.data.content,
                        label: event.data.content?.substring(0, 50) || '',
                        keywords: event.data.keywords
                    },
                    edges: event.data.edges || [],
                    timestamp: event.data.timestamp
                }
            }));
        } else if (event.data.action === 'graphUpdated') {
            if (event.data.nodeId) {
                document.dispatchEvent(new CustomEvent('graph-update-event', {
                    detail: { nodeId: event.data.nodeId }
                }));
            }
        }
    }

    /**
     * Check if an event is a duplicate
     */
    private isDuplicateEvent(message: any): boolean {
        const eventKey = `${message.type}-${message.nodeId || message.id || ''}`;
        const now = Date.now();
        const lastEventTime = this.eventHistory.get(eventKey);

        if (lastEventTime && (now - lastEventTime) < this.EVENT_DEDUPE_WINDOW) {
            return true;
        }

        this.eventHistory.set(eventKey, now);

        // Clean up old entries
        for (const [key, timestamp] of this.eventHistory.entries()) {
            if (now - timestamp > this.EVENT_DEDUPE_WINDOW * 2) {
                this.eventHistory.delete(key);
            }
        }

        return false;
    }

    /**
     * Process queued messages after reconnection
     */
    private processMessageQueue(): void {
        if (this.messageQueue.length === 0) {
            return;
        }

        const toProcess = [...this.messageQueue];
        this.messageQueue = [];

        console.log(`Processing ${toProcess.length} queued messages`);

        toProcess.forEach(queuedMessage => {
            if (this.socket && this.socket.readyState === WebSocket.OPEN) {
                this.socket.send(JSON.stringify(queuedMessage.data));
            }
        });
    }

    /**
     * Send message with queuing support
     */
    public sendMessage(type: string, data: any): void {
        const message = {
            id: `${Date.now()}-${Math.random().toString(36).substr(2, 9)}`,
            type,
            data: { type, ...data },
            timestamp: Date.now(),
            retries: 0
        };

        if (this.socket && this.socket.readyState === WebSocket.OPEN) {
            this.socket.send(JSON.stringify(message.data));
        } else {
            // Queue message for later
            if (this.messageQueue.length < this.MAX_QUEUE_SIZE) {
                this.messageQueue.push(message);
            } else {
                console.warn('Message queue full, dropping oldest message');
                this.messageQueue.shift();
                this.messageQueue.push(message);
            }
        }
    }

    /**
     * Emit internal events
     */
    private emitEvent(type: string, data: any): void {
        document.dispatchEvent(new CustomEvent(`websocket-${type}`, { detail: data }));
    }

    /**
     * Get connection metrics
     */
    public getMetrics(): ConnectionMetrics & { connectionState: string; queueSize: number } {
        return {
            ...this.metrics,
            connectionState: this.connectionState,
            queueSize: this.messageQueue.length
        };
    }

    /**
     * Get connection state
     */
    public getConnectionState(): string {
        return this.connectionState;
    }

    /**
     * Disconnect and clean up resources
     */
    public disconnect(): void {
        // Cancel timers
        if (this.reconnectTimer) {
            clearTimeout(this.reconnectTimer);
            this.reconnectTimer = null;
        }

        if (this.batchTimer) {
            clearTimeout(this.batchTimer);
            this.batchTimer = null;
        }

        this.stopHeartbeat();

        // Close socket
        if (this.socket) {
            this.socket.close(1000, "User logged out");
            this.socket = null;
        }

        // Reset state
        this.connectionState = 'disconnected';
        this.isConnecting = false;
        this.reconnectAttempts = 0;
        this.messageQueue = [];
        this.messageBuffer = [];
        this.eventHistory.clear();

        this.emitEvent('connection', { state: 'disconnected', timestamp: Date.now() });
    }
}

// Export singleton instance
export const optimizedWebSocketClient = new OptimizedWebSocketClient();

// Legacy export for backward compatibility
export const webSocketClient = {
    connect: () => optimizedWebSocketClient.connect(),
    disconnect: () => optimizedWebSocketClient.disconnect(),
    sendMessage: (type: string, data: any) => optimizedWebSocketClient.sendMessage(type, data),
    getMetrics: () => optimizedWebSocketClient.getMetrics(),
    getConnectionState: () => optimizedWebSocketClient.getConnectionState()
};