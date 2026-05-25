package authority

import (
	"sync"
	"time"
)

// Mode describes authoritative DNS deployment shape.
type Mode string

const (
	ModeSingle Mode = "single"
	ModeMulti  Mode = "multi"
	ModeAnycast Mode = "anycast"
)

// Node represents one authoritative transport entry point.
type Node struct {
	ID       string
	Address  string
	Healthy  bool
	LastSeen time.Time
	Weight   int
}

// Registry tracks authoritative nodes for client/server selection.
type Registry struct {
	mu     sync.RWMutex
	mode   Mode
	nodes  []Node
	round  uint64
}

// NewRegistry creates an authority registry.
func NewRegistry(mode Mode, peers []string) *Registry {
	r := &Registry{mode: mode}
	if mode == "" {
		mode = ModeSingle
		r.mode = mode
	}
	for i, p := range peers {
		if p == "" {
			continue
		}
		r.nodes = append(r.nodes, Node{
			ID:      p,
			Address: p,
			Healthy: true,
			Weight:  1,
		})
		if i == 0 && mode == ModeSingle {
			break
		}
	}
	if len(r.nodes) == 0 && mode == ModeSingle {
		r.nodes = []Node{{ID: "local", Address: "0.0.0.0:53", Healthy: true, Weight: 1}}
	}
	return r
}

// Mode returns deployment mode.
func (r *Registry) Mode() Mode {
	if r == nil {
		return ModeSingle
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.mode
}

// Select returns the best authority endpoint for new sessions.
func (r *Registry) Select() (Node, bool) {
	if r == nil {
		return Node{}, false
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.nodes) == 0 {
		return Node{}, false
	}
	healthy := make([]Node, 0, len(r.nodes))
	for _, n := range r.nodes {
		if n.Healthy {
			healthy = append(healthy, n)
		}
	}
	if len(healthy) == 0 {
		return r.nodes[0], true
	}
	if r.mode == ModeMulti || r.mode == ModeAnycast {
		r.round++
		return healthy[int(r.round%uint64(len(healthy)))], true
	}
	return healthy[0], true
}

// MarkHealth updates node health.
func (r *Registry) MarkHealth(id string, healthy bool) {
	if r == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	for i := range r.nodes {
		if r.nodes[i].ID == id || r.nodes[i].Address == id {
			r.nodes[i].Healthy = healthy
			r.nodes[i].LastSeen = time.Now()
			return
		}
	}
}

// Nodes returns a snapshot of registered nodes.
func (r *Registry) Nodes() []Node {
	if r == nil {
		return nil
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]Node, len(r.nodes))
	copy(out, r.nodes)
	return out
}
