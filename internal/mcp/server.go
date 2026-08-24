package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/ngohuynhngockhanh/ksp-camera-auto/internal/config"
	"github.com/ngohuynhngockhanh/ksp-camera-auto/internal/redbida"
	"github.com/ngohuynhngockhanh/ksp-camera-auto/internal/shinobi"
)

// Server implements the core Model Context Protocol (MCP) server.
type Server struct {
	cfg      *config.Config
	inv      *config.Inventory
	shinobi  *shinobi.Client
	redbida  *redbida.Service
	registry *Registry
	mu       sync.RWMutex
	sessions map[string]*Session
}

// NewServer initializes a new MCP server and registers all standard tools.
func NewServer(cfg *config.Config, inv *config.Inventory, shinobiClient *shinobi.Client, redbidaService ...*redbida.Service) *Server {
	if cfg == nil {
		defaultCfg := config.Default()
		cfg = &defaultCfg
	}

	var rSvc *redbida.Service
	if len(redbidaService) > 0 && redbidaService[0] != nil {
		rSvc = redbidaService[0]
	} else if cfg.Redbida.Enabled {
		broker := redbida.NewMQTTBroker(redbida.MQTTOptions{
			Host: cfg.Redbida.BrokerHost, Port: cfg.Redbida.BrokerPort,
			ReadTopic: cfg.Redbida.ReadTopic, ReadAckTopic: cfg.Redbida.ReadAckTopic,
			WriteTopic: cfg.Redbida.WriteTopic, WriteAckTopic: cfg.Redbida.WriteAckTopic,
			Timeout: time.Duration(cfg.Redbida.TimeoutSeconds) * time.Second,
		})
		rSvc = redbida.NewService(broker, redbida.NewCatalog(cfg.Redbida.KeyDir), cfg.Redbida.MaxBatchKeys)
	}

	registry := NewRegistry()
	registerCameraInventoryTools(registry, cfg, inv)
	registerCameraConfigTools(registry, cfg, inv)
	registerDiscoveryDiagnosisTools(registry, cfg, inv)
	registerShinobiTools(registry, cfg, inv, shinobiClient)
	registerRedbidaTools(registry, cfg, rSvc)

	return &Server{
		cfg:      cfg,
		inv:      inv,
		shinobi:  shinobiClient,
		redbida:  rSvc,
		registry: registry,
		sessions: make(map[string]*Session),
	}
}

// Registry returns the underlying Tool Registry.
func (s *Server) Registry() *Registry {
	return s.registry
}

// ProcessRequest executes an incoming JSON-RPC 2.0 request and generates the matching response.
// If the message is a notification, isNotification returns true and the response should not be sent.
func (s *Server) ProcessRequest(ctx context.Context, req JSONRPCRequest) (resp JSONRPCResponse, isNotification bool) {
	resp = JSONRPCResponse{
		JSONRPC: JSONRPCVersion,
		ID:      req.ID,
	}

	if req.JSONRPC != "" && req.JSONRPC != JSONRPCVersion {
		resp.Error = NewJSONRPCError(CodeInvalidRequest, fmt.Sprintf("invalid jsonrpc version %q (expected %q)", req.JSONRPC, JSONRPCVersion), nil)
		return resp, false
	}

	// Notifications have no ID (or ID is nil) and start with "notifications/" or are notifications/initialized
	if req.ID == nil && req.Method == "notifications/initialized" {
		return resp, true
	}

	switch req.Method {
	case "initialize":
		var params InitializeParams
		if len(req.Params) > 0 {
			_ = json.Unmarshal(req.Params, &params)
		}

		res := InitializeResult{
			ProtocolVersion: ProtocolVersion,
			Capabilities: ServerCapabilities{
				Tools: &ToolsCapability{
					ListChanged: false,
				},
			},
			ServerInfo: ServerInfo{
				Name:    "kspcam",
				Version: "1.0.0",
			},
		}
		resp.Result = res
		return resp, false

	case "notifications/initialized":
		// Handshake completion notification
		return resp, true

	case "ping":
		resp.Result = map[string]any{}
		return resp, false

	case "tools/list":
		tools := s.registry.List()
		resp.Result = ToolsListResult{
			Tools: tools,
		}
		return resp, false

	case "tools/call":
		var params ToolCallParams
		if err := json.Unmarshal(req.Params, &params); err != nil {
			resp.Error = NewJSONRPCError(CodeInvalidParams, "failed to parse tools/call params: "+err.Error(), nil)
			return resp, false
		}

		toolResult, err := s.registry.Call(ctx, params.Name, params.Arguments)
		if err != nil && len(toolResult.Content) == 0 {
			resp.Error = NewJSONRPCError(CodeInternalError, err.Error(), nil)
			return resp, false
		}

		resp.Result = toolResult
		return resp, false

	default:
		resp.Error = NewJSONRPCError(CodeMethodNotFound, fmt.Sprintf("method %q not found", req.Method), nil)
		return resp, false
	}
}

// ProcessMessage processes raw byte message and returns serialized JSON response.
func (s *Server) ProcessMessage(ctx context.Context, msg []byte) ([]byte, bool, error) {
	var req JSONRPCRequest
	if err := json.Unmarshal(msg, &req); err != nil {
		errResp := JSONRPCResponse{
			JSONRPC: JSONRPCVersion,
			Error:   NewJSONRPCError(CodeParseError, "parse error: "+err.Error(), nil),
		}
		b, _ := json.Marshal(errResp)
		return b, false, nil
	}

	resp, isNotification := s.ProcessRequest(ctx, req)
	if isNotification {
		return nil, true, nil
	}

	b, err := json.Marshal(resp)
	if err != nil {
		log.Printf("mcp marshal response error: %v", err)
		return nil, false, err
	}

	return b, false, nil
}

// HTTPHandler returns the HTTP handler for /mcp and /mcp/messages.
func (s *Server) HTTPHandler() http.Handler {
	return NewHTTPHandler(s, s.cfg.MCP)
}
