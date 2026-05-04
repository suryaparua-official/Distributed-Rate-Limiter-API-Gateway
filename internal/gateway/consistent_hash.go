package gateway

import (
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"sort"
	"sync"
)

// ConsistentHash implements a consistent hash ring for distributing
// requests across multiple rate limiter nodes.
// Virtual nodes (replicas) ensure even distribution.
type ConsistentHash struct {
	mu       sync.RWMutex
	replicas int               // virtual nodes per real node
	ring     map[uint32]string // hash → node address
	sorted   []uint32          // sorted hashes for binary search
}

func NewConsistentHash(replicas int) *ConsistentHash {
	return &ConsistentHash{
		replicas: replicas,
		ring:     make(map[uint32]string),
	}
}

// AddNode adds a real node with `replicas` virtual nodes on the ring.
func (ch *ConsistentHash) AddNode(addr string) {
	ch.mu.Lock()
	defer ch.mu.Unlock()

	for i := 0; i < ch.replicas; i++ {
		hash := ch.hash(fmt.Sprintf("%s#%d", addr, i))
		ch.ring[hash] = addr
		ch.sorted = append(ch.sorted, hash)
	}
	sort.Slice(ch.sorted, func(i, j int) bool {
		return ch.sorted[i] < ch.sorted[j]
	})
}

// RemoveNode removes a node and all its virtual nodes from the ring.
func (ch *ConsistentHash) RemoveNode(addr string) {
	ch.mu.Lock()
	defer ch.mu.Unlock()

	for i := 0; i < ch.replicas; i++ {
		hash := ch.hash(fmt.Sprintf("%s#%d", addr, i))
		delete(ch.ring, hash)
	}

	// Rebuild sorted slice
	ch.sorted = ch.sorted[:0]
	for h := range ch.ring {
		ch.sorted = append(ch.sorted, h)
	}
	sort.Slice(ch.sorted, func(i, j int) bool {
		return ch.sorted[i] < ch.sorted[j]
	})
}

// GetNode returns the node responsible for the given key.
// Uses binary search on sorted ring — O(log n)
func (ch *ConsistentHash) GetNode(key string) string {
	ch.mu.RLock()
	defer ch.mu.RUnlock()

	if len(ch.ring) == 0 {
		return ""
	}

	hash := ch.hash(key)

	// Binary search: find first node with hash >= key hash
	idx := sort.Search(len(ch.sorted), func(i int) bool {
		return ch.sorted[i] >= hash
	})

	// Wrap around — ring is circular
	if idx == len(ch.sorted) {
		idx = 0
	}

	return ch.ring[ch.sorted[idx]]
}

// hash converts a string key to uint32 using SHA256
func (ch *ConsistentHash) hash(key string) uint32 {
	h := sha256.Sum256([]byte(key))
	return binary.BigEndian.Uint32(h[:4])
}

// Nodes returns all unique real nodes in the ring
func (ch *ConsistentHash) Nodes() []string {
	ch.mu.RLock()
	defer ch.mu.RUnlock()

	seen := make(map[string]bool)
	var nodes []string
	for _, addr := range ch.ring {
		if !seen[addr] {
			seen[addr] = true
			nodes = append(nodes, addr)
		}
	}
	return nodes
}