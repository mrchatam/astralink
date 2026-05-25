package transport

import (
	"sync"
	"sync/atomic"
	"time"

	Enums "github.com/astralink/astralink-go/internal/enums"
	"github.com/astralink/astralink-go/internal/logger"
)

// Runtime is the multipath QUIC-over-DNS transport core.
type Runtime struct {
	cfg       Config
	log       *logger.Logger
	paths     *PathManager
	sched     *Scheduler
	fec       *FECEngine
	bundle    *BundlePlanner
	fecWin    *FECWindow
	streams   *StreamRouteTable
	pending   *PendingTracker
	sends     *SendRegistry
	mu        sync.RWMutex
	telemetry map[string]PathStats
	metrics   TransportMetrics
}

// NewRuntime creates the transport core.
func NewRuntime(cfg Config, log *logger.Logger) *Runtime {
	pm := NewPathManager(cfg)
	sched := NewScheduler(cfg, pm)
	streams := NewStreamRouteTable()
	sched.SetStreamRoutes(streams)
	return &Runtime{
		cfg:       cfg,
		log:       log,
		paths:     pm,
		sched:     sched,
		fec:       NewFECEngine(cfg),
		bundle:    NewBundlePlanner(cfg),
		fecWin:    NewFECWindow(cfg),
		streams:   streams,
		pending:   NewPendingTracker(),
		sends:     NewSendRegistry(),
		telemetry: make(map[string]PathStats),
	}
}

// Config returns runtime configuration.
func (r *Runtime) Config() Config {
	return r.cfg
}

// Metrics returns transport counters.
func (r *Runtime) Metrics() TransportMetrics {
	if r == nil {
		return TransportMetrics{}
	}
	return TransportMetrics{
		FECRecoveryAttempted: atomic.LoadUint64(&r.metrics.FECRecoveryAttempted),
		FECRecoverySuccess:   atomic.LoadUint64(&r.metrics.FECRecoverySuccess),
		FECFallbackARQ:       atomic.LoadUint64(&r.metrics.FECFallbackARQ),
		BundledFrames:        atomic.LoadUint64(&r.metrics.BundledFrames),
		BundlePlans:          atomic.LoadUint64(&r.metrics.BundlePlans),
		BundleFallbacks:      atomic.LoadUint64(&r.metrics.BundleFallbacks),
		TinyExchangesAvoided: atomic.LoadUint64(&r.metrics.TinyExchangesAvoided),
		SchedulerFallbacks:   atomic.LoadUint64(&r.metrics.SchedulerFallbacks),
	}
}

// SyncPaths updates path manager from resolver keys.
func (r *Runtime) SyncPaths(activeKeys, standbyKeys []string) {
	for _, k := range activeKeys {
		r.paths.Upsert(k, true)
	}
	for _, k := range standbyKeys {
		r.paths.Upsert(k, false)
	}
}

// UpdatePathStats ingests external telemetry without counting as ack.
func (r *Runtime) UpdatePathStats(key string, stats PathStats) {
	r.paths.SetStats(key, stats)
	r.syncTelemetry(key)
}

// Schedule selects paths for outbound traffic (legacy API).
func (r *Runtime) Schedule(req ScheduleRequest) ScheduleResult {
	res := r.sched.Schedule(req)
	res.UseFEC = r.shouldUseFEC(req)
	return res
}

// OnSent records packet left the client (after successful UDP write).
func (r *Runtime) OnSent(pathKey string, dnsID uint16, bytes int, streamID uint16, seqNum uint16, packetType uint8) {
	if r == nil || pathKey == "" {
		return
	}
	if !r.pending.MarkSent(pathKey, dnsID) {
		return
	}
	r.paths.MarkSent(pathKey, bytes)
	if r.sends != nil {
		r.sends.RegisterSend(SendRecord{
			PathKey:    pathKey,
			DNSID:      dnsID,
			StreamID:   streamID,
			SeqNum:     seqNum,
			PacketType: packetType,
			Bytes:      bytes,
			SentAt:     time.Now(),
		})
	}
	r.syncTelemetry(pathKey)
}

// OnSentLegacy keeps backward compatibility without stream metadata.
func (r *Runtime) OnSentLegacy(pathKey string, dnsID uint16, bytes int, streamID uint16) {
	r.OnSent(pathKey, dnsID, bytes, streamID, 0, 0)
}

// OnStreamAcked confirms delivery using stream/seq correlation to all duplicate sends.
func (r *Runtime) OnStreamAcked(streamID, seq uint16, packetType uint8, rtt time.Duration) bool {
	if r == nil || r.sends == nil {
		return false
	}
	recs, ok := r.sends.ConfirmStreamAck(streamID, seq, packetType, streamDataAckMatch)
	if !ok || len(recs) == 0 {
		return false
	}
	ackedAny := false
	stickPath := recs[0].PathKey
	for _, rec := range recs {
		if !r.pending.TryAck(rec.PathKey, rec.DNSID) {
			continue
		}
		ackRTT := rtt
		if ackRTT <= 0 && !rec.SentAt.IsZero() {
			ackRTT = time.Since(rec.SentAt)
		}
		r.paths.MarkAcked(rec.PathKey, ackRTT, rec.Bytes)
		ackedAny = true
	}
	if ackedAny {
		r.streams.NoteOutcome(streamID, stickPath, true, false)
		r.syncTelemetry(stickPath)
	}
	return ackedAny
}

func streamDataAckMatch(sendType, ackType uint8) bool {
	if ackType != Enums.PACKET_STREAM_DATA_ACK {
		return false
	}
	return sendType == Enums.PACKET_STREAM_DATA || sendType == Enums.PACKET_STREAM_RESEND
}

// OnDNSRoundTrip records resolver RTT without confirming stream payload delivery.
func (r *Runtime) OnDNSRoundTrip(pathKey string, dnsID uint16, rtt time.Duration) {
	if r == nil || pathKey == "" || rtt <= 0 {
		return
	}
	_ = dnsID
}

// OnDNSDelivered terminals non-data sends when the DNS exchange succeeded.
func (r *Runtime) OnDNSDelivered(pathKey string, dnsID uint16, rtt time.Duration) bool {
	if r == nil || pathKey == "" {
		return false
	}
	rec, ok := r.sends.LookupByDNS(pathKey, dnsID)
	if !ok {
		return false
	}
	if AwaitsStreamAck(rec.PacketType) {
		return false
	}
	return r.terminalAck(pathKey, dnsID, rec.StreamID, rec.Bytes, rtt)
}

func (r *Runtime) terminalAck(pathKey string, dnsID uint16, streamID uint16, bytes int, rtt time.Duration) bool {
	if !r.pending.TryAck(pathKey, dnsID) {
		return false
	}
	if rtt > 0 || bytes > 0 {
		if rtt <= 0 {
			rtt = time.Millisecond
		}
		r.paths.MarkAcked(pathKey, rtt, bytes)
	}
	if streamID != 0 {
		r.streams.NoteOutcome(streamID, pathKey, true, false)
	}
	r.sends.RemoveByDNS(pathKey, dnsID)
	r.syncTelemetry(pathKey)
	return true
}

// OnAcked is deprecated for data-path stickiness; prefer OnStreamAcked.
func (r *Runtime) OnAcked(pathKey string, dnsID uint16, rtt time.Duration, bytes int, streamID uint16) {
	if r == nil || pathKey == "" {
		return
	}
	if !r.pending.TryAck(pathKey, dnsID) {
		return
	}
	r.paths.MarkAcked(pathKey, rtt, bytes)
	_ = streamID
	r.syncTelemetry(pathKey)
}

// OnLost records explicit loss (reject, RCODE failure, NACK).
func (r *Runtime) OnLost(pathKey string, dnsID uint16, streamID uint16) {
	if r == nil || pathKey == "" {
		return
	}
	rec, ok := r.sends.LookupByDNS(pathKey, dnsID)
	bytes, sid := rec.Bytes, streamID
	if ok && sid == 0 {
		sid = rec.StreamID
	}
	if !r.pending.TryLost(pathKey, dnsID) {
		return
	}
	if bytes > 0 {
		r.paths.ReleaseInflight(pathKey, bytes)
	}
	r.paths.MarkLoss(pathKey)
	if sid != 0 {
		r.streams.NoteOutcome(sid, pathKey, false, false)
	}
	r.sends.RemoveByDNS(pathKey, dnsID)
	r.sched.PromoteStandby(pathKey)
	r.syncTelemetry(pathKey)
}

// OnTimedOut records no response within deadline.
func (r *Runtime) OnTimedOut(pathKey string, dnsID uint16, streamID uint16) {
	if r == nil || pathKey == "" {
		return
	}
	rec, ok := r.sends.LookupByDNS(pathKey, dnsID)
	bytes, sid := rec.Bytes, streamID
	if ok && sid == 0 {
		sid = rec.StreamID
	}
	if !r.pending.TryTimeout(pathKey, dnsID) {
		return
	}
	if bytes > 0 {
		r.paths.ReleaseInflight(pathKey, bytes)
	}
	r.paths.MarkTimeout(pathKey)
	if sid != 0 {
		r.streams.NoteOutcome(sid, pathKey, false, true)
	}
	r.sends.RemoveByDNS(pathKey, dnsID)
	r.sched.PromoteStandby(pathKey)
	r.syncTelemetry(pathKey)
}

// LookupSend returns metadata for a pending DNS exchange.
func (r *Runtime) LookupSend(pathKey string, dnsID uint16) (SendRecord, bool) {
	if r == nil || r.sends == nil {
		return SendRecord{}, false
	}
	return r.sends.LookupByDNS(pathKey, dnsID)
}

// RecordSchedulerFallback increments when path selection falls back to balancer.
func (r *Runtime) RecordSchedulerFallback() {
	if r == nil {
		return
	}
	atomic.AddUint64(&r.metrics.SchedulerFallbacks, 1)
}

// OnSendSuccess is deprecated; maps to OnAcked when dnsID unknown (no dedup).
func (r *Runtime) OnSendSuccess(pathKey string, rtt time.Duration, bytes int) {
	if dnsID := uint16(0); dnsID == 0 && rtt == 0 {
		if rtt <= 0 {
			r.paths.MarkSent(pathKey, bytes)
			r.syncTelemetry(pathKey)
			return
		}
	}
	r.OnAcked(pathKey, 0, rtt, bytes, 0)
}

// OnSendLoss is deprecated; use OnLost.
func (r *Runtime) OnSendLoss(pathKey string) {
	r.OnLost(pathKey, 0, 0)
}

// OnSendTimeout is deprecated; use OnTimedOut.
func (r *Runtime) OnSendTimeout(pathKey string) {
	r.OnTimedOut(pathKey, 0, 0)
}

func (r *Runtime) syncTelemetry(pathKey string) {
	r.mu.Lock()
	if st, ok := r.paths.Stats(pathKey); ok {
		r.telemetry[pathKey] = st
	}
	r.mu.Unlock()
}

// RecordFECRecoveryAttempt increments FEC recovery counters.
func (r *Runtime) RecordFECRecoveryAttempt(success bool) {
	if r == nil {
		return
	}
	atomic.AddUint64(&r.metrics.FECRecoveryAttempted, 1)
	if success {
		atomic.AddUint64(&r.metrics.FECRecoverySuccess, 1)
	} else {
		atomic.AddUint64(&r.metrics.FECFallbackARQ, 1)
	}
}

// RecordBundlePlan records bundling planner usage.
func (r *Runtime) RecordBundlePlan(frameCount int, bundled bool) {
	if r == nil {
		return
	}
	atomic.AddUint64(&r.metrics.BundlePlans, 1)
	if bundled && frameCount > 1 {
		atomic.AddUint64(&r.metrics.BundledFrames, uint64(frameCount))
		atomic.AddUint64(&r.metrics.TinyExchangesAvoided, uint64(frameCount-1))
	} else if !bundled {
		atomic.AddUint64(&r.metrics.BundleFallbacks, 1)
	}
}

// FEC returns the FEC engine.
func (r *Runtime) FEC() *FECEngine {
	return r.fec
}

// Bundle returns the bundle planner.
func (r *Runtime) Bundle() *BundlePlanner {
	return r.bundle
}

// FECWindow returns the FEC window tracker.
func (r *Runtime) FECWindow() *FECWindow {
	return r.fecWin
}

// StreamRoutes exposes stream stickiness table.
func (r *Runtime) StreamRoutes() *StreamRouteTable {
	return r.streams
}

// TelemetrySnapshot returns per-path stats for observability.
func (r *Runtime) TelemetrySnapshot() map[string]PathStats {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make(map[string]PathStats, len(r.telemetry))
	for k, v := range r.telemetry {
		out[k] = v
	}
	return out
}

// LogTelemetry emits channel stats when logger is available.
func (r *Runtime) LogTelemetry() {
	if r.log == nil {
		return
	}
	m := r.Metrics()
	r.log.Infof("astralink_transport fec_ok=%d fec_try=%d bundle_frames=%d tiny_avoided=%d",
		m.FECRecoverySuccess, m.FECRecoveryAttempted, m.BundledFrames, m.TinyExchangesAvoided)
	for k, st := range r.TelemetrySnapshot() {
		r.log.Infof("astralink_path key=%s rtt_ms=%d loss=%.3f cwnd=%d", k, st.RTT.Milliseconds(), st.LossRate, st.CWND)
	}
}
