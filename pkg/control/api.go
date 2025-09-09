// Package control implements the Beenet local control API as specified in Phase 1.
package control

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"sync"

	"github.com/WebFirstLanguage/beenet/internal/dht"
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
	case "peers":
		return s.handleGetPeers(request)
	case "seeds.list":
		return s.handleSeedsList(request)
	case "seeds.add":
		return s.handleSeedsAdd(request)
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

// handleGetPeers handles the peers operation
func (s *Server) handleGetPeers(request Request) Response {
	s.mu.RLock()
	defer s.mu.RUnlock()

	dht := s.agent.GetDHT()
	if dht == nil {
		return Response{
			ID:    request.ID,
			Error: "DHT not initialized",
		}
	}

	nodes := dht.GetAllNodes()
	peers := make([]map[string]interface{}, len(nodes))

	for i, node := range nodes {
		peers[i] = map[string]interface{}{
			"bid":       node.BID,
			"addrs":     node.Addrs,
			"last_seen": node.LastSeen.Format("2006-01-02T15:04:05Z07:00"),
		}
	}

	return Response{
		ID: request.ID,
		Result: map[string]interface{}{
			"peers": peers,
		},
	}
}

// handleSeedsList handles the seeds.list operation
func (s *Server) handleSeedsList(request Request) Response {
	s.mu.RLock()
	defer s.mu.RUnlock()

	bootstrap := s.agent.GetBootstrap()
	if bootstrap == nil {
		return Response{
			ID:    request.ID,
			Error: "Bootstrap not initialized",
		}
	}

	seedNodes := bootstrap.GetSeedNodes()
	seeds := make([]map[string]interface{}, len(seedNodes))

	for i, seed := range seedNodes {
		seeds[i] = map[string]interface{}{
			"bid":   seed.BID,
			"addrs": seed.Addrs,
			"name":  seed.Name,
		}
	}

	return Response{
		ID: request.ID,
		Result: map[string]interface{}{
			"seeds": seeds,
		},
	}
}

// handleSeedsAdd handles the seeds.add operation
func (s *Server) handleSeedsAdd(request Request) Response {
	s.mu.RLock()
	defer s.mu.RUnlock()

	bootstrap := s.agent.GetBootstrap()
	if bootstrap == nil {
		return Response{
			ID:    request.ID,
			Error: "Bootstrap not initialized",
		}
	}

	// Extract parameters
	params := request.Params
	if params == nil {
		return Response{
			ID:    request.ID,
			Error: "parameters required",
		}
	}

	bid, ok := params["bid"].(string)
	if !ok || bid == "" {
		return Response{
			ID:    request.ID,
			Error: "bid parameter is required",
		}
	}

	addrsInterface, ok := params["addrs"]
	if !ok {
		return Response{
			ID:    request.ID,
			Error: "addrs parameter is required",
		}
	}

	// Convert addrs to string slice
	var addrs []string
	if addrsList, ok := addrsInterface.([]interface{}); ok {
		addrs = make([]string, len(addrsList))
		for i, addr := range addrsList {
			if addrStr, ok := addr.(string); ok {
				addrs[i] = addrStr
			} else {
				return Response{
					ID:    request.ID,
					Error: "all addresses must be strings",
				}
			}
		}
	} else {
		return Response{
			ID:    request.ID,
			Error: "addrs must be an array of strings",
		}
	}

	name, _ := params["name"].(string) // Optional parameter

	// Create seed node
	seed := &dht.SeedNode{
		BID:   bid,
		Addrs: addrs,
		Name:  name,
	}

	// Add seed node
	if err := bootstrap.AddSeedNode(seed); err != nil {
		return Response{
			ID:    request.ID,
			Error: fmt.Sprintf("failed to add seed node: %v", err),
		}
	}

	return Response{
		ID: request.ID,
		Result: map[string]interface{}{
			"success": true,
			"message": "Seed node added successfully",
		},
	}
}
