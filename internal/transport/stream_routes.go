package transport

import "sync"

const streamMigrateLossThreshold = 3

// StreamRouteTable keeps transport-owned path stickiness per stream.
type StreamRouteTable struct {
	mu     sync.RWMutex
	routes map[uint16]*streamRoute
}

type streamRoute struct {
	pathKey      string
	lossStreak   int
	timeoutStreak int
}

// NewStreamRouteTable creates a stream routing table.
func NewStreamRouteTable() *StreamRouteTable {
	return &StreamRouteTable{routes: make(map[uint16]*streamRoute)}
}

// PreferredPath returns sticky path for a stream if still viable.
func (t *StreamRouteTable) PreferredPath(streamID uint16, ranked []string, pm *PathManager) string {
	if len(ranked) == 0 {
		return ""
	}
	if t == nil || streamID == 0 {
		return ranked[0]
	}
	t.mu.RLock()
	route := t.routes[streamID]
	sticky := ""
	if route != nil {
		sticky = route.pathKey
	}
	t.mu.RUnlock()
	if sticky == "" {
		return ranked[0]
	}
	inRanked := false
	for _, k := range ranked {
		if k == sticky {
			inRanked = true
			break
		}
	}
	if !inRanked {
		return ranked[0]
	}
	if st, ok := pm.Stats(sticky); ok && !st.Disabled && st.LossRate < 0.5 && st.TimeoutRate < 0.5 {
		return sticky
	}
	return ranked[0]
}

// NoteOutcome updates stickiness after ack/loss/timeout on a stream data path.
func (t *StreamRouteTable) NoteOutcome(streamID uint16, pathKey string, acked bool, timedOut bool) {
	if t == nil || streamID == 0 || pathKey == "" {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	route := t.routes[streamID]
	if route == nil {
		route = &streamRoute{pathKey: pathKey}
		t.routes[streamID] = route
	}
	if acked {
		route.lossStreak = 0
		route.timeoutStreak = 0
		route.pathKey = pathKey
		return
	}
	if timedOut {
		route.timeoutStreak++
	} else {
		route.lossStreak++
	}
	if route.lossStreak >= streamMigrateLossThreshold || route.timeoutStreak >= streamMigrateLossThreshold {
		route.pathKey = ""
		route.lossStreak = 0
		route.timeoutStreak = 0
	}
}

// CleanupStream removes route state when stream closes.
func (t *StreamRouteTable) CleanupStream(streamID uint16) {
	if t == nil || streamID == 0 {
		return
	}
	t.mu.Lock()
	delete(t.routes, streamID)
	t.mu.Unlock()
}
