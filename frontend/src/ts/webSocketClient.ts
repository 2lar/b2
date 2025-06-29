import { auth } from './authClient';

let socket: WebSocket | null = null;
let reconnectTimer: ReturnType<typeof setTimeout> | null = null;
const WEBSOCKET_URL = import.meta.env.VITE_WEBSOCKET_URL;

async function connect() {
    if (socket && socket.readyState === WebSocket.OPEN) {
        console.log('WebSocket is already open.');
        return;
    }

    if (!WEBSOCKET_URL) {
        console.error("VITE_WEBSOCKET_URL is not defined in the environment.");
        return;
    }

    const token = await auth.getJwtToken();
    if (!token) {
        console.error('Authentication token not found. Cannot connect WebSocket.');
        return;
    }

    const urlWithToken = `${WEBSOCKET_URL}?token=${token}`;
    socket = new WebSocket(urlWithToken);

    socket.onopen = () => {
        console.log('WebSocket connection established.');
        if (reconnectTimer) {
            clearTimeout(reconnectTimer);
            reconnectTimer = null;
        }
    };

    socket.onmessage = (event) => {
        try {
            const message = JSON.parse(event.data);
            if (message.action === 'graphUpdated') {
                console.log('Received graph-update-event');
                document.dispatchEvent(new CustomEvent('graph-update-event'));
            }
        } catch (err) {
            console.error('Error parsing message:', err);
        }
    };

    socket.onclose = (event) => {
        console.warn(`WebSocket closed. Code: ${event.code}. Reconnecting in 3 seconds...`);
        if (!reconnectTimer) {
            reconnectTimer = setTimeout(connect, 3000);
        }
    };

    socket.onerror = (error) => {
        console.error('WebSocket error:', error);
        socket?.close(); // This will trigger the onclose event handler for reconnection
    };
}

function disconnect() {
    if (reconnectTimer) {
        clearTimeout(reconnectTimer);
        reconnectTimer = null;
    }
    if (socket) {
        socket.close(1000, "User logged out");
        socket = null;
    }
}

export const webSocketClient = {
    connect,
    disconnect,
};
