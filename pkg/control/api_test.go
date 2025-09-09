package control

import (
	"context"
	"encoding/json"
	"net"
	"testing"
	"time"

	"github.com/WebFirstLanguage/beenet/pkg/agent"
	"github.com/WebFirstLanguage/beenet/pkg/identity"
)

// TestControlAPIServer tests the control API server lifecycle
func TestControlAPIServer(t *testing.T) {
	// Create test identity and agent
	testIdentity, err := identity.GenerateIdentity()
	if err != nil {
		t.Fatalf("Failed to generate test identity: %v", err)
	}

	testAgent := agent.New(testIdentity)
	
	// Create control API server
	server := NewServer(testAgent)
	
	// Start server on a random port
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to create listener: %v", err)
	}
	defer listener.Close()
	
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	
	// Start server
	go func() {
		if err := server.Serve(ctx, listener); err != nil && err != context.Canceled {
			t.Errorf("Server error: %v", err)
		}
	}()
	
	// Give server time to start
	time.Sleep(10 * time.Millisecond)
	
	// Test connection
	conn, err := net.Dial("tcp", listener.Addr().String())
	if err != nil {
		t.Fatalf("Failed to connect to server: %v", err)
	}
	defer conn.Close()
	
	// Server should accept the connection
	// More detailed tests will be in specific operation tests
}

// TestGetInfoOperation tests the GetInfo control operation
func TestGetInfoOperation(t *testing.T) {
	// Create test identity and agent
	testIdentity, err := identity.GenerateIdentity()
	if err != nil {
		t.Fatalf("Failed to generate test identity: %v", err)
	}

	testAgent := agent.New(testIdentity)
	testAgent.SetNickname("alice")
	
	// Create control API server
	server := NewServer(testAgent)
	
	// Start server
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to create listener: %v", err)
	}
	defer listener.Close()
	
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	
	go func() {
		server.Serve(ctx, listener)
	}()
	
	time.Sleep(10 * time.Millisecond)
	
	// Connect and send GetInfo request
	conn, err := net.Dial("tcp", listener.Addr().String())
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close()
	
	// Send GetInfo request
	request := Request{
		Method: "GetInfo",
		ID:     "test-1",
	}
	
	encoder := json.NewEncoder(conn)
	if err := encoder.Encode(request); err != nil {
		t.Fatalf("Failed to send request: %v", err)
	}
	
	// Read response
	decoder := json.NewDecoder(conn)
	var response Response
	if err := decoder.Decode(&response); err != nil {
		t.Fatalf("Failed to read response: %v", err)
	}
	
	// Check response
	if response.ID != "test-1" {
		t.Errorf("Expected response ID 'test-1', got %s", response.ID)
	}
	
	if response.Error != "" {
		t.Errorf("Unexpected error in response: %s", response.Error)
	}
	
	// Check result contains expected fields
	result, ok := response.Result.(map[string]interface{})
	if !ok {
		t.Fatalf("Expected result to be a map, got %T", response.Result)
	}
	
	if result["bid"] == "" {
		t.Error("Expected BID in result")
	}
	
	if result["nickname"] != "alice" {
		t.Errorf("Expected nickname 'alice', got %v", result["nickname"])
	}
	
	if result["handle"] == "" {
		t.Error("Expected handle in result")
	}
	
	if result["state"] == "" {
		t.Error("Expected state in result")
	}
}

// TestSetNicknameOperation tests the SetNickname control operation
func TestSetNicknameOperation(t *testing.T) {
	// Create test identity and agent
	testIdentity, err := identity.GenerateIdentity()
	if err != nil {
		t.Fatalf("Failed to generate test identity: %v", err)
	}

	testAgent := agent.New(testIdentity)
	
	// Create control API server
	server := NewServer(testAgent)
	
	// Start server
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to create listener: %v", err)
	}
	defer listener.Close()
	
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	
	go func() {
		server.Serve(ctx, listener)
	}()
	
	time.Sleep(10 * time.Millisecond)
	
	// Connect and send SetNickname request
	conn, err := net.Dial("tcp", listener.Addr().String())
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close()
	
	// Send SetNickname request
	request := Request{
		Method: "SetNickname",
		ID:     "test-2",
		Params: map[string]interface{}{
			"nickname": "bob",
		},
	}
	
	encoder := json.NewEncoder(conn)
	if err := encoder.Encode(request); err != nil {
		t.Fatalf("Failed to send request: %v", err)
	}
	
	// Read response
	decoder := json.NewDecoder(conn)
	var response Response
	if err := decoder.Decode(&response); err != nil {
		t.Fatalf("Failed to read response: %v", err)
	}
	
	// Check response
	if response.ID != "test-2" {
		t.Errorf("Expected response ID 'test-2', got %s", response.ID)
	}
	
	if response.Error != "" {
		t.Errorf("Unexpected error in response: %s", response.Error)
	}
	
	// Check that nickname was actually set
	if testAgent.Nickname() != "bob" {
		t.Errorf("Expected agent nickname 'bob', got %s", testAgent.Nickname())
	}
}

// TestInvalidNicknameOperation tests SetNickname with invalid nickname
func TestInvalidNicknameOperation(t *testing.T) {
	// Create test identity and agent
	testIdentity, err := identity.GenerateIdentity()
	if err != nil {
		t.Fatalf("Failed to generate test identity: %v", err)
	}

	testAgent := agent.New(testIdentity)
	
	// Create control API server
	server := NewServer(testAgent)
	
	// Start server
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to create listener: %v", err)
	}
	defer listener.Close()
	
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	
	go func() {
		server.Serve(ctx, listener)
	}()
	
	time.Sleep(10 * time.Millisecond)
	
	// Connect and send SetNickname request with invalid nickname
	conn, err := net.Dial("tcp", listener.Addr().String())
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close()
	
	// Send SetNickname request with invalid nickname (too short)
	request := Request{
		Method: "SetNickname",
		ID:     "test-3",
		Params: map[string]interface{}{
			"nickname": "ab", // Too short
		},
	}
	
	encoder := json.NewEncoder(conn)
	if err := encoder.Encode(request); err != nil {
		t.Fatalf("Failed to send request: %v", err)
	}
	
	// Read response
	decoder := json.NewDecoder(conn)
	var response Response
	if err := decoder.Decode(&response); err != nil {
		t.Fatalf("Failed to read response: %v", err)
	}
	
	// Check response contains error
	if response.ID != "test-3" {
		t.Errorf("Expected response ID 'test-3', got %s", response.ID)
	}
	
	if response.Error == "" {
		t.Error("Expected error in response for invalid nickname")
	}
}
