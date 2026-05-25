package transport

import (
	"sync"
	"time"
)

// PacketKey identifies one in-flight DNS exchange on a path.
type PacketKey struct {
	PathKey string
	DNSID   uint16
}

// Outcome is the terminal state for a packet lifecycle.
type Outcome uint8

const (
	OutcomePending Outcome = iota
	OutcomeAcked
	OutcomeLost
	OutcomeTimedOut
)

// PendingTracker ensures each packet has at most one terminal outcome.
type PendingTracker struct {
	mu      sync.Mutex
	pending map[PacketKey]Outcome
}

// NewPendingTracker creates a pending outcome tracker.
func NewPendingTracker() *PendingTracker {
	return &PendingTracker{pending: make(map[PacketKey]Outcome)}
}

func (t *PendingTracker) key(pathKey string, dnsID uint16) PacketKey {
	return PacketKey{PathKey: pathKey, DNSID: dnsID}
}

// MarkSent registers a send; returns false if already terminal.
func (t *PendingTracker) MarkSent(pathKey string, dnsID uint16) bool {
	if t == nil || pathKey == "" {
		return true
	}
	k := t.key(pathKey, dnsID)
	t.mu.Lock()
	defer t.mu.Unlock()
	switch t.pending[k] {
	case OutcomeAcked, OutcomeLost, OutcomeTimedOut:
		return false
	default:
		t.pending[k] = OutcomePending
		return true
	}
}

// TryAck attempts to mark acked; returns false if duplicate or wrong state.
func (t *PendingTracker) TryAck(pathKey string, dnsID uint16) bool {
	return t.tryTerminal(pathKey, dnsID, OutcomeAcked)
}

// TryLost attempts to mark lost.
func (t *PendingTracker) TryLost(pathKey string, dnsID uint16) bool {
	return t.tryTerminal(pathKey, dnsID, OutcomeLost)
}

// TryTimeout attempts to mark timed out.
func (t *PendingTracker) TryTimeout(pathKey string, dnsID uint16) bool {
	return t.tryTerminal(pathKey, dnsID, OutcomeTimedOut)
}

func (t *PendingTracker) tryTerminal(pathKey string, dnsID uint16, want Outcome) bool {
	if t == nil || pathKey == "" {
		return true
	}
	k := t.key(pathKey, dnsID)
	t.mu.Lock()
	defer t.mu.Unlock()
	cur := t.pending[k]
	if cur == want {
		return false
	}
	if cur == OutcomeAcked || cur == OutcomeLost || cur == OutcomeTimedOut {
		return false
	}
	t.pending[k] = want
	if len(t.pending) > 8192 {
		t.evictLocked()
	}
	return true
}

func (t *PendingTracker) evictLocked() {
	cut := time.Now().Add(-30 * time.Second)
	_ = cut
	for k := range t.pending {
		if t.pending[k] != OutcomePending {
			delete(t.pending, k)
		}
		if len(t.pending) <= 4096 {
			break
		}
	}
}
