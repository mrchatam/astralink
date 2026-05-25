package transport

import (
	"sync"
	"time"
)

// SendRecord tracks one outbound DNS exchange awaiting stream-level confirmation.
type SendRecord struct {
	PathKey    string
	DNSID      uint16
	StreamID   uint16
	SeqNum     uint16
	PacketType uint8
	Bytes      int
	SentAt     time.Time
}

// SendRegistry maps pending sends for stream ACK correlation and multipath duplicates.
type SendRegistry struct {
	mu       sync.Mutex
	byDNS    map[PacketKey]*SendRecord
	byLogical map[uint64][]*SendRecord
}

// NewSendRegistry creates a send registry.
func NewSendRegistry() *SendRegistry {
	return &SendRegistry{
		byDNS:     make(map[PacketKey]*SendRecord),
		byLogical: make(map[uint64][]*SendRecord),
	}
}

func logicalSendKey(streamID, seq uint16) uint64 {
	return (uint64(streamID) << 16) | uint64(seq)
}

// RegisterSend records metadata for a sent packet (supports duplicate paths per logical send).
func (r *SendRegistry) RegisterSend(rec SendRecord) {
	if r == nil || rec.PathKey == "" {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	cp := rec
	pk := PacketKey{PathKey: rec.PathKey, DNSID: rec.DNSID}
	r.byDNS[pk] = &cp
	if rec.StreamID != 0 {
		lk := logicalSendKey(rec.StreamID, rec.SeqNum)
		r.byLogical[lk] = append(r.byLogical[lk], &cp)
	}
	if len(r.byDNS) > 8192 {
		r.evictLocked()
	}
}

// RemoveByDNS removes a pending send record after terminal outcome.
func (r *SendRegistry) RemoveByDNS(pathKey string, dnsID uint16) (SendRecord, bool) {
	if r == nil || pathKey == "" {
		return SendRecord{}, false
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	pk := PacketKey{PathKey: pathKey, DNSID: dnsID}
	rec, ok := r.byDNS[pk]
	if !ok || rec == nil {
		return SendRecord{}, false
	}
	delete(r.byDNS, pk)
	r.removeFromLogicalLocked(rec)
	return *rec, true
}

// LookupByDNS returns a send record for timeout/loss release.
func (r *SendRegistry) LookupByDNS(pathKey string, dnsID uint16) (SendRecord, bool) {
	if r == nil || pathKey == "" {
		return SendRecord{}, false
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	rec, ok := r.byDNS[PacketKey{PathKey: pathKey, DNSID: dnsID}]
	if !ok || rec == nil {
		return SendRecord{}, false
	}
	return *rec, true
}

// ConfirmStreamAck finds all sends for stream+seq acknowledged by stream ACK and removes them.
func (r *SendRegistry) ConfirmStreamAck(streamID, seq uint16, ackType uint8, match func(sendType, ackType uint8) bool) ([]SendRecord, bool) {
	if r == nil || streamID == 0 {
		return nil, false
	}
	if match == nil {
		return nil, false
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	lk := logicalSendKey(streamID, seq)
	group := r.byLogical[lk]
	if len(group) == 0 {
		return nil, false
	}
	out := make([]SendRecord, 0, len(group))
	remaining := group[:0]
	for _, rec := range group {
		if rec == nil || !match(rec.PacketType, ackType) {
			remaining = append(remaining, rec)
			continue
		}
		out = append(out, *rec)
		delete(r.byDNS, PacketKey{PathKey: rec.PathKey, DNSID: rec.DNSID})
	}
	if len(remaining) == 0 {
		delete(r.byLogical, lk)
	} else {
		r.byLogical[lk] = remaining
	}
	if len(out) == 0 {
		return nil, false
	}
	return out, true
}

func (r *SendRegistry) evictLocked() {
	for k := range r.byDNS {
		rec := r.byDNS[k]
		delete(r.byDNS, k)
		if rec != nil {
			r.removeFromLogicalLocked(rec)
		}
		if len(r.byDNS) <= 4096 {
			break
		}
	}
}

func (r *SendRegistry) removeFromLogicalLocked(rec *SendRecord) {
	if rec == nil || rec.StreamID == 0 {
		return
	}
	lk := logicalSendKey(rec.StreamID, rec.SeqNum)
	group := r.byLogical[lk]
	filtered := group[:0]
	for _, item := range group {
		if item != rec {
			filtered = append(filtered, item)
		}
	}
	if len(filtered) == 0 {
		delete(r.byLogical, lk)
	} else {
		r.byLogical[lk] = filtered
	}
}
