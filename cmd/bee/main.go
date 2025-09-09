// Package main implements the Bee CLI as specified in §2.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"

	"github.com/WebFirstLanguage/beenet/pkg/agent"
	"github.com/WebFirstLanguage/beenet/pkg/control"
	"github.com/WebFirstLanguage/beenet/pkg/identity"
)

// Build-time variables set by ldflags
var (
	version    = "dev"
	buildTime  = "unknown"
	commitHash = "unknown"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	command := os.Args[1]
	switch command {
	case "version", "--version", "-v":
		printVersion()
	case "help", "--help", "-h":
		printUsage()
	case "start":
		if err := startCommand(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	case "create":
		fmt.Println("Creating new swarm... (not implemented yet)")
	case "status":
		if err := statusCommand(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	case "keygen":
		if err := keygenCommand(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	case "handle":
		if err := handleCommand(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	case "peers":
		if err := peersCommand(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	case "seeds":
		if err := seedsCommand(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	default:
		fmt.Printf("Unknown command: %s\n\n", command)
		printUsage()
		os.Exit(1)
	}
}

func printVersion() {
	fmt.Printf("Bee %s\n", version)
	fmt.Printf("Built: %s\n", buildTime)
	fmt.Printf("Commit: %s\n", commitHash)
}

func printUsage() {
	fmt.Printf(`Bee v%s - Beenet P2P mesh agent

Usage:
  bee <command> [options]

Commands:
  start     Start the bee agent daemon
  create    Create a new swarm
  status    Show agent status
  keygen    Generate new identity keys
  handle    Show current handle
  peers     Display discovered peer nodes
  seeds     Manage seed nodes (add/list)
  version   Show version information
  help      Show this help message

Examples:
  # Start agent (join mode - default)
  bee start --swarm <swarm-id> --seed <multiaddr> [--psk <hex> | --token <jwt>]

  # Create mode (explicit)
  bee create --name teamnet --seed-self --listen /ip4/0.0.0.0/udp/27487/quic

  # Import invite
  bee join beenet:swarm/<b32id>?seed=/ip4/203.0.113.5/udp/27487/quic&psk=<b32>

  # Generate new identity
  bee keygen

  # Show current handle
  bee handle

For more information, visit: https://github.com/WebFirstLanguage/beenet

`, version)
}

// getIdentityPath returns the path to the identity file
func getIdentityPath() string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "bee-identity.json"
	}
	return filepath.Join(homeDir, ".bee", "identity.json")
}

// loadOrCreateIdentity loads existing identity or creates a new one
func loadOrCreateIdentity() (*identity.Identity, error) {
	identityPath := getIdentityPath()

	// Try to load existing identity
	if _, err := os.Stat(identityPath); err == nil {
		return identity.LoadFromFile(identityPath)
	}

	// Create new identity
	fmt.Println("No existing identity found, generating new identity...")
	id, err := identity.GenerateIdentity()
	if err != nil {
		return nil, fmt.Errorf("failed to generate identity: %w", err)
	}

	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(identityPath), 0700); err != nil {
		return nil, fmt.Errorf("failed to create identity directory: %w", err)
	}

	// Save identity
	if err := id.SaveToFile(identityPath); err != nil {
		return nil, fmt.Errorf("failed to save identity: %w", err)
	}

	fmt.Printf("New identity created and saved to %s\n", identityPath)
	return id, nil
}

// startCommand implements the start subcommand
func startCommand() error {
	fmt.Println("Starting bee agent...")

	// Load or create identity
	id, err := loadOrCreateIdentity()
	if err != nil {
		return err
	}

	// Create agent
	a := agent.New(id)

	// Set default nickname if not set
	if a.Nickname() == "" {
		if err := a.SetNickname("bee"); err != nil {
			return fmt.Errorf("failed to set default nickname: %w", err)
		}
	}

	// Print identity and handle
	fmt.Printf("BID: %s\n", a.BID())
	fmt.Printf("Handle: %s\n", a.Handle(a.Nickname()))

	// Start agent
	ctx := context.Background()
	if err := a.Start(ctx); err != nil {
		return fmt.Errorf("failed to start agent: %w", err)
	}

	// Create control API server
	server := control.NewServer(a)

	// Listen on TCP (for now, Unix socket can be added later)
	listener, err := net.Listen("tcp", "127.0.0.1:27777")
	if err != nil {
		return fmt.Errorf("failed to create control listener: %w", err)
	}
	defer listener.Close()

	fmt.Printf("Control API listening on %s\n", listener.Addr().String())

	// Start control API server
	go func() {
		if err := server.Serve(ctx, listener); err != nil {
			fmt.Printf("Control API error: %v\n", err)
		}
	}()

	// Keep running until interrupted
	fmt.Println("Agent running. Press Ctrl+C to stop.")
	select {} // Block forever
}

// statusCommand implements the status subcommand
func statusCommand() error {
	// Try to connect to control API
	conn, err := net.Dial("tcp", "127.0.0.1:27777")
	if err != nil {
		fmt.Println("Agent is not running")
		return nil
	}
	defer conn.Close()

	// Send GetInfo request
	request := control.Request{
		Method: "GetInfo",
		ID:     "status-check",
	}

	encoder := json.NewEncoder(conn)
	if err := encoder.Encode(request); err != nil {
		return fmt.Errorf("failed to send status request: %w", err)
	}

	// Read response
	decoder := json.NewDecoder(conn)
	var response control.Response
	if err := decoder.Decode(&response); err != nil {
		return fmt.Errorf("failed to read status response: %w", err)
	}

	if response.Error != "" {
		return fmt.Errorf("status error: %s", response.Error)
	}

	// Print status
	result, ok := response.Result.(map[string]interface{})
	if !ok {
		return fmt.Errorf("unexpected response format")
	}

	fmt.Println("Agent is running")
	fmt.Printf("BID: %v\n", result["bid"])
	fmt.Printf("State: %v\n", result["state"])
	if nickname := result["nickname"]; nickname != "" {
		fmt.Printf("Nickname: %v\n", nickname)
		fmt.Printf("Handle: %v\n", result["handle"])
	}

	return nil
}

// keygenCommand implements the keygen subcommand
func keygenCommand() error {
	fmt.Println("Generating new identity...")

	// Generate new identity
	id, err := identity.GenerateIdentity()
	if err != nil {
		return fmt.Errorf("failed to generate identity: %w", err)
	}

	// Get identity path
	identityPath := getIdentityPath()

	// Check if identity already exists
	if _, err := os.Stat(identityPath); err == nil {
		fmt.Printf("Warning: Identity already exists at %s\n", identityPath)
		fmt.Print("Overwrite? (y/N): ")
		var response string
		fmt.Scanln(&response)
		if response != "y" && response != "Y" {
			fmt.Println("Identity generation cancelled")
			return nil
		}
	}

	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(identityPath), 0700); err != nil {
		return fmt.Errorf("failed to create identity directory: %w", err)
	}

	// Save identity
	if err := id.SaveToFile(identityPath); err != nil {
		return fmt.Errorf("failed to save identity: %w", err)
	}

	fmt.Printf("New identity generated and saved to %s\n", identityPath)
	fmt.Printf("BID: %s\n", id.BID())
	fmt.Printf("Honeytag: %s\n", id.Honeytag())

	return nil
}

// handleCommand implements the handle subcommand
func handleCommand() error {
	// Load identity
	id, err := loadOrCreateIdentity()
	if err != nil {
		return err
	}

	// Check if agent is running to get current nickname
	conn, err := net.Dial("tcp", "127.0.0.1:27777")
	if err == nil {
		defer conn.Close()

		// Send GetInfo request
		request := control.Request{
			Method: "GetInfo",
			ID:     "handle-check",
		}

		encoder := json.NewEncoder(conn)
		if err := encoder.Encode(request); err == nil {
			decoder := json.NewDecoder(conn)
			var response control.Response
			if err := decoder.Decode(&response); err == nil && response.Error == "" {
				result, ok := response.Result.(map[string]interface{})
				if ok {
					fmt.Printf("BID: %v\n", result["bid"])
					if nickname := result["nickname"]; nickname != "" {
						fmt.Printf("Nickname: %v\n", nickname)
						fmt.Printf("Handle: %v\n", result["handle"])
					} else {
						fmt.Println("No nickname set")
					}
					return nil
				}
			}
		}
	}

	// Agent not running, show identity info
	fmt.Printf("BID: %s\n", id.BID())
	fmt.Printf("Honeytag: %s\n", id.Honeytag())
	fmt.Println("No nickname set (agent not running)")

	return nil
}

// peersCommand implements the peers subcommand
func peersCommand() error {
	// Connect to control API
	conn, err := net.Dial("tcp", "127.0.0.1:27777")
	if err != nil {
		return fmt.Errorf("failed to connect to agent (is it running?): %w", err)
	}
	defer conn.Close()

	// Send peers request
	request := map[string]interface{}{
		"method": "peers",
	}

	if err := json.NewEncoder(conn).Encode(request); err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}

	// Read response
	var response map[string]interface{}
	if err := json.NewDecoder(conn).Decode(&response); err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	// Check for error
	if errMsg, exists := response["error"]; exists {
		return fmt.Errorf("agent error: %v", errMsg)
	}

	// Display peers
	if peers, exists := response["peers"]; exists {
		if peerList, ok := peers.([]interface{}); ok {
			if len(peerList) == 0 {
				fmt.Println("No peers discovered yet")
				return nil
			}

			fmt.Printf("Discovered peers (%d):\n\n", len(peerList))
			for i, peer := range peerList {
				if peerMap, ok := peer.(map[string]interface{}); ok {
					fmt.Printf("%d. BID: %v\n", i+1, peerMap["bid"])
					if addrs, ok := peerMap["addrs"].([]interface{}); ok && len(addrs) > 0 {
						fmt.Printf("   Addresses: %v\n", addrs)
					}
					if lastSeen, ok := peerMap["last_seen"].(string); ok {
						fmt.Printf("   Last seen: %v\n", lastSeen)
					}
					fmt.Println()
				}
			}
		}
	}

	return nil
}

// seedsCommand implements the seeds subcommand
func seedsCommand() error {
	if len(os.Args) < 3 {
		fmt.Println("Usage:")
		fmt.Println("  bee seeds list              - List current seed nodes")
		fmt.Println("  bee seeds add <bid> <addr>  - Add a new seed node")
		fmt.Println("  bee seeds add <bid> <addr> <name> - Add a new seed node with name")
		return nil
	}

	subcommand := os.Args[2]
	switch subcommand {
	case "list":
		return seedsListCommand()
	case "add":
		return seedsAddCommand()
	default:
		return fmt.Errorf("unknown seeds subcommand: %s", subcommand)
	}
}

// seedsListCommand lists all configured seed nodes
func seedsListCommand() error {
	// Connect to control API
	conn, err := net.Dial("tcp", "127.0.0.1:27777")
	if err != nil {
		return fmt.Errorf("failed to connect to agent (is it running?): %w", err)
	}
	defer conn.Close()

	// Send seeds list request
	request := map[string]interface{}{
		"method": "seeds.list",
	}

	if err := json.NewEncoder(conn).Encode(request); err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}

	// Read response
	var response map[string]interface{}
	if err := json.NewDecoder(conn).Decode(&response); err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	// Check for error
	if errMsg, exists := response["error"]; exists {
		return fmt.Errorf("agent error: %v", errMsg)
	}

	// Display seeds
	if seeds, exists := response["seeds"]; exists {
		if seedList, ok := seeds.([]interface{}); ok {
			if len(seedList) == 0 {
				fmt.Println("No seed nodes configured")
				return nil
			}

			fmt.Printf("Configured seed nodes (%d):\n\n", len(seedList))
			for i, seed := range seedList {
				if seedMap, ok := seed.(map[string]interface{}); ok {
					fmt.Printf("%d. BID: %v\n", i+1, seedMap["bid"])
					if name, ok := seedMap["name"].(string); ok && name != "" {
						fmt.Printf("   Name: %v\n", name)
					}
					if addrs, ok := seedMap["addrs"].([]interface{}); ok && len(addrs) > 0 {
						fmt.Printf("   Addresses: %v\n", addrs)
					}
					fmt.Println()
				}
			}
		}
	}

	return nil
}

// seedsAddCommand adds a new seed node
func seedsAddCommand() error {
	if len(os.Args) < 5 {
		return fmt.Errorf("usage: bee seeds add <bid> <addr> [name]")
	}

	bid := os.Args[3]
	addr := os.Args[4]
	name := ""
	if len(os.Args) > 5 {
		name = os.Args[5]
	}

	// Connect to control API
	conn, err := net.Dial("tcp", "127.0.0.1:27777")
	if err != nil {
		return fmt.Errorf("failed to connect to agent (is it running?): %w", err)
	}
	defer conn.Close()

	// Send seeds add request
	request := map[string]interface{}{
		"method": "seeds.add",
		"params": map[string]interface{}{
			"bid":   bid,
			"addrs": []string{addr},
			"name":  name,
		},
	}

	if err := json.NewEncoder(conn).Encode(request); err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}

	// Read response
	var response map[string]interface{}
	if err := json.NewDecoder(conn).Decode(&response); err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	// Check for error
	if errMsg, exists := response["error"]; exists {
		return fmt.Errorf("agent error: %v", errMsg)
	}

	fmt.Printf("Added seed node: %s\n", bid)
	if name != "" {
		fmt.Printf("Name: %s\n", name)
	}
	fmt.Printf("Address: %s\n", addr)

	return nil
}
