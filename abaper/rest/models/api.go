package models

import "github.com/bluefunda/abaper/types"

// REST API request and response structures

// APIRequest represents a generic API request
type APIRequest struct {
	Action      string            `json:"action"`
	ObjectType  string            `json:"object_type,omitempty"`
	ObjectName  string            `json:"object_name,omitempty"`
	Description string            `json:"description,omitempty"`
	Source      string            `json:"source,omitempty"`
	Package     string            `json:"package,omitempty"`
	PackageName string            `json:"package_name,omitempty"` // used by packages/contents
	ETag        string            `json:"etag,omitempty"`
	Line        int               `json:"line,omitempty"`
	Column      int               `json:"column,omitempty"`
	Args        []string          `json:"args,omitempty"`
	Prompt      string            `json:"prompt,omitempty"`
	Config      map[string]string `json:"config,omitempty"`

	// Structured properties for DDIC objects that are not source-text based.
	DomainProperties      *types.DomainProperties      `json:"domain_properties,omitempty"`
	DataElementProperties *types.DataElementProperties `json:"data_element_properties,omitempty"`

	// Structured properties for a Service Binding (SRVB/SVB, not source-text).
	ServiceBindingProperties *types.ServiceBindingProperties `json:"service_binding_properties,omitempty"`
}

// APIResponse is the standard response envelope. T is the data payload type.
// Use APIResponse[any] when the payload type is dynamic or unknown.
type APIResponse[T any] struct {
	Success bool   `json:"success"`
	Data    T      `json:"data,omitempty"`
	Error   string `json:"error,omitempty"`
	Message string `json:"message,omitempty"`
}

// ObjectRequest for object retrieval requests
type ObjectRequest struct {
	ObjectType string   `json:"object_type"`
	ObjectName string   `json:"object_name"`
	Args       []string `json:"args,omitempty"` // For function groups, etc.
}

// SearchRequest for object search requests
type SearchRequest struct {
	Pattern     string   `json:"pattern"`
	ObjectTypes []string `json:"object_types,omitempty"`
}

// ListRequest for object listing requests
type ListRequest struct {
	ObjectType string `json:"object_type"` // "packages", etc.
	Pattern    string `json:"pattern,omitempty"`
}

// ConnectionResponse for connection test results
type ConnectionResponse struct {
	Status        string `json:"status"`
	Authenticated bool   `json:"authenticated"`
	Timestamp     string `json:"timestamp"`
	Message       string `json:"message"`
}

// GenerateRequest for AI generation endpoints (removed but kept for compatibility)
type GenerateRequest struct {
	Prompt string `json:"prompt"`
	Stream bool   `json:"stream,omitempty"`
}

// ChatRequest for conversational AI endpoints (removed but kept for compatibility)
type ChatRequest struct {
	Message string `json:"message"`
	Context string `json:"context,omitempty"`
	Stream  bool   `json:"stream,omitempty"`
}
