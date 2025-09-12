package acl

import (
	"context"
	"fmt"
	"time"

	"backend/domain/core/entities"
	"backend/domain/core/valueobjects"
)

// ExternalAPIAdapter is an Anti-Corruption Layer that translates between
// external API responses and our domain models
type ExternalAPIAdapter interface {
	// TranslateToNode converts external data to our domain Node entity
	TranslateToNode(externalData interface{}) (*entities.Node, error)
	
	// TranslateFromNode converts our domain Node to external format
	TranslateFromNode(node *entities.Node) (interface{}, error)
	
	// ValidateExternalData ensures external data meets our domain requirements
	ValidateExternalData(data interface{}) error
}

// WebContentAdapter adapts web content (from WebFetch) to domain models
type WebContentAdapter struct {
	defaultUserID string
}

// NewWebContentAdapter creates a new web content adapter
func NewWebContentAdapter(defaultUserID string) *WebContentAdapter {
	return &WebContentAdapter{
		defaultUserID: defaultUserID,
	}
}

// WebContent represents external web content
type WebContent struct {
	URL         string                 `json:"url"`
	Title       string                 `json:"title"`
	Content     string                 `json:"content"`
	Metadata    map[string]interface{} `json:"metadata"`
	ExtractedAt time.Time              `json:"extracted_at"`
}

// TranslateToNode converts web content to a domain Node
func (w *WebContentAdapter) TranslateToNode(externalData interface{}) (*entities.Node, error) {
	webContent, ok := externalData.(*WebContent)
	if !ok {
		return nil, fmt.Errorf("invalid data type: expected WebContent")
	}

	// Validate the external data first
	if err := w.ValidateExternalData(webContent); err != nil {
		return nil, fmt.Errorf("validation failed: %w", err)
	}

	// Create domain value objects with proper validation
	content, err := valueobjects.NewNodeContent(
		webContent.Title,
		webContent.Content,
		valueobjects.FormatMarkdown,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create content: %w", err)
	}

	position, _ := valueobjects.NewPosition3D(0, 0, 0) // Default position

	// Create the domain entity
	node, err := entities.NewNode(w.defaultUserID, content, position)
	if err != nil {
		return nil, fmt.Errorf("failed to create node: %w", err)
	}

	// Add URL as metadata
	node.SetMetadata("url", webContent.URL)
	node.SetMetadata("source", "web")
	node.SetMetadata("extracted_at", webContent.ExtractedAt.Format(time.RFC3339))

	// Extract and add tags from metadata if present
	if tags, ok := webContent.Metadata["tags"].([]string); ok {
		for _, tag := range tags {
			node.AddTag(tag)
		}
	}

	return node, nil
}

// TranslateFromNode converts a domain Node to external web content format
func (w *WebContentAdapter) TranslateFromNode(node *entities.Node) (interface{}, error) {
	if node == nil {
		return nil, fmt.Errorf("node cannot be nil")
	}

	content := node.Content()
	metadata := node.GetMetadata()

	// Extract URL from metadata if present
	url := ""
	if urlValue, exists := metadata["url"]; exists {
		url, _ = urlValue.(string)
	}

	return &WebContent{
		URL:         url,
		Title:       content.Title(),
		Content:     content.Body(),
		Metadata:    metadata,
		ExtractedAt: time.Now(),
	}, nil
}

// ValidateExternalData ensures web content meets our domain requirements
func (w *WebContentAdapter) ValidateExternalData(data interface{}) error {
	webContent, ok := data.(*WebContent)
	if !ok {
		return fmt.Errorf("invalid data type")
	}

	if webContent.Title == "" {
		return fmt.Errorf("title is required")
	}

	if len(webContent.Title) > 500 {
		return fmt.Errorf("title too long (max 500 characters)")
	}

	if len(webContent.Content) > 50000 {
		return fmt.Errorf("content too long (max 50000 characters)")
	}

	return nil
}

// AIServiceAdapter adapts AI service responses to domain models
type AIServiceAdapter struct {
	maxTokens int
}

// NewAIServiceAdapter creates a new AI service adapter
func NewAIServiceAdapter(maxTokens int) *AIServiceAdapter {
	return &AIServiceAdapter{
		maxTokens: maxTokens,
	}
}

// AIResponse represents an external AI service response
type AIResponse struct {
	Prompt       string   `json:"prompt"`
	Response     string   `json:"response"`
	Model        string   `json:"model"`
	Tokens       int      `json:"tokens"`
	Temperature  float64  `json:"temperature"`
	GeneratedAt  time.Time `json:"generated_at"`
	Keywords     []string `json:"keywords,omitempty"`
	Sentiment    string   `json:"sentiment,omitempty"`
}

// TranslateToNode converts AI response to a domain Node
func (a *AIServiceAdapter) TranslateToNode(externalData interface{}) (*entities.Node, error) {
	aiResponse, ok := externalData.(*AIResponse)
	if !ok {
		return nil, fmt.Errorf("invalid data type: expected AIResponse")
	}

	// Validate the external data
	if err := a.ValidateExternalData(aiResponse); err != nil {
		return nil, fmt.Errorf("validation failed: %w", err)
	}

	// Create title from prompt (truncate if needed)
	title := aiResponse.Prompt
	if len(title) > 100 {
		title = title[:97] + "..."
	}

	// Create domain value objects
	content, err := valueobjects.NewNodeContent(
		title,
		aiResponse.Response,
		valueobjects.FormatPlainText,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create content: %w", err)
	}

	position, _ := valueobjects.NewPosition3D(0, 0, 0)

	// Create the domain entity
	node, err := entities.NewNode("ai-service", content, position)
	if err != nil {
		return nil, fmt.Errorf("failed to create node: %w", err)
	}

	// Add AI-specific metadata
	node.SetMetadata("source", "ai")
	node.SetMetadata("model", aiResponse.Model)
	node.SetMetadata("prompt", aiResponse.Prompt)
	node.SetMetadata("temperature", aiResponse.Temperature)
	node.SetMetadata("tokens", aiResponse.Tokens)
	node.SetMetadata("generated_at", aiResponse.GeneratedAt.Format(time.RFC3339))

	// Add keywords as tags
	for _, keyword := range aiResponse.Keywords {
		node.AddTag(keyword)
	}

	// Add sentiment if present
	if aiResponse.Sentiment != "" {
		node.SetMetadata("sentiment", aiResponse.Sentiment)
	}

	return node, nil
}

// TranslateFromNode converts a domain Node to AI response format
func (a *AIServiceAdapter) TranslateFromNode(node *entities.Node) (interface{}, error) {
	if node == nil {
		return nil, fmt.Errorf("node cannot be nil")
	}

	content := node.Content()
	metadata := node.GetMetadata()

	// Extract AI-specific metadata
	prompt, _ := metadata["prompt"].(string)
	model, _ := metadata["model"].(string)
	temperature, _ := metadata["temperature"].(float64)
	tokens, _ := metadata["tokens"].(int)
	sentiment, _ := metadata["sentiment"].(string)

	return &AIResponse{
		Prompt:      prompt,
		Response:    content.Body(),
		Model:       model,
		Tokens:      tokens,
		Temperature: temperature,
		GeneratedAt: time.Now(),
		Keywords:    node.GetTags(),
		Sentiment:   sentiment,
	}, nil
}

// ValidateExternalData ensures AI response meets our domain requirements
func (a *AIServiceAdapter) ValidateExternalData(data interface{}) error {
	aiResponse, ok := data.(*AIResponse)
	if !ok {
		return fmt.Errorf("invalid data type")
	}

	if aiResponse.Response == "" {
		return fmt.Errorf("response is required")
	}

	if aiResponse.Tokens > a.maxTokens {
		return fmt.Errorf("response exceeds maximum tokens (%d > %d)", aiResponse.Tokens, a.maxTokens)
	}

	if aiResponse.Temperature < 0 || aiResponse.Temperature > 2 {
		return fmt.Errorf("invalid temperature value")
	}

	return nil
}

// DatabaseImportAdapter adapts external database records to domain models
type DatabaseImportAdapter struct {
	fieldMappings map[string]string
}

// NewDatabaseImportAdapter creates a new database import adapter
func NewDatabaseImportAdapter(fieldMappings map[string]string) *DatabaseImportAdapter {
	return &DatabaseImportAdapter{
		fieldMappings: fieldMappings,
	}
}

// ExternalRecord represents a record from an external database
type ExternalRecord struct {
	ID         string                 `json:"id"`
	Fields     map[string]interface{} `json:"fields"`
	ImportedAt time.Time              `json:"imported_at"`
	Source     string                 `json:"source"`
}

// TranslateToNode converts external database record to a domain Node
func (d *DatabaseImportAdapter) TranslateToNode(externalData interface{}) (*entities.Node, error) {
	record, ok := externalData.(*ExternalRecord)
	if !ok {
		return nil, fmt.Errorf("invalid data type: expected ExternalRecord")
	}

	// Map fields using configured mappings
	title := d.mapField(record.Fields, "title")
	body := d.mapField(record.Fields, "content")

	if title == "" {
		// Try to generate title from other fields
		title = fmt.Sprintf("Imported Record %s", record.ID)
	}

	// Create domain value objects
	content, err := valueobjects.NewNodeContent(
		title,
		body,
		valueobjects.FormatPlainText,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create content: %w", err)
	}

	position, _ := valueobjects.NewPosition3D(0, 0, 0)

	// Create the domain entity
	node, err := entities.NewNode("import-service", content, position)
	if err != nil {
		return nil, fmt.Errorf("failed to create node: %w", err)
	}

	// Add import metadata
	node.SetMetadata("source", record.Source)
	node.SetMetadata("external_id", record.ID)
	node.SetMetadata("imported_at", record.ImportedAt.Format(time.RFC3339))

	// Map additional fields as metadata
	for key, value := range record.Fields {
		if key != "title" && key != "content" {
			node.SetMetadata(fmt.Sprintf("import_%s", key), value)
		}
	}

	return node, nil
}

// TranslateFromNode converts a domain Node back to external record format
func (d *DatabaseImportAdapter) TranslateFromNode(node *entities.Node) (interface{}, error) {
	if node == nil {
		return nil, fmt.Errorf("node cannot be nil")
	}

	content := node.Content()
	metadata := node.GetMetadata()

	// Extract external ID if present
	externalID, _ := metadata["external_id"].(string)
	if externalID == "" {
		externalID = node.ID().String()
	}

	// Build fields map
	fields := make(map[string]interface{})
	fields["title"] = content.Title()
	fields["content"] = content.Body()

	// Add all import_ prefixed metadata back as fields
	for key, value := range metadata {
		if len(key) > 7 && key[:7] == "import_" {
			fields[key[7:]] = value
		}
	}

	source, _ := metadata["source"].(string)

	return &ExternalRecord{
		ID:         externalID,
		Fields:     fields,
		ImportedAt: time.Now(),
		Source:     source,
	}, nil
}

// ValidateExternalData ensures external record meets our domain requirements
func (d *DatabaseImportAdapter) ValidateExternalData(data interface{}) error {
	record, ok := data.(*ExternalRecord)
	if !ok {
		return fmt.Errorf("invalid data type")
	}

	if record.ID == "" {
		return fmt.Errorf("record ID is required")
	}

	if record.Fields == nil || len(record.Fields) == 0 {
		return fmt.Errorf("record must have fields")
	}

	return nil
}

// mapField maps external field names to internal field names
func (d *DatabaseImportAdapter) mapField(fields map[string]interface{}, internalName string) string {
	// Check if there's a mapping for this internal name
	if externalName, exists := d.fieldMappings[internalName]; exists {
		if value, ok := fields[externalName]; ok {
			return fmt.Sprintf("%v", value)
		}
	}

	// Try direct field name
	if value, ok := fields[internalName]; ok {
		return fmt.Sprintf("%v", value)
	}

	return ""
}

// ExternalSystemFacade provides a unified interface for all external system interactions
type ExternalSystemFacade struct {
	adapters map[string]ExternalAPIAdapter
}

// NewExternalSystemFacade creates a new facade for external systems
func NewExternalSystemFacade() *ExternalSystemFacade {
	return &ExternalSystemFacade{
		adapters: make(map[string]ExternalAPIAdapter),
	}
}

// RegisterAdapter registers an adapter for a specific external system
func (f *ExternalSystemFacade) RegisterAdapter(systemName string, adapter ExternalAPIAdapter) {
	f.adapters[systemName] = adapter
}

// ImportFromExternalSystem imports data from an external system
func (f *ExternalSystemFacade) ImportFromExternalSystem(
	ctx context.Context,
	systemName string,
	externalData interface{},
) (*entities.Node, error) {
	adapter, exists := f.adapters[systemName]
	if !exists {
		return nil, fmt.Errorf("no adapter registered for system: %s", systemName)
	}

	// Validate the external data
	if err := adapter.ValidateExternalData(externalData); err != nil {
		return nil, fmt.Errorf("external data validation failed: %w", err)
	}

	// Translate to domain model
	node, err := adapter.TranslateToNode(externalData)
	if err != nil {
		return nil, fmt.Errorf("translation failed: %w", err)
	}

	// Add system metadata
	node.SetMetadata("imported_from", systemName)
	node.SetMetadata("import_timestamp", time.Now().Format(time.RFC3339))

	return node, nil
}

// ExportToExternalSystem exports a node to an external system format
func (f *ExternalSystemFacade) ExportToExternalSystem(
	ctx context.Context,
	systemName string,
	node *entities.Node,
) (interface{}, error) {
	adapter, exists := f.adapters[systemName]
	if !exists {
		return nil, fmt.Errorf("no adapter registered for system: %s", systemName)
	}

	// Translate from domain model
	externalData, err := adapter.TranslateFromNode(node)
	if err != nil {
		return nil, fmt.Errorf("translation failed: %w", err)
	}

	return externalData, nil
}