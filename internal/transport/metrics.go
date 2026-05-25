package transport

import (
	"sync"
	"time"
)

// PathMetrics tracks live path telemetry updated from runtime events.
type PathMetrics struct {
	Key string

	mu sync.RWMutex

	RTT           time.Duration
	LossRate      float64
	TimeoutRate   float64
	InFlightBytes int
	CWND          int
	LastSuccess   time.Time
	LastActive    time.Time
	Disabled      bool

	Sent      uint64
	Acked     uint64
	Lost      uint64
	Timeouts  uint64
	Throughput float64 // bytes/sec EMA
}

func (m *PathMetrics) snapshot() PathStats {
	if m == nil {
		return PathStats{}
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	return PathStats{
		Key:           m.Key,
		RTT:           m.RTT,
		LossRate:      m.LossRate,
		TimeoutRate:   m.TimeoutRate,
		InFlightBytes: m.InFlightBytes,
		CWND:          m.CWND,
		LastSuccess:   m.LastSuccess,
		Disabled:      m.Disabled,
	}
}

func (m *PathMetrics) recordSent(bytes int) {
	if m == nil {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Sent++
	m.LastActive = time.Now()
	_ = bytes
}

func (m *PathMetrics) recordAcked(rtt time.Duration, bytes int) {
	if m == nil {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Acked++
	m.LastSuccess = time.Now()
	m.LastActive = time.Now()
	if rtt > 0 {
		if m.RTT == 0 {
			m.RTT = rtt
		} else {
			m.RTT = (m.RTT*7 + rtt) / 8
		}
		// throughput EMA
		if rtt > 0 {
			inst := float64(bytes) / rtt.Seconds()
			if m.Throughput == 0 {
				m.Throughput = inst
			} else {
				m.Throughput = m.Throughput*0.8 + inst*0.2
			}
		}
	}
	m.recomputeRatesLocked()
}

// recordSuccess is an alias for recordAcked.
func (m *PathMetrics) recordSuccess(rtt time.Duration, bytes int) {
	m.recordAcked(rtt, bytes)
}

func (m *PathMetrics) recordLoss() {
	if m == nil {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Lost++
	m.recomputeRatesLocked()
}

func (m *PathMetrics) recordTimeout() {
	if m == nil {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Timeouts++
	m.recomputeRatesLocked()
}

func (m *PathMetrics) recomputeRatesLocked() {
	if m.Sent > 0 {
		m.LossRate = float64(m.Lost) / float64(m.Sent)
		m.TimeoutRate = float64(m.Timeouts) / float64(m.Sent)
	}
}
