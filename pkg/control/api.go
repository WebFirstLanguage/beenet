// Package control implements the Beenet local control API as specified in Phase 1.
package control

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"sync"

	"github.com/WebFirstLanguage/beenet/pkg/agent"
)

// Request represents a control API request
type Request struct {
	Method string                 `json:"method"`
	ID     string                 `json:"id"`
	Params map[string]interface{} `json:"params,omitempty"`
}

// Response represents a control API response
type Response struct {
	ID     string      `json:"id"`
	Result interface{} `json:"result,omitempty"`
	Error  string      `json:"error,omitempty"`
}

// Server implements the control API server
type Server struct {
	mu    sync.RWMutex
	agent *agent.Agent
}

// NewServer creates a new control API server
func NewServer(agent *agent.Agent) *Server {
	return &Server{
		agent: agent,
	}
}

// Serve starts the control API server on the given listener
func (s *Server) Serve(ctx context.Context, listener net.Listener) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			conn, err := listener.Accept()
			if err != nil {
				select {
				case <-ctx.Done():
					return ctx.Err()
				default:
					continue // Continue accepting connections
				}
			}

			// Handle connection in goroutine
			go s.handleConnection(ctx, conn)
		}
	}
}

// handleConnection handles a single client connection
func (s *Server) handleConnection(ctx context.Context, conn net.Conn) {
	defer conn.Close()

	decoder := json.NewDecoder(conn)
	encoder := json.NewEncoder(conn)

	for {
		select {
		case <-ctx.Done():
			return
		default:
			var request Request
			if err := decoder.Decode(&request); err != nil {
				// Connection closed or invalid JSON
				return
			}

			response := s.handleRequest(request)

			if err := encoder.Encode(response); err != nil {
				// Failed to send response
				return
			}
		}
	}
}

// handleRequest processes a single API request
func (s *Server) handleRequest(request Request) Response {
	switch request.Method {
	case "GetInfo":
		return s.handleGetInfo(request)
	case "SetNickname":
		return s.handleSetNickname(request)
	default:
		return Response{
			ID:    request.ID,
			Error: fmt.Sprintf("unknown method: %s", request.Method),
		}
	}
}

// handleGetInfo handles the GetInfo operation
func (s *Server) handleGetInfo(request Request) Response {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := map[string]interface{}{
		"bid":      s.agent.BID(),
		"nickname": s.agent.Nickname(),
		"state":    s.agent.State().String(),
	}

	// Add handle if nickname is set
	if nickname := s.agent.Nickname(); nickname != "" {
		result["handle"] = s.agent.Handle(nickname)
	}

	return Response{
		ID:     request.ID,
		Result: result,
	}
}

// handleSetNickname handles the SetNickname operation
func (s *Server) handleSetNickname(request Request) Response {
	// Extract nickname from params
	nickname, ok := request.Params["nickname"].(string)
	if !ok {
		return Response{
			ID:    request.ID,
			Error: "nickname parameter is required and must be a string",
		}
	}

	// Set nickname on agent
	if err := s.agent.SetNickname(nickname); err != nil {
		return Response{
			ID:    request.ID,
			Error: fmt.Sprintf("failed to set nickname: %v", err),
		}
	}

	return Response{
		ID: request.ID,
		Result: map[string]interface{}{
			"nickname": s.agent.Nickname(),
			"handle":   s.agent.Handle(s.agent.Nickname()),
		},
	}
}
