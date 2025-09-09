package events

// Event sources - These define where events originate from
const (
	// SourceBackend is the primary backend service source
	SourceBackend = "brain2.backend"
	
	// SourceConnectNode is the connect-node Lambda source
	SourceConnectNode = "brain2.connectNode"
	
	// SourceCleanup is the cleanup Lambda source
	SourceCleanup = "brain2.cleanup"
)

// Event types - These define the types of events in the system
const (
	// Node events
	TypeNodeCreated            = "node.created"
	TypeNodeUpdated            = "node.updated"
	TypeNodeDeleted            = "node.deleted"
	TypeNodeArchived           = "node.archived"
	TypeNodeRestored           = "node.restored"
	TypeNodeCreatedWithPending = "node.created.with.pending.edges"
	
	// Edge events
	TypeEdgesCreated   = "edges.created"
	TypeEdgeDeleted    = "edge.deleted"
	TypeEdgesDiscovered = "edges.discovered"
	
	// Graph events
	TypeGraphCreated = "graph.created"
	TypeGraphUpdated = "graph.updated"
	TypeGraphDeleted = "graph.deleted"
	
	// Cleanup events
	TypeCleanupInitiated = "cleanup.initiated"
	TypeCleanupCompleted = "cleanup.completed"
)

// Event detail keys - Common keys used in event details
const (
	DetailNodeID       = "nodeId"
	DetailGraphID      = "graphId"
	DetailUserID       = "userId"
	DetailEdgeCount    = "edgeCount"
	DetailSyncEdges    = "syncEdgesCreated"
	DetailAsyncPending = "asyncEdgesPending"
)