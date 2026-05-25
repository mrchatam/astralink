package transport

import (
	"sync"
	"time"
)

// Path represents one multipath resolver channel.
type Path struct {
	Key        string
	Active     bool
	Standby    bool
	Role       PathRole
	metrics    *PathMetrics
	congestion *CongestionState
	mu         sync.RWMutex
}

// PathManager tracks all resolver channels.
type PathManager struct {
	mu            sync.RWMutex
	paths         map[string]*Path
	cfg           Config
	globalInflight int
}

// NewPathManager creates a path manager.
func NewPathManager(cfg Config) *PathManager {
	return &PathManager{
		paths: make(map[string]*Path),
		cfg:   cfg,
	}
}

func (pm *PathManager) getOrCreate(key string) *Path {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	p, ok := pm.paths[key]
	if !ok {
		p = &Path{
			Key:        key,
			metrics:    &PathMetrics{Key: key},
			congestion: NewCongestionState(pm.cfg.CongestionProfile),
		}
		pm.paths[key] = p
	}
	return p
}

// Upsert registers or updates a path.
func (pm *PathManager) Upsert(key string, active bool) {
	p := pm.getOrCreate(key)
	p.mu.Lock()
	p.Active = active
	p.Standby = !active
	p.mu.Unlock()
}

// SetStats merges external stats into path metrics.
func (pm *PathManager) SetStats(key string, stats PathStats) {
	p := pm.getOrCreate(key)
	p.mu.Lock()
	m := p.metrics
	m.RTT = stats.RTT
	m.LossRate = stats.LossRate
	m.TimeoutRate = stats.TimeoutRate
	m.Disabled = stats.Disabled
	p.mu.Unlock()
}

// ActivePaths returns keys marked active, excluding disabled paths.
func (pm *PathManager) ActivePaths() []string {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	out := make([]string, 0, len(pm.paths))
	for k, p := range pm.paths {
		p.mu.RLock()
		if p.Active && !p.metrics.Disabled {
			out = append(out, k)
		}
		p.mu.RUnlock()
	}
	return out
}

// StandbyPaths returns standby path keys.
func (pm *PathManager) StandbyPaths() []string {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	out := make([]string, 0)
	for k, p := range pm.paths {
		p.mu.RLock()
		if p.Standby && !p.metrics.Disabled {
			out = append(out, k)
		}
		p.mu.RUnlock()
	}
	return out
}

// Stats returns a snapshot of path stats.
func (pm *PathManager) Stats(key string) (PathStats, bool) {
	pm.mu.RLock()
	p, ok := pm.paths[key]
	pm.mu.RUnlock()
	if !ok {
		return PathStats{}, false
	}
	return p.metrics.snapshot(), true
}

// ReserveInflight reserves bytes on path and global budget.
func (pm *PathManager) ReserveInflight(key string, nbytes int) bool {
	if nbytes <= 0 {
		return true
	}
	p := pm.getOrCreate(key)
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.metrics.Disabled || !p.congestion.CanSend(nbytes) {
		return false
	}
	maxGlobal := pm.cfg.MaxGlobalInflightBytes()
	if maxGlobal > 0 && pm.globalInflight+nbytes > maxGlobal {
		return false
	}
	p.congestion.Reserve(nbytes)
	p.metrics.InFlightBytes = p.congestion.InFlight
	p.metrics.CWND = p.congestion.Window
	pm.mu.Lock()
	pm.globalInflight += nbytes
	pm.mu.Unlock()
	return true
}

// ReleaseInflight releases reserved bytes without ACK.
func (pm *PathManager) ReleaseInflight(key string, nbytes int) {
	if nbytes <= 0 {
		return
	}
	pm.mu.RLock()
	p, ok := pm.paths[key]
	pm.mu.RUnlock()
	if !ok {
		return
	}
	p.mu.Lock()
	p.congestion.Release(nbytes)
	p.metrics.InFlightBytes = p.congestion.InFlight
	p.metrics.CWND = p.congestion.Window
	p.mu.Unlock()
	pm.mu.Lock()
	pm.globalInflight -= nbytes
	if pm.globalInflight < 0 {
		pm.globalInflight = 0
	}
	pm.mu.Unlock()
}

// MarkSent records that bytes left the client on this path (no RTT/congestion growth).
func (pm *PathManager) MarkSent(key string, bytes int) {
	pm.mu.RLock()
	p, ok := pm.paths[key]
	pm.mu.RUnlock()
	if !ok {
		return
	}
	p.mu.Lock()
	p.metrics.recordSent(bytes)
	p.Role = updatePathRole(p.metrics, p.Active)
	p.mu.Unlock()
}

// MarkAcked records confirmed delivery with RTT; grows congestion window.
func (pm *PathManager) MarkAcked(key string, rtt time.Duration, bytes int) {
	pm.mu.RLock()
	p, ok := pm.paths[key]
	pm.mu.RUnlock()
	if !ok {
		return
	}
	p.mu.Lock()
	p.congestion.OnAck(bytes)
	p.metrics.InFlightBytes = p.congestion.InFlight
	p.metrics.CWND = p.congestion.Window
	p.metrics.recordAcked(rtt, bytes)
	p.Role = updatePathRole(p.metrics, p.Active)
	p.mu.Unlock()
	pm.mu.Lock()
	pm.globalInflight -= bytes
	if pm.globalInflight < 0 {
		pm.globalInflight = 0
	}
	pm.mu.Unlock()
}

// MarkSuccess is deprecated; use MarkAcked.
func (pm *PathManager) MarkSuccess(key string, rtt time.Duration, bytes int) {
	pm.MarkAcked(key, rtt, bytes)
}

// MarkLoss records loss on a path.
func (pm *PathManager) MarkLoss(key string) {
	pm.mu.RLock()
	p, ok := pm.paths[key]
	pm.mu.RUnlock()
	if !ok {
		return
	}
	p.mu.Lock()
	p.congestion.OnLoss()
	p.metrics.InFlightBytes = p.congestion.InFlight
	p.metrics.CWND = p.congestion.Window
	p.metrics.recordLoss()
	p.Role = updatePathRole(p.metrics, p.Active)
	p.mu.Unlock()
}

// MarkTimeout records timeout on a path.
func (pm *PathManager) MarkTimeout(key string) {
	pm.mu.RLock()
	p, ok := pm.paths[key]
	pm.mu.RUnlock()
	if !ok {
		return
	}
	p.mu.Lock()
	p.congestion.OnLoss()
	p.metrics.InFlightBytes = p.congestion.InFlight
	p.metrics.CWND = p.congestion.Window
	p.metrics.recordTimeout()
	p.Role = updatePathRole(p.metrics, p.Active)
	p.mu.Unlock()
}

// PathsByRole returns path keys grouped by role for scheduling.
func (pm *PathManager) PathsByRole() map[PathRole][]string {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	out := make(map[PathRole][]string)
	for k, p := range pm.paths {
		p.mu.RLock()
		if p.metrics.Disabled {
			p.mu.RUnlock()
			continue
		}
		role := p.Role
		if role == 0 {
			role = updatePathRole(p.metrics, p.Active)
		}
		p.mu.RUnlock()
		out[role] = append(out[role], k)
	}
	return out
}

// CanSend reports whether a path can accept more bytes in flight.
func (pm *PathManager) CanSend(key string, nbytes int) bool {
	pm.mu.RLock()
	p, ok := pm.paths[key]
	pm.mu.RUnlock()
	if !ok {
		return false
	}
	p.mu.RLock()
	defer p.mu.RUnlock()
	if p.metrics.Disabled {
		return false
	}
	if !p.congestion.CanSend(nbytes) {
		return false
	}
	maxGlobal := pm.cfg.MaxGlobalInflightBytes()
	if maxGlobal > 0 && pm.globalInflight+nbytes > maxGlobal {
		return false
	}
	return true
}

// DisablePath temporarily stops scheduling a path.
func (pm *PathManager) DisablePath(key string, dur time.Duration) {
	p := pm.getOrCreate(key)
	p.mu.Lock()
	p.metrics.Disabled = true
	p.mu.Unlock()
	if dur > 0 {
		go func() {
			time.Sleep(dur)
			p.mu.Lock()
			p.metrics.Disabled = false
			p.mu.Unlock()
		}()
	}
}
