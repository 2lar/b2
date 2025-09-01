package projections

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
	
	"brain2-backend/internal/domain/shared"
)

// AuditEntry represents a single audit log entry
type AuditEntry struct {
	ID            string                 `json:"id"`
	Timestamp     time.Time             `json:"timestamp"`
	UserID        string                `json:"userId"`
	AggregateID   string                `json:"aggregateId"`
	AggregateType string                `json:"aggregateType"`
	EventType     string                `json:"eventType"`
	EventData     map[string]interface{} `json:"eventData"`
	Version       int                   `json:"version"`
	IPAddress     string                `json:"ipAddress,omitempty"`
	UserAgent     string                `json:"userAgent,omitempty"`
	SessionID     string                `json:"sessionId,omitempty"`
	CorrelationID string                `json:"correlationId,omitempty"`
	
	// Computed fields for analysis
	Action      string `json:"action"`      // Human-readable action description
	Resource    string `json:"resource"`    // Resource identifier
	OldValue    string `json:"oldValue,omitempty"`
	NewValue    string `json:"newValue,omitempty"`
	Impact      string `json:"impact"`      // LOW, MEDIUM, HIGH
	Category    string `json:"category"`    // CREATE, UPDATE, DELETE, ACCESS
}

// AuditProjection handles the creation of audit logs from domain events
type AuditProjection struct {
	store AuditStore
}

// AuditStore persists audit entries
type AuditStore interface {
	SaveAuditEntry(ctx context.Context, entry AuditEntry) error
	GetAuditLog(ctx context.Context, filter AuditFilter) ([]AuditEntry, error)
	GetUserActivity(ctx context.Context, userID string, from, to time.Time) ([]AuditEntry, error)
	GetResourceHistory(ctx context.Context, resourceID string) ([]AuditEntry, error)
	GetAuditStats(ctx context.Context, from, to time.Time) (*AuditStats, error)
}

// AuditFilter defines criteria for filtering audit logs
type AuditFilter struct {
	UserID        string    `json:"userId,omitempty"`
	AggregateID   string    `json:"aggregateId,omitempty"`
	EventType     string    `json:"eventType,omitempty"`
	Category      string    `json:"category,omitempty"`
	FromTimestamp time.Time `json:"fromTimestamp,omitempty"`
	ToTimestamp   time.Time `json:"toTimestamp,omitempty"`
	Limit         int       `json:"limit,omitempty"`
	Offset        int       `json:"offset,omitempty"`
}

// AuditStats provides statistical information about audit logs
type AuditStats struct {
	TotalEvents      int                       `json:"totalEvents"`
	EventsByType     map[string]int           `json:"eventsByType"`
	EventsByUser     map[string]int           `json:"eventsByUser"`
	EventsByCategory map[string]int           `json:"eventsByCategory"`
	MostActiveUsers  []UserActivity           `json:"mostActiveUsers"`
	RecentActivity   []AuditEntry             `json:"recentActivity"`
}

// UserActivity represents user activity statistics
type UserActivity struct {
	UserID      string `json:"userId"`
	EventCount  int    `json:"eventCount"`
	LastActive  time.Time `json:"lastActive"`
}

// NewAuditProjection creates a new audit projection
func NewAuditProjection(store AuditStore) *AuditProjection {
	return &AuditProjection{
		store: store,
	}
}

// Handle processes a domain event and creates an audit entry
func (p *AuditProjection) Handle(ctx context.Context, event shared.DomainEvent) error {
	entry := p.createAuditEntry(event)
	
	// Enrich with context information if available
	if ctxValue := ctx.Value("ip_address"); ctxValue != nil {
		entry.IPAddress = ctxValue.(string)
	}
	if ctxValue := ctx.Value("user_agent"); ctxValue != nil {
		entry.UserAgent = ctxValue.(string)
	}
	if ctxValue := ctx.Value("session_id"); ctxValue != nil {
		entry.SessionID = ctxValue.(string)
	}
	if ctxValue := ctx.Value("correlation_id"); ctxValue != nil {
		entry.CorrelationID = ctxValue.(string)
	}
	
	return p.store.SaveAuditEntry(ctx, entry)
}

// createAuditEntry transforms a domain event into an audit entry
func (p *AuditProjection) createAuditEntry(event shared.DomainEvent) AuditEntry {
	entry := AuditEntry{
		ID:            fmt.Sprintf("audit_%s_%d", event.EventID(), time.Now().UnixNano()),
		Timestamp:     event.Timestamp(),
		UserID:        event.UserID(),
		AggregateID:   event.AggregateID(),
		EventType:     event.EventType(),
		EventData:     event.EventData(),
		Version:       event.Version(),
	}
	
	// Determine aggregate type and enrich entry
	switch event.EventType() {
	case "NodeCreated":
		entry.AggregateType = "Node"
		entry.Action = "Created new node"
		entry.Resource = fmt.Sprintf("node:%s", event.AggregateID())
		entry.Category = "CREATE"
		entry.Impact = "MEDIUM"
		if content, ok := event.EventData()["content"].(string); ok {
			entry.NewValue = truncate(content, 100)
		}
		
	case "NodeUpdated":
		entry.AggregateType = "Node"
		entry.Action = "Updated node content"
		entry.Resource = fmt.Sprintf("node:%s", event.AggregateID())
		entry.Category = "UPDATE"
		entry.Impact = "LOW"
		if oldContent, ok := event.EventData()["oldContent"].(string); ok {
			entry.OldValue = truncate(oldContent, 100)
		}
		if newContent, ok := event.EventData()["content"].(string); ok {
			entry.NewValue = truncate(newContent, 100)
		}
		
	case "NodeArchived":
		entry.AggregateType = "Node"
		entry.Action = "Archived node"
		entry.Resource = fmt.Sprintf("node:%s", event.AggregateID())
		entry.Category = "UPDATE"
		entry.Impact = "MEDIUM"
		
	case "NodeDeleted":
		entry.AggregateType = "Node"
		entry.Action = "Deleted node"
		entry.Resource = fmt.Sprintf("node:%s", event.AggregateID())
		entry.Category = "DELETE"
		entry.Impact = "HIGH"
		
	case "CategoryCreated":
		entry.AggregateType = "Category"
		entry.Action = "Created new category"
		entry.Resource = fmt.Sprintf("category:%s", event.AggregateID())
		entry.Category = "CREATE"
		entry.Impact = "MEDIUM"
		if name, ok := event.EventData()["name"].(string); ok {
			entry.NewValue = name
		}
		
	case "CategoryUpdated":
		entry.AggregateType = "Category"
		entry.Action = "Updated category"
		entry.Resource = fmt.Sprintf("category:%s", event.AggregateID())
		entry.Category = "UPDATE"
		entry.Impact = "LOW"
		
	case "CategoryMoved":
		entry.AggregateType = "Category"
		entry.Action = "Moved category"
		entry.Resource = fmt.Sprintf("category:%s", event.AggregateID())
		entry.Category = "UPDATE"
		entry.Impact = "MEDIUM"
		if oldParent, ok := event.EventData()["oldParentId"].(string); ok {
			entry.OldValue = fmt.Sprintf("parent:%s", oldParent)
		}
		if newParent, ok := event.EventData()["parentId"].(string); ok {
			entry.NewValue = fmt.Sprintf("parent:%s", newParent)
		}
		
	case "CategoryDeleted":
		entry.AggregateType = "Category"
		entry.Action = "Deleted category"
		entry.Resource = fmt.Sprintf("category:%s", event.AggregateID())
		entry.Category = "DELETE"
		entry.Impact = "HIGH"
		
	case "EdgeCreated":
		entry.AggregateType = "Edge"
		entry.Action = "Created connection"
		entry.Resource = fmt.Sprintf("edge:%s", event.AggregateID())
		entry.Category = "CREATE"
		entry.Impact = "LOW"
		if sourceID, ok := event.EventData()["sourceId"].(string); ok {
			if targetID, ok2 := event.EventData()["targetId"].(string); ok2 {
				entry.NewValue = fmt.Sprintf("%s -> %s", sourceID, targetID)
			}
		}
		
	case "EdgeDeleted":
		entry.AggregateType = "Edge"
		entry.Action = "Deleted connection"
		entry.Resource = fmt.Sprintf("edge:%s", event.AggregateID())
		entry.Category = "DELETE"
		entry.Impact = "LOW"
		
	default:
		entry.AggregateType = "Unknown"
		entry.Action = fmt.Sprintf("Performed %s", event.EventType())
		entry.Resource = event.AggregateID()
		entry.Category = "UPDATE"
		entry.Impact = "LOW"
	}
	
	return entry
}

// ComplianceReport generates a compliance report from audit logs
type ComplianceReport struct {
	Period           string                 `json:"period"`
	StartDate        time.Time             `json:"startDate"`
	EndDate          time.Time             `json:"endDate"`
	TotalEvents      int                   `json:"totalEvents"`
	DataModifications int                   `json:"dataModifications"`
	DataDeletions    int                   `json:"dataDeletions"`
	UserAccess       map[string]int        `json:"userAccess"`
	SensitiveActions []AuditEntry          `json:"sensitiveActions"`
	Anomalies        []AnomalyDetection    `json:"anomalies"`
	ComplianceStatus string                `json:"complianceStatus"`
}

// AnomalyDetection represents detected anomalies in audit logs
type AnomalyDetection struct {
	Type        string    `json:"type"`
	Description string    `json:"description"`
	Severity    string    `json:"severity"`
	Timestamp   time.Time `json:"timestamp"`
	UserID      string    `json:"userId"`
	Details     map[string]interface{} `json:"details"`
}

// GenerateComplianceReport creates a compliance report for a given period
func (p *AuditProjection) GenerateComplianceReport(ctx context.Context, from, to time.Time) (*ComplianceReport, error) {
	filter := AuditFilter{
		FromTimestamp: from,
		ToTimestamp:   to,
	}
	
	entries, err := p.store.GetAuditLog(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve audit logs: %w", err)
	}
	
	report := &ComplianceReport{
		Period:           fmt.Sprintf("%s to %s", from.Format("2006-01-02"), to.Format("2006-01-02")),
		StartDate:        from,
		EndDate:          to,
		TotalEvents:      len(entries),
		UserAccess:       make(map[string]int),
		SensitiveActions: []AuditEntry{},
		Anomalies:        []AnomalyDetection{},
	}
	
	// Analyze entries
	userEventCounts := make(map[string]map[string]int) // userID -> eventType -> count
	
	for _, entry := range entries {
		// Count user access
		report.UserAccess[entry.UserID]++
		
		// Track event types per user for anomaly detection
		if userEventCounts[entry.UserID] == nil {
			userEventCounts[entry.UserID] = make(map[string]int)
		}
		userEventCounts[entry.UserID][entry.EventType]++
		
		// Count modifications and deletions
		switch entry.Category {
		case "UPDATE":
			report.DataModifications++
		case "DELETE":
			report.DataDeletions++
			// Deletions are sensitive actions
			report.SensitiveActions = append(report.SensitiveActions, entry)
		}
		
		// High impact actions are sensitive
		if entry.Impact == "HIGH" {
			report.SensitiveActions = append(report.SensitiveActions, entry)
		}
	}
	
	// Detect anomalies
	report.Anomalies = p.detectAnomalies(entries, userEventCounts)
	
	// Determine compliance status
	if len(report.Anomalies) > 0 {
		report.ComplianceStatus = "REQUIRES_REVIEW"
	} else if report.DataDeletions > 10 {
		report.ComplianceStatus = "WARNING"
	} else {
		report.ComplianceStatus = "COMPLIANT"
	}
	
	return report, nil
}

// detectAnomalies identifies unusual patterns in audit logs
func (p *AuditProjection) detectAnomalies(entries []AuditEntry, userEventCounts map[string]map[string]int) []AnomalyDetection {
	anomalies := []AnomalyDetection{}
	
	// Detect bulk operations (more than 50 similar operations in 5 minutes)
	timeWindows := make(map[string][]AuditEntry)
	for _, entry := range entries {
		window := entry.Timestamp.Truncate(5 * time.Minute).Format("2006-01-02T15:04:05")
		key := fmt.Sprintf("%s_%s_%s", entry.UserID, entry.EventType, window)
		timeWindows[key] = append(timeWindows[key], entry)
	}
	
	for _, windowEntries := range timeWindows {
		if len(windowEntries) > 50 {
			anomalies = append(anomalies, AnomalyDetection{
				Type:        "BULK_OPERATION",
				Description: fmt.Sprintf("Detected %d operations in 5-minute window", len(windowEntries)),
				Severity:    "MEDIUM",
				Timestamp:   windowEntries[0].Timestamp,
				UserID:      windowEntries[0].UserID,
				Details: map[string]interface{}{
					"eventType": windowEntries[0].EventType,
					"count":     len(windowEntries),
				},
			})
		}
	}
	
	// Detect unusual deletion patterns (more than 10 deletions by a user)
	for userID, eventCounts := range userEventCounts {
		deleteCount := 0
		for eventType, count := range eventCounts {
			if eventType == "NodeDeleted" || eventType == "CategoryDeleted" || eventType == "EdgeDeleted" {
				deleteCount += count
			}
		}
		
		if deleteCount > 10 {
			anomalies = append(anomalies, AnomalyDetection{
				Type:        "EXCESSIVE_DELETIONS",
				Description: fmt.Sprintf("User performed %d deletion operations", deleteCount),
				Severity:    "HIGH",
				UserID:      userID,
				Details: map[string]interface{}{
					"deleteCount": deleteCount,
				},
			})
		}
	}
	
	return anomalies
}

// ExportAuditLog exports audit logs in various formats
func (p *AuditProjection) ExportAuditLog(ctx context.Context, filter AuditFilter, format string) ([]byte, error) {
	entries, err := p.store.GetAuditLog(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve audit logs: %w", err)
	}
	
	switch format {
	case "json":
		return json.MarshalIndent(entries, "", "  ")
	case "csv":
		return p.exportToCSV(entries)
	default:
		return nil, fmt.Errorf("unsupported export format: %s", format)
	}
}

// exportToCSV converts audit entries to CSV format
func (p *AuditProjection) exportToCSV(entries []AuditEntry) ([]byte, error) {
	// Simple CSV export implementation
	csv := "Timestamp,UserID,Action,Resource,Category,Impact\n"
	for _, entry := range entries {
		csv += fmt.Sprintf("%s,%s,%s,%s,%s,%s\n",
			entry.Timestamp.Format(time.RFC3339),
			entry.UserID,
			entry.Action,
			entry.Resource,
			entry.Category,
			entry.Impact,
		)
	}
	return []byte(csv), nil
}

// Helper function to truncate strings
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}