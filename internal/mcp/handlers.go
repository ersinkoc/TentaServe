package mcp

import (
	"encoding/json"
	"fmt"
	"time"
)

// ListToolsRequest is the request for tools/list method.
type ListToolsRequest struct {
	Cursor string `json:"cursor,omitempty"`
}

// ListToolsResult is the response for tools/list method.
type ListToolsResult struct {
	Tools      []*Tool `json:"tools"`
	NextCursor string  `json:"nextCursor,omitempty"`
}

// RegisterToolsHandlers registers the tools/list and tools/call handlers.
func (s *Server) RegisterToolsHandlers(registry *ToolRegistry) {
	// tools/list handler
	s.Register("tools/list", func(req *Request) (any, *Error) {
		var listReq ListToolsRequest
		if len(req.Params) > 0 {
			if err := json.Unmarshal(req.Params, &listReq); err != nil {
				return nil, NewErrorWithData(ErrInvalidParams, "Invalid params", err.Error())
			}
		}

		// Get all tools from registry
		tools := registry.List()

		// Update registered tools gauge
		if s.metrics != nil {
			s.metrics.UpdateToolCount(len(tools))
		}

		// Pagination is not implemented yet - return all tools
		// In the future, we could use listReq.Cursor for pagination
		result := &ListToolsResult{
			Tools:      tools,
			NextCursor: "", // No more pages
		}

		return result, nil
	})

	// tools/call handler
	s.Register("tools/call", func(req *Request) (any, *Error) {
		var callReq CallToolRequest
		if len(req.Params) > 0 {
			if err := json.Unmarshal(req.Params, &callReq); err != nil {
				return nil, NewErrorWithData(ErrInvalidParams, "Invalid params", err.Error())
			}
		}

		// Validate request
		if err := callReq.Validate(); err != nil {
			return nil, NewErrorWithData(ErrInvalidParams, err.Error(), nil)
		}

		// Look up the tool
		tool := registry.Get(callReq.Name)
		if tool == nil {
			return &CallToolResult{
				Content: []ToolContent{NewErrorContent(fmt.Sprintf("Tool '%s' not found", callReq.Name))},
				IsError: true,
			}, nil
		}

		// Execute the tool and record metrics
		start := time.Now()
		result, err := executeTool(tool, callReq.Arguments)
		duration := time.Since(start)

		// Record metrics
		if s.metrics != nil {
			s.metrics.RecordToolCall(tool.Name, tool.Upstream, duration)
		}

		if err != nil {
			return &CallToolResult{
				Content: []ToolContent{NewErrorContent(err.Error())},
				IsError: true,
			}, nil
		}

		return result, nil
	})
}

// CallToolRequest is the request for tools/call method.
type CallToolRequest struct {
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments,omitempty"`
}

// CallToolResult is the response for tools/call method.
type CallToolResult struct {
	Content []ToolContent `json:"content"`
	IsError bool          `json:"isError,omitempty"`
}

// ToolContent represents content in a tool result.
type ToolContent struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
	// URI is used for resource content
	URI string `json:"uri,omitempty"`
	// MIME type for binary content
	MIMETYPE string `json:"mimeType,omitempty"`
	// Data is used for binary content (base64 encoded)
	Data string `json:"data,omitempty"`
}

// Validate validates the CallToolRequest.
func (r *CallToolRequest) Validate() error {
	if r.Name == "" {
		return fmt.Errorf("tool name is required")
	}
	return nil
}

// NewTextContent creates a new text content item.
func NewTextContent(text string) ToolContent {
	return ToolContent{
		Type: "text",
		Text: text,
	}
}

// NewErrorContent creates a new error content item.
func NewErrorContent(err string) ToolContent {
	return ToolContent{
		Type: "text",
		Text: err,
	}
}

// executeTool executes a tool with the given arguments.
// This is a stub implementation - full execution requires proxy integration.
func executeTool(tool *Tool, args json.RawMessage) (*CallToolResult, error) {
	// Parse arguments
	var arguments map[string]interface{}
	if len(args) > 0 {
		if err := json.Unmarshal(args, &arguments); err != nil {
			return nil, fmt.Errorf("invalid arguments: %w", err)
		}
	}

	// For now, return a placeholder response indicating what would be executed
	// Full implementation would:
	// 1. Map tool arguments to upstream parameters using tool.ArgMapping
	// 2. Build the appropriate request (REST or GraphQL) using tool.Operation, tool.Method, tool.Path
	// 3. Execute via the proxy layer
	// 4. Return the actual response

	execInfo := map[string]any{
		"tool":      tool.Name,
		"upstream":  tool.Upstream,
		"operation": tool.Operation,
		"arguments": arguments,
	}

	// If it's a REST operation, include method and path
	if tool.Method != "" {
		execInfo["method"] = tool.Method
	}
	if tool.Path != "" {
		execInfo["path"] = tool.Path
	}

	// Format as pretty JSON for display
	execJSON, err := json.MarshalIndent(execInfo, "", "  ")
	if err != nil {
		return nil, err
	}

	return &CallToolResult{
		Content: []ToolContent{
			NewTextContent(fmt.Sprintf("Tool execution stub - would execute:\n%s", string(execJSON))),
		},
	}, nil
}

// Resource represents an MCP resource.
type Resource struct {
	URI         string `json:"uri"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	MIMEType    string `json:"mimeType,omitempty"`
}

// ResourceContent represents the content of a resource.
type ResourceContent struct {
	URI      string `json:"uri"`
	MIMEType string `json:"mimeType,omitempty"`
	Text     string `json:"text,omitempty"`
	Blob     string `json:"blob,omitempty"` // base64 encoded binary data
}

// ListResourcesRequest is the request for resources/list method.
type ListResourcesRequest struct {
	Cursor string `json:"cursor,omitempty"`
}

// ListResourcesResult is the response for resources/list method.
type ListResourcesResult struct {
	Resources  []*Resource `json:"resources"`
	NextCursor string      `json:"nextCursor,omitempty"`
}

// ReadResourceRequest is the request for resources/read method.
type ReadResourceRequest struct {
	URI string `json:"uri"`
}

// ReadResourceResult is the response for resources/read method.
type ReadResourceResult struct {
	Contents []ResourceContent `json:"contents"`
}

// RegisterResourcesHandlers registers the resources/list and resources/read handlers.
func (s *Server) RegisterResourcesHandlers(resources []*Resource) {
	// resources/list handler
	s.Register("resources/list", func(req *Request) (any, *Error) {
		var listReq ListResourcesRequest
		if len(req.Params) > 0 {
			if err := json.Unmarshal(req.Params, &listReq); err != nil {
				return nil, NewErrorWithData(ErrInvalidParams, "Invalid params", err.Error())
			}
		}

		// Update registered resources gauge
		if s.metrics != nil {
			s.metrics.UpdateResourceCount(len(resources))
		}

		// Return all resources (pagination not implemented)
		result := &ListResourcesResult{
			Resources:  resources,
			NextCursor: "",
		}

		return result, nil
	})

	// resources/read handler
	s.Register("resources/read", func(req *Request) (any, *Error) {
		var readReq ReadResourceRequest
		if len(req.Params) > 0 {
			if err := json.Unmarshal(req.Params, &readReq); err != nil {
				return nil, NewErrorWithData(ErrInvalidParams, "Invalid params", err.Error())
			}
		}

		// Validate URI
		if readReq.URI == "" {
			return nil, NewErrorWithData(ErrInvalidParams, "URI is required", nil)
		}

		// Find resource by URI
		var resource *Resource
		for _, r := range resources {
			if r.URI == readReq.URI {
				resource = r
				break
			}
		}

		if resource == nil {
			return nil, NewErrorWithData(ErrInvalidParams, fmt.Sprintf("Resource '%s' not found", readReq.URI), nil)
		}

		// Record resource read metric
		if s.metrics != nil {
			s.metrics.RecordResourceRead(resource.URI)
		}

		// For now, return a stub response
		// Full implementation would fetch actual resource content
		content := ResourceContent{
			URI:      resource.URI,
			MIMEType: resource.MIMEType,
			Text:     fmt.Sprintf("Stub content for resource: %s", resource.Name),
		}

		return &ReadResourceResult{
			Contents: []ResourceContent{content},
		}, nil
	})
}