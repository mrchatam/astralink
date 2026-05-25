package client

import (
	"encoding/binary"
	"time"

	"github.com/astralink/astralink-go/internal/authority"
	Enums "github.com/astralink/astralink-go/internal/enums"
	"github.com/astralink/astralink-go/internal/transport"
	VpnProto "github.com/astralink/astralink-go/internal/vpnproto"
)

// initTransportRuntime wires the multipath QUIC-over-DNS core.
func (c *Client) initTransportRuntime() {
	if c == nil {
		return
	}
	cfg := transport.ConfigFromClient(c.cfg)
	c.transport = transport.NewRuntime(cfg, c.log)
	c.initAuthorityBootstrap()
	c.syncTransportPaths()
	if c.balancer != nil {
		c.balancer.SetResolverTimeoutHandler(func(serverKey string, dnsID uint16) {
			streamID := uint16(0)
			if rec, ok := c.transport.LookupSend(serverKey, dnsID); ok {
				streamID = rec.StreamID
			}
			c.transportReportTimedOut(serverKey, dnsID, streamID)
		})
	}
}

func (c *Client) initAuthorityBootstrap() {
	if c == nil {
		return
	}
	endpoints := authority.ClientAuthorityEndpoints(c.cfg.AuthorityEndpoints)
	mode := authority.ModeSingle
	if len(endpoints) > 1 {
		mode = authority.ModeMulti
	}
	c.authorityBootstrap = authority.BuildBootstrap(mode, endpoints, false)
	if node, ok := authority.SelectClientAuthority(endpoints); ok && c.log != nil {
		c.log.Infof("astralink authority bootstrap primary=%s fallbacks=%d", node.Address, len(c.authorityBootstrap.Fallbacks))
	}
}

func (c *Client) syncTransportPaths() {
	if c == nil || c.transport == nil || c.balancer == nil {
		return
	}
	active := make([]string, 0, c.cfg.MaxActivePaths)
	standby := make([]string, 0, c.cfg.MaxStandbyPaths)
	for _, conn := range c.balancer.ActiveConnections() {
		if len(active) < c.cfg.MaxActivePaths {
			active = append(active, conn.Key)
		}
		c.syncTransportPathStats(conn)
	}
	for _, conn := range c.balancer.InactiveConnections() {
		if len(standby) < c.cfg.MaxStandbyPaths {
			standby = append(standby, conn.Key)
		}
		c.syncTransportPathStats(conn)
	}
	c.transport.SyncPaths(active, standby)
}

func (c *Client) syncTransportPathStats(conn Connection) {
	if c == nil || c.transport == nil {
		return
	}
	st := transport.PathStats{Key: conn.Key, RTT: conn.MTUResolveTime}
	if rtt, loss, timeout := c.balancer.PathTelemetry(conn.Key); rtt > 0 || loss > 0 {
		if rtt > 0 {
			st.RTT = rtt
		}
		st.LossRate = loss
		st.TimeoutRate = timeout
	}
	edns := conn.UploadMTUChars
	if edns <= 0 {
		edns = EDnsSafeUDPSize
	}
	c.transport.Bundle().SetEDNSSafeSize(conn.Key, edns)
	c.transport.UpdatePathStats(conn.Key, st)
}

func transportPacketClass(packetType uint8) transport.PacketClass {
	switch packetType {
	case Enums.PACKET_STREAM_SYN, Enums.PACKET_SOCKS5_SYN,
		Enums.PACKET_STREAM_CLOSE_WRITE, Enums.PACKET_STREAM_CLOSE_READ, Enums.PACKET_STREAM_RST:
		return transport.ClassHandshake
	case Enums.PACKET_STREAM_DATA_ACK, Enums.PACKET_STREAM_DATA_NACK, Enums.PACKET_PACKED_CONTROL_BLOCKS:
		return transport.ClassControl
	default:
		return transport.ClassData
	}
}

func dnsIDFromPacket(packet []byte) uint16 {
	if len(packet) < 2 {
		return 0
	}
	return binary.BigEndian.Uint16(packet[:2])
}

func (c *Client) transportPlanRequest(task plannerTask) transport.PlanRequest {
	payloadLen := len(task.opts.Payload)
	if payloadLen < 1 {
		payloadLen = 64
	}
	return transport.PlanRequest{
		Class:       transportPacketClass(task.opts.PacketType),
		StreamID:    task.opts.StreamID,
		SequenceNum: task.opts.SequenceNum,
		PayloadLen:  payloadLen,
		DupBudget:   c.cfg.MaxActivePaths,
	}
}

// ensureTransportPlan computes transport plan once per planner task.
func (c *Client) ensureTransportPlan(task *plannerTask) transport.PlanResult {
	if task == nil {
		return transport.PlanResult{DupCount: 1}
	}
	if task.hasTransportPlan {
		return task.transportPlan
	}
	if c == nil || c.transport == nil {
		task.transportPlan = transport.PlanResult{DupCount: max(1, task.dupCount)}
		task.hasTransportPlan = true
		return task.transportPlan
	}
	c.syncTransportPaths()
	task.transportPlan = c.transport.PlanOutbound(c.transportPlanRequest(*task))
	if task.transportPlan.DupCount < 1 {
		task.transportPlan.DupCount = 1
	}
	task.hasTransportPlan = true
	return task.transportPlan
}

// selectTransportConnections chooses send targets via transport PlanOutbound (hot path).
func (c *Client) selectTransportConnections(task plannerTask) ([]Connection, error) {
	if c == nil || c.balancer == nil {
		return nil, ErrNoValidConnections
	}

	plan := c.ensureTransportPlan(&task)

	if c.transport == nil {
		targetCount := task.dupCount
		if targetCount < 1 {
			targetCount = 1
		}
		return c.balancer.SelectTargets(task.opts.PacketType, task.opts.StreamID, targetCount)
	}

	pathKeys := make([]string, 0, 1+len(plan.Extras))
	if plan.Primary != "" {
		pathKeys = append(pathKeys, plan.Primary)
	}
	for _, k := range plan.Extras {
		if k != "" && k != plan.Primary {
			pathKeys = append(pathKeys, k)
		}
	}

	if len(pathKeys) == 0 {
		c.transport.RecordSchedulerFallback()
		return c.balancer.SelectTargets(task.opts.PacketType, task.opts.StreamID, max(1, plan.DupCount))
	}

	conns := make([]Connection, 0, len(pathKeys))
	for _, key := range pathKeys {
		if conn, ok := c.balancer.GetConnectionByKey(key); ok && conn.IsValid {
			conns = append(conns, conn)
		}
	}
	if len(conns) == 0 {
		c.transport.RecordSchedulerFallback()
		return c.balancer.SelectTargets(task.opts.PacketType, task.opts.StreamID, len(pathKeys))
	}

	payloadLen := len(task.opts.Payload)
	if payloadLen < 1 {
		payloadLen = 64
	}
	reserveBytes := payloadLen + 128
	allowedKeys := c.transport.ReserveSend(pathKeys, reserveBytes)
	if len(allowedKeys) == 0 {
		return nil, ErrNoValidConnections
	}
	filtered := make([]Connection, 0, len(allowedKeys))
	allowed := make(map[string]struct{}, len(allowedKeys))
	for _, k := range allowedKeys {
		allowed[k] = struct{}{}
	}
	for _, conn := range conns {
		if _, ok := allowed[conn.Key]; ok {
			filtered = append(filtered, conn)
		} else {
			c.transport.ReleaseSend([]string{conn.Key}, reserveBytes)
		}
	}
	if len(filtered) == 0 {
		return nil, ErrNoValidConnections
	}
	return filtered, nil
}

func (c *Client) transportDupCount(task plannerTask) int {
	plan := c.ensureTransportPlan(&task)
	n := plan.DupCount
	if n < 1 {
		n = 1
	}
	return n
}

func (c *Client) transportShouldBundle(task plannerTask) bool {
	return c.ensureTransportPlan(&task).Bundle
}

func (c *Client) transportRecordBundleExecution(blockCount int, bundled bool) {
	if c == nil || c.transport == nil || blockCount < 1 {
		return
	}
	c.transport.RecordBundlePlan(blockCount, bundled)
}

func (c *Client) transportShouldFEC(task plannerTask) bool {
	return c.ensureTransportPlan(&task).UseFEC
}

func (c *Client) transportEDNSSizeForPath(pathKey string) int {
	if c == nil || c.transport == nil {
		return EDnsSafeUDPSize
	}
	limit := c.transport.Bundle().MaxBundleSize(pathKey)
	if limit > EDnsSafeUDPSize {
		return limit
	}
	if limit < 512 {
		return EDnsSafeUDPSize
	}
	return limit
}

func (c *Client) transportWrapPayloadForSend(task plannerTask, encoded []byte, pathKey string) []byte {
	if !c.transportShouldFEC(task) {
		return encoded
	}
	flags := uint8(VpnProto.TransportFlagFEC)
	pathID := uint16(0)
	for i, conn := range c.balancer.ActiveConnections() {
		if conn.Key == pathKey {
			pathID = uint16(i + 1)
			break
		}
	}
	shards := maxInt(2, c.cfg.FECDataShards)
	seqField := VpnProto.FECWireSeq(task.opts.StreamID, task.opts.SequenceNum, shards)
	return VpnProto.PrependTransportHeader(encoded, flags, pathID, seqField, uint8(transportPacketClass(task.opts.PacketType)))
}

func (c *Client) transportMaybeFECParity(task plannerTask, encoded []byte) []byte {
	if c == nil || c.transport == nil || c.transport.FECWindow() == nil || !c.transportShouldFEC(task) {
		return nil
	}
	return c.transport.FECWindow().Push(task.opts.StreamID, task.opts.SequenceNum, encoded)
}

func (c *Client) transportReportSent(pathKey string, packet []byte, bytes int, streamID, seqNum uint16, packetType uint8) {
	if c != nil && c.transport != nil {
		c.transport.OnSent(pathKey, dnsIDFromPacket(packet), bytes, streamID, seqNum, packetType)
	}
}

func (c *Client) transportReportDNSRoundTrip(pathKey string, dnsID uint16, rtt time.Duration) {
	if c != nil && c.transport != nil {
		c.transport.OnDNSRoundTrip(pathKey, dnsID, rtt)
	}
}

func (c *Client) transportReportDNSDelivered(pathKey string, dnsID uint16, rtt time.Duration) {
	if c != nil && c.transport != nil {
		c.transport.OnDNSDelivered(pathKey, dnsID, rtt)
	}
}

func (c *Client) transportConfirmStreamAck(streamID, seq uint16, ackType uint8, rtt time.Duration) {
	if c != nil && c.transport != nil {
		c.transport.OnStreamAcked(streamID, seq, ackType, rtt)
	}
}

func (c *Client) transportReportLost(pathKey string, dnsID uint16, streamID uint16) {
	if c != nil && c.transport != nil {
		c.transport.OnLost(pathKey, dnsID, streamID)
	}
}

func (c *Client) transportReportTimedOut(pathKey string, dnsID uint16, streamID uint16) {
	if c != nil && c.transport != nil {
		c.transport.OnTimedOut(pathKey, dnsID, streamID)
	}
}

func (c *Client) transportReleaseReserved(pathKeys []string, bytes int) {
	if c != nil && c.transport != nil {
		c.transport.ReleaseSend(pathKeys, bytes)
	}
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// preferAuthorityConnections reorders connections to try authority endpoints first.
func (c *Client) preferAuthorityConnections(conns []Connection) []Connection {
	if c == nil || len(c.authorityBootstrap.Primary.Address) == 0 || len(conns) < 2 {
		return conns
	}
	primary := c.authorityBootstrap.Primary.Address
	out := make([]Connection, 0, len(conns))
	rest := make([]Connection, 0, len(conns))
	for _, conn := range conns {
		if conn.Resolver == primary || conn.Domain == primary {
			out = append(out, conn)
		} else {
			rest = append(rest, conn)
		}
	}
	for _, fb := range c.authorityBootstrap.Fallbacks {
		for i, conn := range rest {
			if conn.Resolver == fb.Address || conn.Domain == fb.Address {
				out = append(out, conn)
				rest = append(rest[:i], rest[i+1:]...)
				break
			}
		}
	}
	return append(out, rest...)
}

// authorityFallbackConnections returns next authority-ordered resolver set after failure.
func (c *Client) authorityFallbackConnections(failed Connection) []Connection {
	if c == nil || c.balancer == nil {
		return nil
	}
	active := c.balancer.ActiveConnections()
	if len(c.authorityBootstrap.Fallbacks) == 0 {
		return active
	}
	out := make([]Connection, 0, len(active))
	for _, fb := range c.authorityBootstrap.Fallbacks {
		for _, conn := range active {
			if conn.Key != failed.Key && (conn.Resolver == fb.Address || conn.Domain == fb.Address) {
				out = append(out, conn)
			}
		}
	}
	for _, conn := range active {
		if conn.Key != failed.Key {
			out = append(out, conn)
		}
	}
	return out
}
