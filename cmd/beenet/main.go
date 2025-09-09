// Package main implements the Beenet CLI as specified in §2.
package main

import (
	"fmt"
	"os"
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
		fmt.Println("Starting Beenet node... (not implemented yet)")
	case "create":
		fmt.Println("Creating new Beenet swarm... (not implemented yet)")
	case "join":
		fmt.Println("Joining Beenet swarm... (not implemented yet)")
	case "name":
		fmt.Println("Name operations... (not implemented yet)")
	default:
		fmt.Printf("Unknown command: %s\n\n", command)
		printUsage()
		os.Exit(1)
	}
}

func printVersion() {
	fmt.Printf("Beenet %s\n", version)
	fmt.Printf("Built: %s\n", buildTime)
	fmt.Printf("Commit: %s\n", commitHash)
}

func printUsage() {
	fmt.Printf(`Beenet v%s - A P2P mesh network

Usage:
  beenet <command> [options]

Commands:
  start     Start a Beenet node (join mode - default)
  create    Create a new Beenet swarm (explicit)
  join      Join a Beenet swarm via invite
  name      Name operations (claim, transfer, delegate)
  version   Show version information
  help      Show this help message

Examples:
  # Join mode (default)
  beenet start --swarm <swarm-id> --seed <multiaddr> [--psk <hex> | --token <jwt>]

  # Create mode (explicit)
  beenet create --name teamnet --seed-self --listen /ip4/0.0.0.0/udp/27487/quic

  # Import invite
  beenet join beenet:swarm/<b32id>?seed=/ip4/203.0.113.5/udp/27487/quic&psk=<b32>

  # Name operations
  beenet name claim brad
  beenet name transfer brad --to bee:key:z6Mk...
  beenet name delegate brad --device bee:key:z6Ms...

For more information, visit: https://github.com/WebFirstLanguage/beenet

`, version)
}
