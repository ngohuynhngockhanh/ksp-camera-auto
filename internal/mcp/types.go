// Package mcp implements the Model Context Protocol (MCP) server for kspcam.
// It supports both Stdio and HTTP/SSE transports over JSON-RPC 2.0.
package mcp

import (
	"encoding/json"
	"fmt"
)

// Standard JSON-RPC 2.0 and MCP protocol versions and error codes.
const (
	JSONRPCVersion     = "2.0"
	ProtocolVersion    = "2024-11-05"
	ProtocolVersionOld = "2024-10-07"

	CodeParseError        = -32700
	CodeInvalidRequest    = -32600
	CodeMethodNotFound    = -32601
	CodeInvalidParams     = -32602
	CodeInternalError     = -32603
	CodeUnauthorized      = -32001
	CodeDeviceUnreachable = -32002
)

// JSONRPCRequest represents an incoming JSON-RPC 2.0 request.
type JSONRPCRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      any             `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

// JSONRPCResponse represents an outgoing JSON-RPC 2.0 response.
type JSONRPCResponse struct {
	JSONRPC string        `json:"jsonrpc"`
	ID      any           `json:"id"`
	Result  any           `json:"result,omitempty"`
	Error   *JSONRPCError `json:"error,omitempty"`
}

// JSONRPCNotification represents a notification message without an ID.
type JSONRPCNotification struct {
	JSONRPC string          `json:"jsonrpc"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

// JSONRPCError defines standard and application-specific error structures.
type JSONRPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

func (e *JSONRPCError) Error() string {
	return fmt.Sprintf("jsonrpc error [%d]: %s", e.Code, e.Message)
}

// NewJSONRPCError creates a formatted JSONRPCError.
func NewJSONRPCError(code int, message string, data any) *JSONRPCError {
	return &JSONRPCError{
		Code:    code,
		Message: message,
		Data:    data,
	}
}

// ClientInfo describes the connecting MCP client.
type ClientInfo struct {
	Name    string `json:"name"`
	Version string `json:"version,omitempty"`
}

// InitializeParams holds parameters for the "initialize" handshake.
type InitializeParams struct {
	ProtocolVersion string          `json:"protocolVersion"`
	Capabilities    json.RawMessage `json:"capabilities,omitempty"`
	ClientInfo      *ClientInfo     `json:"clientInfo,omitempty"`
}

// ServerInfo identifies the MCP server.
type ServerInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// ToolsCapability indicates server tool support.
type ToolsCapability struct {
	ListChanged bool `json:"listChanged"`
}

// ServerCapabilities defines features offered by the MCP server.
type ServerCapabilities struct {
	Tools *ToolsCapability `json:"tools,omitempty"`
}

// InitializeResult is returned in response to "initialize".
type InitializeResult struct {
	ProtocolVersion string             `json:"protocolVersion"`
	Capabilities    ServerCapabilities `json:"capabilities"`
	ServerInfo      ServerInfo         `json:"serverInfo"`
}

// ToolInputSchema describes the JSON Schema for tool parameters.
type ToolInputSchema struct {
	Type        string         `json:"type"`
	Properties  map[string]any `json:"properties,omitempty"`
	Required    []string       `json:"required,omitempty"`
	Description string         `json:"description,omitempty"`
}

// Tool defines an invokable MCP tool.
type Tool struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	InputSchema ToolInputSchema `json:"inputSchema"`
}

// ToolsListResult is returned by "tools/list".
type ToolsListResult struct {
	Tools []Tool `json:"tools"`
}

// ToolCallParams describes arguments for "tools/call".
type ToolCallParams struct {
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments,omitempty"`
}

// ContentItem represents a single content element (text or image) in a ToolResult.
type ContentItem struct {
	Type     string `json:"type"` // "text" or "image"
	Text     string `json:"text,omitempty"`
	Data     string `json:"data,omitempty"`     // base64 encoded for image
	MIMEType string `json:"mimeType,omitempty"` // e.g. "image/jpeg"
}

// ToolResult represents the output of a tool invocation.
type ToolResult struct {
	Content []ContentItem `json:"content"`
	IsError bool          `json:"isError,omitempty"`
}

// NewTextResult builds a successful ToolResult containing plain text.
func NewTextResult(text string) ToolResult {
	return ToolResult{
		Content: []ContentItem{
			{
				Type: "text",
				Text: text,
			},
		},
		IsError: false,
	}
}

// NewJSONResult serializes any value to pretty-printed JSON text inside a ToolResult.
func NewJSONResult(v any) (ToolResult, error) {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return ToolResult{}, fmt.Errorf("marshal tool result: %w", err)
	}
	return NewTextResult(string(b)), nil
}

// NewErrorResult builds an error ToolResult containing error message text.
func NewErrorResult(errMsg string) ToolResult {
	return ToolResult{
		Content: []ContentItem{
			{
				Type: "text",
				Text: errMsg,
			},
		},
		IsError: true,
	}
}

// NewImageResult builds an image ToolResult with base64 data and MIME type.
func NewImageResult(mimeType, base64Data string) ToolResult {
	return ToolResult{
		Content: []ContentItem{
			{
				Type:     "image",
				Data:     base64Data,
				MIMEType: mimeType,
			},
		},
		IsError: false,
	}
}
