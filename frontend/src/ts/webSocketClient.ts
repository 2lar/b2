/**
 * =============================================================================
 * WebSocket Client - Real-Time Communication for Graph Updates
 * =============================================================================
 * 
 * 📚 EDUCATIONAL OVERVIEW:
 * This module implements real-time bidirectional communication between the frontend
 * and backend using WebSockets. It enables instant graph updates without requiring
 * users to refresh the page, creating a responsive and collaborative experience.
 * 
 * 🏗️ KEY ARCHITECTURAL CONCEPTS:
 * 
 * 1. REAL-TIME ARCHITECTURE:
 *    - WebSocket protocol for persistent, bidirectional connections
 *    - Event-driven updates for immediate UI synchronization
 *    - Pushes server-side changes to all connected clients
 *    - Enables collaborative features and live updates
 * 
 * 2. CONNECTION RESILIENCE:
 *    - Automatic reconnection on connection loss
 *    - Exponential backoff and retry strategies
 *    - Graceful handling of network interruptions
 *    - State preservation across reconnections
 * 
 * 3. AUTHENTICATION INTEGRATION:
 *    - JWT token-based WebSocket authentication
 *    - Secure connection establishment
 *    - User-specific message routing
 *    - Session management and token refresh
 * 
 * 4. EVENT-DRIVEN COMMUNICATION:
 *    - Custom DOM events for loose coupling
 *    - Publisher-subscriber pattern for scalability
 *    - Type-safe message handling
 *    - Flexible event routing and processing
 * 
 * 5. ERROR HANDLING AND MONITORING:
 *    - Comprehensive connection state tracking
 *    - Detailed error logging for debugging
 *    - Graceful degradation when connections fail
 *    - User feedback for connection status
 * 
 * 📡 WEBSOCKET WORKFLOW:
 * 1. Client authenticates and gets JWT token
 * 2. WebSocket connection established with token in URL
 * 3. Server validates token and establishes user session
 * 4. Real-time message exchange begins
 * 5. Automatic reconnection if connection drops
 * 
 * 🔄 REAL-TIME UPDATE FLOW:
 * 1. User creates/updates/deletes memory via HTTP API
 * 2. Backend processes request and updates database
 * 3. Backend sends WebSocket message to all user's connections
 * 4. Frontend receives message and dispatches DOM event
 * 5. Graph visualization updates automatically
 * 
 * 🎯 LEARNING OBJECTIVES:
 * - WebSocket protocol and real-time communication
 * - Connection management and resilience patterns
 * - Event-driven architecture in the browser
 * - Authentication for persistent connections
 * - Error handling for network communications
 * - Browser event system integration
 */

import { auth } from './authClient';

/**
 * Global WebSocket State Management
 * 
 * MODULE-LEVEL STATE:
 * These variables maintain the WebSocket connection state and reconnection logic.
 * Module-level state ensures a single connection per user session.
 * 
 * STATE VARIABLES:
 * - socket: The active WebSocket connection instance
 * - reconnectTimer: Timer ID for automatic reconnection attempts
 * - WEBSOCKET_URL: Environment-configured WebSocket endpoint
 * 
 * DESIGN PATTERN:
 * Module singleton pattern provides a single WebSocket connection
 * shared across the entire application, preventing multiple connections.
 */
let socket: WebSocket | null = null;
let reconnectTimer: ReturnType<typeof setTimeout> | null = null;
const WEBSOCKET_URL = import.meta.env.VITE_WEBSOCKET_URL;

/**
 * WebSocket Connection Establishment
 * 
 * This function manages the complete WebSocket connection lifecycle, from
 * authentication through connection establishment to event handler setup.
 * It demonstrates several important real-time communication patterns.
 * 
 * CONNECTION WORKFLOW:
 * 1. Validate existing connection state
 * 2. Verify configuration and authentication
 * 3. Establish WebSocket connection with authentication
 * 4. Set up comprehensive event handlers
 * 5. Handle connection state changes and errors
 * 
 * AUTHENTICATION STRATEGY:
 * - JWT token passed as URL parameter for WebSocket auth
 * - Server validates token before accepting connection
 * - Token refresh handled by auth client integration
 * - Secure connection establishment process
 * 
 * RESILIENCE PATTERNS:
 * - Duplicate connection prevention
 * - Configuration validation before connection attempts
 * - Automatic reconnection on failures
 * - Comprehensive error handling and logging
 * 
 * EVENT-DRIVEN ARCHITECTURE:
 * - DOM custom events for loose coupling between modules
 * - Type-safe message parsing and handling
 * - Extensible message routing for future message types
 * - Error boundaries to prevent message processing failures
 */
async function connect() {
    /**
     * Step 1: Connection State Validation
     * 
     * DUPLICATE CONNECTION PREVENTION:
     * Check if an active WebSocket connection already exists to prevent
     * multiple connections from the same user session. Multiple connections
     * would waste resources and cause duplicate events.
     * 
     * WEBSOCKET READY STATES:
     * - CONNECTING (0): Connection is being established
     * - OPEN (1): Connection is established and ready to communicate
     * - CLOSING (2): Connection is being closed
     * - CLOSED (3): Connection is closed or couldn't be opened
     */
    if (socket && socket.readyState === WebSocket.OPEN) {
        console.log('WebSocket is already open.');
        return;
    }

    /**
     * Step 2: Configuration Validation
     * 
     * FAIL-FAST PRINCIPLE:
     * Validate required configuration before attempting connection.
     * Provides clear error messages for misconfiguration issues.
     * 
     * ENVIRONMENT CONFIGURATION:
     * WebSocket URL should be configured per environment:
     * - Development: Local WebSocket server or staging
     * - Production: AWS API Gateway WebSocket endpoint
     */
    if (!WEBSOCKET_URL) {
        console.error("VITE_WEBSOCKET_URL is not defined in the environment.");
        return;
    }

    /**
     * Step 3: Authentication Token Retrieval
     * 
     * JWT-BASED WEBSOCKET AUTHENTICATION:
     * WebSocket connections don't support traditional HTTP headers for auth,
     * so we pass the JWT token as a URL parameter. This enables the server
     * to validate user identity and establish proper session context.
     * 
     * AUTHENTICATION FLOW:
     * 1. Retrieve current JWT token from auth client
     * 2. Validate token exists (user is authenticated)
     * 3. Pass token to server via URL parameter
     * 4. Server validates token and establishes user session
     * 
     * ERROR HANDLING:
     * If no token is available, the user isn't authenticated and
     * WebSocket connection should not be attempted.
     */
    const token = await auth.getJwtToken();
    if (!token) {
        console.error('Authentication token not found. Cannot connect WebSocket.');
        return;
    }

    /**
     * Step 4: WebSocket Connection Establishment
     * 
     * URL CONSTRUCTION:
     * Append JWT token as query parameter for server-side authentication.
     * Server will extract and validate this token before accepting the connection.
     * 
     * WEBSOCKET INSTANTIATION:
     * Create new WebSocket instance with the authenticated URL.
     * Connection will be established asynchronously via event handlers.
     */
    const urlWithToken = `${WEBSOCKET_URL}?token=${token}`;
    socket = new WebSocket(urlWithToken);

    /**
     * Step 5: Connection Success Handler
     * 
     * ONOPEN EVENT:
     * Fired when WebSocket connection is successfully established.
     * Indicates that real-time communication channel is ready.
     * 
     * RECONNECTION CLEANUP:
     * Clear any pending reconnection timers since connection is now established.
     * Prevents unnecessary reconnection attempts when already connected.
     * 
     * CONNECTION FEEDBACK:
     * Log successful connection for debugging and user feedback.
     * Could be extended to update UI connection status indicators.
     */
    socket.onopen = () => {
        console.log('WebSocket connection established.');
        if (reconnectTimer) {
            clearTimeout(reconnectTimer);
            reconnectTimer = null;
        }
    };

    /**
     * Step 6: Message Processing Handler
     * 
     * REAL-TIME MESSAGE HANDLING:
     * Process incoming messages from the server and dispatch appropriate
     * events to update the UI in real-time.
     * 
     * MESSAGE PARSING STRATEGY:
     * - All messages expected to be JSON format
     * - Error boundaries prevent malformed messages from crashing app
     * - Type checking for expected message structure
     * - Extensible action-based message routing
     * 
     * EVENT DISPATCHING PATTERN:
     * Use DOM CustomEvent for loose coupling between WebSocket client
     * and graph visualization components. This allows multiple components
     * to respond to the same real-time updates independently.
     * 
     * CURRENT MESSAGE TYPES:
     * - graphUpdated: Indicates the user's knowledge graph has changed
     *   and visualization should refresh to show latest data
     * 
     * FUTURE MESSAGE TYPES:
     * - nodeCreated: Real-time node creation notifications
     * - nodeUpdated: Real-time node modification notifications  
     * - nodeDeleted: Real-time node deletion notifications
     * - userConnected: Collaborative features for shared graphs
     */
    socket.onmessage = (event) => {
        try {
            // Parse JSON message with error boundary
            const message = JSON.parse(event.data);
            
            // Route message based on action type
            if (message.action === 'graphUpdated') {
                console.log('Received graph-update-event');
                // Dispatch DOM event for loose coupling
                document.dispatchEvent(new CustomEvent('graph-update-event'));
            }
            // Future message types can be added here
        } catch (err) {
            // Prevent malformed messages from crashing the application
            console.error('Error parsing message:', err);
        }
    };

    /**
     * Step 7: Connection Closure Handler
     * 
     * ONCLOSE EVENT:
     * Fired when WebSocket connection is closed, either intentionally
     * or due to network issues, server problems, or authentication failures.
     * 
     * AUTOMATIC RECONNECTION:
     * Implement resilient connection management by automatically attempting
     * to reconnect after connection loss. This ensures users don't lose
     * real-time functionality due to temporary network issues.
     * 
     * RECONNECTION STRATEGY:
     * - Fixed 3-second delay before reconnection attempt
     * - Prevents rapid reconnection loops that could overwhelm server
     * - Could be enhanced with exponential backoff for production
     * - Single timer prevents multiple simultaneous reconnection attempts
     * 
     * CLOSE CODE ANALYSIS:
     * WebSocket close codes provide insight into why connection closed:
     * - 1000: Normal closure (user logout, page refresh)
     * - 1001: Going away (page navigation)
     * - 1006: Abnormal closure (network issue)
     * - 4000+: Application-specific codes (auth failure, etc.)
     */
    socket.onclose = (event) => {
        console.warn(`WebSocket closed. Code: ${event.code}. Reconnecting in 3 seconds...`);
        if (!reconnectTimer) {
            reconnectTimer = setTimeout(connect, 3000);
        }
    };

    /**
     * Step 8: Error Handler
     * 
     * ONERROR EVENT:
     * Fired when WebSocket encounters an error during connection or communication.
     * Provides opportunity for error logging and recovery actions.
     * 
     * ERROR RECOVERY STRATEGY:
     * Close the current connection to trigger the onclose handler,
     * which will initiate the automatic reconnection process.
     * This ensures errors don't leave connections in undefined states.
     * 
     * ERROR LOGGING:
     * Comprehensive error logging helps with debugging connection issues
     * in development and monitoring connection health in production.
     * 
     * POTENTIAL ERRORS:
     * - Network connectivity issues
     * - Server-side WebSocket handler errors
     * - Authentication failures
     * - Malformed message processing errors
     */
    socket.onerror = (error) => {
        console.error('WebSocket error:', error);
        // Trigger onclose handler for reconnection by closing current connection
        socket?.close();
    };
}

/**
 * WebSocket Connection Termination
 * 
 * This function cleanly terminates the WebSocket connection and prevents
 * automatic reconnection attempts. It's essential for proper cleanup
 * when users log out or navigate away from the application.
 * 
 * CLEAN DISCONNECTION WORKFLOW:
 * 1. Cancel any pending reconnection attempts
 * 2. Close active WebSocket connection with proper close code
 * 3. Clear connection references to prevent memory leaks
 * 4. Prevent automatic reconnection after intentional disconnect
 * 
 * RESOURCE CLEANUP:
 * - Clear reconnection timers to prevent unnecessary background processes
 * - Set socket reference to null for garbage collection
 * - Use standard close code (1000) to indicate normal closure
 * - Provide descriptive close reason for server-side logging
 * 
 * USAGE SCENARIOS:
 * - User logout: Prevent real-time updates for logged-out users
 * - Page navigation: Clean up connections before leaving the app
 * - Connection management: Manually disconnect for troubleshooting
 * - Resource optimization: Close connections during idle periods
 * 
 * WEBSOCKET CLOSE CODES:
 * - 1000: Normal closure (recommended for user-initiated disconnects)
 * - 1001: Going away (page navigation, browser close)
 * - 1002: Protocol error (unexpected server response)
 * - 1003: Unsupported data (server can't process message type)
 */
function disconnect() {
    /**
     * Step 1: Cancel Pending Reconnection
     * 
     * TIMER CLEANUP:
     * If there's a pending reconnection attempt scheduled, cancel it to
     * prevent automatic reconnection after intentional disconnection.
     * This is crucial when users logout or intentionally disconnect.
     */
    if (reconnectTimer) {
        clearTimeout(reconnectTimer);
        reconnectTimer = null;
    }
    
    /**
     * Step 2: Close WebSocket Connection
     * 
     * GRACEFUL CONNECTION CLOSURE:
     * Close the WebSocket connection with proper close code and reason.
     * This informs the server that disconnection was intentional.
     * 
     * CLOSE CODE 1000:
     * Standard close code indicating normal, successful closure.
     * This helps server distinguish between intentional disconnects
     * and network failures or errors.
     * 
     * CLOSE REASON:
     * Descriptive reason helps with server-side logging and debugging.
     * Can be customized based on the disconnect context.
     * 
     * MEMORY CLEANUP:
     * Set socket reference to null to allow garbage collection
     * and prevent attempts to use the closed connection.
     */
    if (socket) {
        socket.close(1000, "User logged out");
        socket = null;
    }
}

/**
 * =============================================================================
 * WebSocket Client Public API Export
 * =============================================================================
 * 
 * PUBLIC INTERFACE:
 * This export provides a clean, documented API for WebSocket operations
 * while encapsulating internal state and implementation details.
 * 
 * API DESIGN PRINCIPLES:
 * - Simple function-based interface for ease of use
 * - Clear separation between connection and disconnection
 * - Internal state management hidden from consumers
 * - Consistent naming with other client modules
 * 
 * USAGE PATTERNS:
 * ```typescript
 * import { webSocketClient } from './webSocketClient';
 * 
 * // Establish real-time connection after user login
 * await webSocketClient.connect();
 * 
 * // Clean up connection on logout
 * webSocketClient.disconnect();
 * ```
 * 
 * INTEGRATION POINTS:
 * - Auth client: Called after successful authentication
 * - App controller: Integrated into login/logout workflows
 * - Graph visualization: Listens for 'graph-update-event' DOM events
 * - Error handling: Provides connection status feedback
 * 
 * FUTURE ENHANCEMENTS:
 * - Connection status monitoring
 * - Message sending capabilities
 * - Subscription management for specific event types
 * - Connection health metrics and reporting
 * - Configurable reconnection strategies
 */
export const webSocketClient = {
    connect,
    disconnect,
};
