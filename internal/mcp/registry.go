package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"sync"
)

// ToolHandler defines the execution function for an MCP tool.
type ToolHandler func(ctx context.Context, args json.RawMessage) (ToolResult, error)

type registeredTool struct {
	tool    Tool
	handler ToolHandler
}

// Registry manages the set of available MCP tools.
type Registry struct {
	mu    sync.RWMutex
	tools map[string]registeredTool
}

// NewRegistry initializes an empty tool registry.
func NewRegistry() *Registry {
	return &Registry{
		tools: make(map[string]registeredTool),
	}
}

// Register registers a tool definition with its execution handler.
func (r *Registry) Register(tool Tool, handler ToolHandler) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.tools[tool.Name] = registeredTool{
		tool:    tool,
		handler: handler,
	}
}

// Get retrieves a registered tool and handler by name.
func (r *Registry) Get(name string) (Tool, ToolHandler, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	rt, ok := r.tools[name]
	if !ok {
		return Tool{}, nil, false
	}
	return rt.tool, rt.handler, true
}

// List returns all registered tools sorted deterministically by name.
func (r *Registry) List() []Tool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	names := make([]string, 0, len(r.tools))
	for name := range r.tools {
		names = append(names, name)
	}
	sort.Strings(names)

	out := make([]Tool, 0, len(names))
	for _, name := range names {
		out = append(out, r.tools[name].tool)
	}
	return out
}

// Call executes the handler for the named tool.
func (r *Registry) Call(ctx context.Context, name string, args json.RawMessage) (ToolResult, error) {
	r.mu.RLock()
	rt, ok := r.tools[name]
	r.mu.RUnlock()

	if !ok {
		return NewErrorResult(fmt.Sprintf("unknown tool %q", name)), fmt.Errorf("unknown tool %q", name)
	}

	return rt.handler(ctx, args)
}
