implement the organic, animated addition of new nodes to your graph visualization. This will create a much more fluid and intuitive user experience, moving away from the jarring full-refresh model. Here’s a complete guide on how to implement this feature, including the necessary changes across your frontend and backend.

The Plan
Backend (ws-send-message Lambda): We'll modify the WebSocket message to include the ID of the newly created node.

Frontend (webSocketClient.ts): We'll update the WebSocket client to parse this new nodeId and pass it along in a custom event.

Frontend (app.ts): The main application will listen for this event, fetch the full details of the new node (with a retry mechanism to handle eventual consistency), and then trigger the animation.

Frontend (graph-viz.ts): This is where the magic happens. We'll create a new addNodeAndAnimate function that:

Adds the new node, making it "bubble up" from a single point.

Animates the creation of its edges.

Runs a localized layout so it settles into place without disrupting the rest of the graph.

Does all of this without resetting the user's current zoom and pan.

1. Backend Change: Add nodeId to WebSocket Message
First, we need the backend to tell the frontend which node was just created.

📝 File to Modify: backend/cmd/ws-send-message/main.go
Update the handler function to marshal the nodeId into the JSON payload sent over the WebSocket.

Go

// ... (imports)

// EdgesCreatedEvent represents graph change events from EventBridge
type EdgesCreatedEvent struct {
	UserID string `json:"userId"`
	NodeID string `json:"nodeId"`
}

func handler(ctx context.Context, event events.EventBridgeEvent) error {
	var detail EdgesCreatedEvent
	if err := json.Unmarshal(event.Detail, &detail); err != nil {
		log.Printf("ERROR: could not unmarshal event detail: %v", err)
		return err
	}

	// ... (code to query connections) ...

	// MODIFICATION: Create a dynamic message with the nodeId
	message, err := json.Marshal(map[string]string{
		"action": "graphUpdated",
		"nodeId": detail.NodeID,
	})
	if err != nil {
		log.Printf("ERROR: Failed to marshal WebSocket message: %v", err)
		// Don't return, as we can still try to notify other clients
	}

	for _, item := range result.Items {
		connectionID := strings.TrimPrefix(item["SK"].(*types.AttributeValueMemberS).Value, "CONN#")

		_, err := apiGatewayManagementClient.PostToConnection(ctx, &apigatewaymanagementapi.PostToConnectionInput{
			ConnectionId: &connectionID,
			Data:         message, // Use the new message here
		})

		if err != nil {
			// ... (error handling) ...
		}
	}

	return nil
}

// ... (main function)
2. Frontend Changes: Implementing the Animation
Now, let's update the frontend to handle this new, more informative WebSocket message and perform the animation.

📝 File to Modify: frontend/src/ts/webSocketClient.ts
Update the onmessage handler to extract the nodeId and include it in the detail of the dispatched graph-update-event.

TypeScript

// ... (imports and existing code)

    // Handle incoming messages
    socket.onmessage = (event) => {
        try {
            const message = JSON.parse(event.data);
            
            // MODIFICATION: Check for the action and pass the nodeId
            if (message.action === 'graphUpdated' && message.nodeId) {
                console.log(`Received graph-update-event for node: ${message.nodeId}`);
                document.dispatchEvent(new CustomEvent('graph-update-event', {
                    detail: { nodeId: message.nodeId }
                }));
            }
        } catch (err) {
            console.error('Error parsing message:', err);
        }
    };
    
// ... (rest of the file)
📝 File to Modify: frontend/src/ts/app.ts
Here, we'll change the event listener to call a new animation function instead of a full refresh. We also add a helper function to fetch node details with retries, which is crucial for dealing with the slight delay (eventual consistency) in a distributed system.

TypeScript

// ... (imports)
import { api } from './apiClient';
// ...
type NodeDetails = components['schemas']['NodeDetails'];

// ... (variable declarations)

let graphViz: {
    initGraph: () => void; 
    refreshGraph: () => Promise<void>;
    destroyGraph: () => void;
    // ADD THIS NEW FUNCTION DEFINITION
    addNodeAndAnimate: (nodeDetails: NodeDetails) => Promise<void>; 
} | null = null;

async function init(): Promise<void> {
    // ... (existing init code)

    // MODIFICATION: Update the event listener
    document.addEventListener('graph-update-event', async (event: Event) => {
        const customEvent = event as CustomEvent;
        console.log("Graph update event received in app.ts");
        if (graphViz && customEvent.detail.nodeId) {
            showStatus('New connections found! Updating graph...', 'success');
            
            // Fetch node details with retries to handle eventual consistency
            const nodeDetails = await fetchNodeWithRetries(customEvent.detail.nodeId);

            if (nodeDetails) {
                // Pass the fetched data directly to the animation function
                await graphViz.addNodeAndAnimate(nodeDetails);
            } else {
                // Fallback to a full refresh only if fetching fails completely
                console.error(`Failed to fetch details for new node ${customEvent.detail.nodeId}. Performing full refresh.`);
                await graphViz.refreshGraph();
            }
        }
    });

    // ... (rest of init function)
}

/**
 * NEW FUNCTION
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

// ... (rest of app.ts)
📝 File to Modify: frontend/src/ts/graph-viz.ts
This is the core of the implementation. Add the new addNodeAndAnimate function and a new style for newly added nodes.

TypeScript

import cytoscape, { Core, LayoutOptions, ElementDefinition, AnimationOptions } from 'cytoscape';
import { api } from './apiClient';
import { components } from './generated-types';

type NodeDetails = components['schemas']['NodeDetails'];
// ... (existing variables)

export function initGraph(): void {
    // ... (inside initGraph, add the new style to the style array)
    style: [
        // ... (existing styles)
        {
            selector: 'node.newly-added',
            style: { 'background-color': '#10b981', 'border-color': '#059669', 'border-width': 3 }
        },
    ],
    // ... (rest of initGraph)
}

// ... (existing functions like destroyGraph, highlightConnectedNodes, etc.)

/**
 * NEW FUNCTION
 * Organically adds and animates a new node and its edges into the graph
 * without a full layout refresh, preserving the user's viewport.
 */
export async function addNodeAndAnimate(nodeDetails: NodeDetails): Promise<void> {
    if (!cy) return;

    // 1. Don't add if the node is already on the graph
    if (cy.getElementById(nodeDetails.nodeId!).length > 0) {
        console.warn(`Node ${nodeDetails.nodeId} already exists. Skipping animation.`);
        return;
    }

    const existingNodes = cy.nodes();

    try {
        // 2. Lock existing nodes to prevent them from moving during the animation
        existingNodes.lock();

        // 3. Determine a smart initial position for the new node
        let initialPosition = { x: cy.pan().x, y: cy.pan().y }; // Default to center of viewport
        const connectedNodeIds = (nodeDetails.edges || []);
        if (connectedNodeIds.length > 0) {
            // Find neighbors that are already in the graph
            const neighborNodes = cy.nodes().filter(node => connectedNodeIds.includes(node.id()));
            if (neighborNodes.length > 0) {
                // Position the new node at the center of its neighbors
                const bb = neighborNodes.boundingBox();
                initialPosition = { x: bb.x1 + bb.w / 2, y: bb.y1 + bb.h / 2 };
            }
        }

        // 4. Define the new node with initial styles for the "bubble-up" animation
        const label = nodeDetails.content ? (nodeDetails.content.length > 50 ? nodeDetails.content.substring(0, 47) + '...' : nodeDetails.content) : '';
        const newNodeElement: ElementDefinition = {
            group: 'nodes',
            data: { id: nodeDetails.nodeId, label: label },
            style: { 'opacity': 0, 'width': 1, 'height': 1 },
            position: initialPosition,
            classes: 'newly-added' // Temporary class for styling
        };
        
        // 5. Define new edges, initially invisible
        const newEdgeElements: ElementDefinition[] = (nodeDetails.edges || []).map(targetId => ({
            group: 'edges',
            data: { id: `edge-${nodeDetails.nodeId}-${targetId}`, source: nodeDetails.nodeId!, target: targetId },
            style: { 'opacity': 0 }
        }));

        // 6. Add and animate the node
        const addedNode = cy.add(newNodeElement);
        await addedNode.animation({
            style: { 'opacity': 1, 'width': 50, 'height': 50 },
            duration: 800,
            easing: 'ease-out-cubic'
        } as any).play().promise();

        // 7. Add and animate the edges with a slight stagger
        for (const edgeDef of newEdgeElements) {
            cy.add(edgeDef).animation({
                style: { 'opacity': 0.7 },
                duration: 600
            } as any).play();
            await new Promise(resolve => setTimeout(resolve, 150)); // Stagger delay
        }

        // 8. Run a gentle, localized layout on only the new node and its neighbors
        const layout = cy.layout({
            name: 'cose',
            eles: addedNode.union(addedNode.neighborhood()),
            fit: false, // CRITICAL: Do not fit the viewport
            animate: true,
            animationDuration: 1000,
            padding: 80
        } as any);
        layout.run();

        // 9. Clean up after the animation
        setTimeout(() => {
            addedNode.removeClass('newly-added');
            existingNodes.unlock();
        }, 2500);

    } catch (error) {
        console.error('Error adding and animating node:', error);
        existingNodes.unlock();
        await refreshGraph(); // Fallback to a full refresh on error
    }
}


// ... (rest of graph-viz.ts)