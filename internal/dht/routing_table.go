// Package dht implements Kademlia routing table
package dht

import (
	"sync"
	"time"
)

// RoutingTable implements a Kademlia routing table with 256 buckets
type RoutingTable struct {
	mu      sync.RWMutex
	localID NodeID
	buckets [256]*Bucket
}

// NewRoutingTable creates a new routing table for the given local node ID
func NewRoutingTable(localID NodeID) *RoutingTable {
	rt := &RoutingTable{
		localID: localID,
	}
	
	// Initialize all buckets
	for i := 0; i < 256; i++ {
		rt.buckets[i] = NewBucket()
	}
	
	return rt
}

// Add adds a node to the appropriate bucket in the routing table
func (rt *RoutingTable) Add(node *Node) bool {
	if node.ID == rt.localID {
		return false // Don't add ourselves
	}
	
	bucketIndex := rt.getBucketIndex(node.ID)
	return rt.buckets[bucketIndex].Add(node)
}

// Remove removes a node from the routing table
func (rt *RoutingTable) Remove(nodeID NodeID) bool {
	if nodeID == rt.localID {
		return false // Don't remove ourselves
	}
	
	bucketIndex := rt.getBucketIndex(nodeID)
	return rt.buckets[bucketIndex].Remove(nodeID)
}

// Get retrieves a node by ID
func (rt *RoutingTable) Get(nodeID NodeID) *Node {
	if nodeID == rt.localID {
		return nil // Don't return ourselves
	}
	
	bucketIndex := rt.getBucketIndex(nodeID)
	return rt.buckets[bucketIndex].Get(nodeID)
}

// GetClosest returns the k closest nodes to the target ID
func (rt *RoutingTable) GetClosest(target NodeID, k int) []*Node {
	rt.mu.RLock()
	defer rt.mu.RUnlock()
	
	var candidates []*Node
	
	// Start with the bucket that should contain the target
	targetBucket := rt.getBucketIndex(target)
	
	// Collect nodes from buckets, starting with the target bucket and expanding outward
	collected := make(map[int]bool)
	
	// Add nodes from target bucket first
	candidates = append(candidates, rt.buckets[targetBucket].GetAll()...)
	collected[targetBucket] = true
	
	// Expand outward from target bucket
	for distance := 1; len(candidates) < k && distance < 256; distance++ {
		// Check bucket above
		if targetBucket+distance < 256 && !collected[targetBucket+distance] {
			candidates = append(candidates, rt.buckets[targetBucket+distance].GetAll()...)
			collected[targetBucket+distance] = true
		}
		
		// Check bucket below
		if targetBucket-distance >= 0 && !collected[targetBucket-distance] {
			candidates = append(candidates, rt.buckets[targetBucket-distance].GetAll()...)
			collected[targetBucket-distance] = true
		}
	}
	
	// If we still don't have enough, collect from all remaining buckets
	if len(candidates) < k {
		for i := 0; i < 256; i++ {
			if !collected[i] {
				candidates = append(candidates, rt.buckets[i].GetAll()...)
			}
		}
	}
	
	// Sort candidates by distance to target and return top k
	return rt.sortByDistance(candidates, target, k)
}

// GetAllNodes returns all nodes in the routing table
func (rt *RoutingTable) GetAllNodes() []*Node {
	rt.mu.RLock()
	defer rt.mu.RUnlock()
	
	var nodes []*Node
	for _, bucket := range rt.buckets {
		nodes = append(nodes, bucket.GetAll()...)
	}
	return nodes
}

// Size returns the total number of nodes in the routing table
func (rt *RoutingTable) Size() int {
	rt.mu.RLock()
	defer rt.mu.RUnlock()
	
	total := 0
	for _, bucket := range rt.buckets {
		total += bucket.Size()
	}
	return total
}

// RemoveStale removes stale nodes from all buckets
func (rt *RoutingTable) RemoveStale(timeout time.Duration) int {
	rt.mu.Lock()
	defer rt.mu.Unlock()
	
	total := 0
	for _, bucket := range rt.buckets {
		total += bucket.RemoveStale(timeout)
	}
	return total
}

// GetBucketInfo returns information about bucket utilization
func (rt *RoutingTable) GetBucketInfo() map[int]int {
	rt.mu.RLock()
	defer rt.mu.RUnlock()
	
	info := make(map[int]int)
	for i, bucket := range rt.buckets {
		size := bucket.Size()
		if size > 0 {
			info[i] = size
		}
	}
	return info
}

// getBucketIndex calculates which bucket a node ID should go into
func (rt *RoutingTable) getBucketIndex(nodeID NodeID) int {
	// Calculate XOR distance
	distance := rt.localID.Distance(nodeID)
	
	// Find the position of the most significant bit
	for i := 0; i < 32; i++ {
		if distance[i] != 0 {
			// Find the position of the highest bit in this byte
			for j := 7; j >= 0; j-- {
				if (distance[i]>>j)&1 == 1 {
					return 255 - (i*8 + (7 - j))
				}
			}
		}
	}
	
	// If distance is 0 (shouldn't happen as we filter out self), use bucket 0
	return 0
}

// sortByDistance sorts nodes by distance to target and returns up to k nodes
func (rt *RoutingTable) sortByDistance(nodes []*Node, target NodeID, k int) []*Node {
	if len(nodes) == 0 {
		return nil
	}
	
	// Create distance pairs for sorting
	type distancePair struct {
		node     *Node
		distance NodeID
	}
	
	pairs := make([]distancePair, len(nodes))
	for i, node := range nodes {
		pairs[i] = distancePair{
			node:     node,
			distance: node.ID.Distance(target),
		}
	}
	
	// Sort by distance (insertion sort for small arrays, otherwise use a more efficient sort)
	for i := 1; i < len(pairs); i++ {
		key := pairs[i]
		j := i - 1
		
		for j >= 0 && key.distance.Less(pairs[j].distance) {
			pairs[j+1] = pairs[j]
			j--
		}
		pairs[j+1] = key
	}
	
	// Extract nodes and return up to k
	if k > len(pairs) {
		k = len(pairs)
	}
	
	result := make([]*Node, k)
	for i := 0; i < k; i++ {
		result[i] = pairs[i].node
	}
	
	return result
}
