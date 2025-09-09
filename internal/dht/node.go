// Package dht implements a Kademlia-compatible Distributed Hash Table
// as specified in §6.2 and §14 of the Beenet specification.
package dht

import (
	"fmt"
	"net"
	"time"

	"lukechampine.com/blake3"
)

// NodeID represents a 256-bit node identifier in the DHT keyspace
type NodeID [32]byte

// Node represents a peer node in the DHT
type Node struct {
	ID       NodeID    // 256-bit node identifier
	BID      string    // Beenet ID (multibase/multicodec Ed25519-pub)
	Addrs    []string  // Multiaddresses for connecting to this node
	LastSeen time.Time // Last time we heard from this node

	// Connection state
	Connected bool
	Conn      net.Conn // Active connection if any
}

// NewNodeID creates a NodeID from a BID using BLAKE3 hash
func NewNodeID(bid string) NodeID {
	hash := blake3.Sum256([]byte(bid))
	return NodeID(hash)
}

// NewNode creates a new DHT node
func NewNode(bid string, addrs []string) *Node {
	return &Node{
		ID:       NewNodeID(bid),
		BID:      bid,
		Addrs:    addrs,
		LastSeen: time.Now(),
	}
}

// Distance calculates the XOR distance between two node IDs
func (n NodeID) Distance(other NodeID) NodeID {
	var result NodeID
	for i := 0; i < 32; i++ {
		result[i] = n[i] ^ other[i]
	}
	return result
}

// String returns the hex representation of the NodeID
func (n NodeID) String() string {
	return fmt.Sprintf("%x", n[:])
}

// Bytes returns the NodeID as a byte slice
func (n NodeID) Bytes() []byte {
	return n[:]
}

// IsZero returns true if the NodeID is all zeros
func (n NodeID) IsZero() bool {
	for _, b := range n {
		if b != 0 {
			return false
		}
	}
	return true
}

// Less returns true if this NodeID is less than the other (for sorting)
func (n NodeID) Less(other NodeID) bool {
	for i := 0; i < 32; i++ {
		if n[i] < other[i] {
			return true
		}
		if n[i] > other[i] {
			return false
		}
	}
	return false
}

// CommonPrefixLen returns the number of leading bits that are the same
func (n NodeID) CommonPrefixLen(other NodeID) int {
	for i := 0; i < 32; i++ {
		xor := n[i] ^ other[i]
		if xor == 0 {
			continue
		}
		// Count leading zeros in the XOR byte
		for j := 7; j >= 0; j-- {
			if (xor>>j)&1 == 1 {
				return i*8 + (7 - j)
			}
		}
	}
	return 256 // All bits are the same
}

// IsValid checks if the node has valid data
func (n *Node) IsValid() bool {
	return n.BID != "" && len(n.Addrs) > 0 && !n.ID.IsZero()
}

// UpdateLastSeen updates the last seen timestamp
func (n *Node) UpdateLastSeen() {
	n.LastSeen = time.Now()
}

// IsStale returns true if the node hasn't been seen recently
func (n *Node) IsStale(timeout time.Duration) bool {
	return time.Since(n.LastSeen) > timeout
}

// Copy creates a deep copy of the node
func (n *Node) Copy() *Node {
	addrs := make([]string, len(n.Addrs))
	copy(addrs, n.Addrs)

	return &Node{
		ID:        n.ID,
		BID:       n.BID,
		Addrs:     addrs,
		LastSeen:  n.LastSeen,
		Connected: n.Connected,
		Conn:      n.Conn,
	}
}

// String returns a string representation of the node
func (n *Node) String() string {
	return fmt.Sprintf("Node{ID: %s, BID: %s, Addrs: %v, LastSeen: %v}",
		n.ID.String()[:16]+"...", n.BID, n.Addrs, n.LastSeen.Format(time.RFC3339))
}
