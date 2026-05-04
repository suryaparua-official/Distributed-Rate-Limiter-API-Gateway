package gateway

import (
	"fmt"
	"testing"
)

func TestConsistentHash_BasicRouting(t *testing.T) {
	ch := NewConsistentHash(100) // 100 virtual nodes per real node
	ch.AddNode("node1:50051")
	ch.AddNode("node2:50051")
	ch.AddNode("node3:50051")

	// Same key must always go to same node
	key := "user:123"
	node1 := ch.GetNode(key)
	node2 := ch.GetNode(key)
	node3 := ch.GetNode(key)

	if node1 != node2 || node2 != node3 {
		t.Fatalf("same key routed to different nodes: %s, %s, %s", node1, node2, node3)
	}
	t.Logf("key=%s → node=%s (consistent ✓)", key, node1)
}

func TestConsistentHash_Distribution(t *testing.T) {
	ch := NewConsistentHash(150)
	nodes := []string{"node1:50051", "node2:50051", "node3:50051"}
	for _, n := range nodes {
		ch.AddNode(n)
	}

	// Distribute 10000 keys — check even distribution
	dist := make(map[string]int)
	for i := 0; i < 10000; i++ {
		key := fmt.Sprintf("user:%d", i)
		node := ch.GetNode(key)
		dist[node]++
	}

	t.Log("Distribution across nodes:")
	for node, count := range dist {
		pct := float64(count) / 100.0
		t.Logf("  %s → %d requests (%.1f%%)", node, count, pct)
	}

	// Each node should get roughly 33% ± 10%
	for node, count := range dist {
		pct := float64(count) / 10000.0 * 100
		if pct < 23 || pct > 43 {
			t.Errorf("node %s has uneven distribution: %.1f%%", node, pct)
		}
	}
}

func TestConsistentHash_NodeRemoval(t *testing.T) {
	ch := NewConsistentHash(100)
	ch.AddNode("node1:50051")
	ch.AddNode("node2:50051")
	ch.AddNode("node3:50051")

	// Track which node each key goes to before removal
	before := make(map[string]string)
	for i := 0; i < 1000; i++ {
		key := fmt.Sprintf("user:%d", i)
		before[key] = ch.GetNode(key)
	}

	// Remove node3
	ch.RemoveNode("node3:50051")

	// Count how many keys were remapped
	remapped := 0
	for i := 0; i < 1000; i++ {
		key := fmt.Sprintf("user:%d", i)
		after := ch.GetNode(key)
		if before[key] != after {
			remapped++
		}
	}

	pct := float64(remapped) / 1000.0 * 100
	t.Logf("Keys remapped after removing 1/3 nodes: %d/1000 (%.1f%%)", remapped, pct)

	// Should remap ~33% of keys (only node3's keys move)
	if pct > 45 {
		t.Errorf("too many keys remapped: %.1f%% (expected ~33%%)", pct)
	}
}

func BenchmarkConsistentHash_GetNode(b *testing.B) {
	ch := NewConsistentHash(150)
	ch.AddNode("node1:50051")
	ch.AddNode("node2:50051")
	ch.AddNode("node3:50051")

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			ch.GetNode(fmt.Sprintf("user:%d", i))
			i++
		}
	})
}