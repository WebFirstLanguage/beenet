// Package dht implements Kademlia routing table buckets
package dht

import (
	"sort"
	"sync"
	"time"

	"github.com/WebFirstLanguage/beenet/pkg/constants"
)

// Bucket represents a k-bucket in the Kademlia routing table
type Bucket struct {
	mu    sync.RWMutex
	nodes []*Node
	
	// Bucket configuration
	maxSize int
	
	// Replacement cache for when bucket is full
	replacements []*Node
	maxReplacements int
}

// NewBucket creates a new k-bucket with the specified maximum size
func NewBucket() *Bucket {
	return &Bucket{
		nodes:           make([]*Node, 0, constants.DHTBucketSize),
		maxSize:         constants.DHTBucketSize,
		replacements:    make([]*Node, 0, constants.DHTBucketSize),
		maxReplacements: constants.DHTBucketSize,
	}
}

// Add attempts to add a node to the bucket
// Returns true if the node was added, false if the bucket is full
func (b *Bucket) Add(node *Node) bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	
	// Check if node already exists
	for i, existing := range b.nodes {
		if existing.ID == node.ID {
			// Update existing node and move to end (most recently seen)
			b.nodes[i] = node
			b.moveToEnd(i)
			return true
		}
	}
	
	// If bucket has space, add the node
	if len(b.nodes) < b.maxSize {
		b.nodes = append(b.nodes, node)
		return true
	}
	
	// Bucket is full, add to replacement cache
	b.addToReplacements(node)
	return false
}

// Remove removes a node from the bucket
func (b *Bucket) Remove(nodeID NodeID) bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	
	for i, node := range b.nodes {
		if node.ID == nodeID {
			// Remove node
			b.nodes = append(b.nodes[:i], b.nodes[i+1:]...)
			
			// Try to fill from replacement cache
			b.promoteFromReplacements()
			return true
		}
	}
	
	// Also remove from replacements if present
	for i, node := range b.replacements {
		if node.ID == nodeID {
			b.replacements = append(b.replacements[:i], b.replacements[i+1:]...)
			return true
		}
	}
	
	return false
}

// Get retrieves a node by ID
func (b *Bucket) Get(nodeID NodeID) *Node {
	b.mu.RLock()
	defer b.mu.RUnlock()
	
	for _, node := range b.nodes {
		if node.ID == nodeID {
			return node.Copy()
		}
	}
	return nil
}

// GetAll returns all nodes in the bucket
func (b *Bucket) GetAll() []*Node {
	b.mu.RLock()
	defer b.mu.RUnlock()
	
	result := make([]*Node, len(b.nodes))
	for i, node := range b.nodes {
		result[i] = node.Copy()
	}
	return result
}

// Size returns the number of nodes in the bucket
func (b *Bucket) Size() int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return len(b.nodes)
}

// IsFull returns true if the bucket is at maximum capacity
func (b *Bucket) IsFull() bool {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return len(b.nodes) >= b.maxSize
}

// GetClosest returns the k closest nodes to the target ID
func (b *Bucket) GetClosest(target NodeID, k int) []*Node {
	b.mu.RLock()
	defer b.mu.RUnlock()
	
	if len(b.nodes) == 0 {
		return nil
	}
	
	// Create a copy of nodes for sorting
	nodes := make([]*Node, len(b.nodes))
	for i, node := range b.nodes {
		nodes[i] = node.Copy()
	}
	
	// Sort by distance to target
	sort.Slice(nodes, func(i, j int) bool {
		distI := nodes[i].ID.Distance(target)
		distJ := nodes[j].ID.Distance(target)
		return distI.Less(distJ)
	})
	
	// Return up to k nodes
	if k > len(nodes) {
		k = len(nodes)
	}
	return nodes[:k]
}

// RemoveStale removes nodes that haven't been seen recently
func (b *Bucket) RemoveStale(timeout time.Duration) int {
	b.mu.Lock()
	defer b.mu.Unlock()
	
	removed := 0
	i := 0
	for i < len(b.nodes) {
		if b.nodes[i].IsStale(timeout) {
			// Remove stale node
			b.nodes = append(b.nodes[:i], b.nodes[i+1:]...)
			removed++
		} else {
			i++
		}
	}
	
	// Promote from replacements to fill gaps
	for removed > 0 && len(b.replacements) > 0 {
		b.promoteFromReplacements()
		removed--
	}
	
	return removed
}

// moveToEnd moves the node at index i to the end of the slice (most recently seen)
func (b *Bucket) moveToEnd(i int) {
	if i == len(b.nodes)-1 {
		return // Already at end
	}
	
	node := b.nodes[i]
	copy(b.nodes[i:], b.nodes[i+1:])
	b.nodes[len(b.nodes)-1] = node
}

// addToReplacements adds a node to the replacement cache
func (b *Bucket) addToReplacements(node *Node) {
	// Check if already in replacements
	for i, existing := range b.replacements {
		if existing.ID == node.ID {
			b.replacements[i] = node
			return
		}
	}
	
	// Add to replacements
	if len(b.replacements) < b.maxReplacements {
		b.replacements = append(b.replacements, node)
	} else {
		// Replace oldest replacement
		copy(b.replacements, b.replacements[1:])
		b.replacements[len(b.replacements)-1] = node
	}
}

// promoteFromReplacements moves a node from replacements to the main bucket
func (b *Bucket) promoteFromReplacements() {
	if len(b.replacements) == 0 || len(b.nodes) >= b.maxSize {
		return
	}
	
	// Take the most recent replacement
	node := b.replacements[len(b.replacements)-1]
	b.replacements = b.replacements[:len(b.replacements)-1]
	
	// Add to main bucket
	b.nodes = append(b.nodes, node)
}
