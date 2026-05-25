package client

import (
	"sync"

	Enums "github.com/astralink/astralink-go/internal/enums"
	"github.com/astralink/astralink-go/internal/transport"
	VpnProto "github.com/astralink/astralink-go/internal/vpnproto"
)

const maxFECInboundGroups = 512

type fecGroupBucket struct {
	shards map[uint16][]byte
	parity []byte
}

type fecInboundState struct {
	mu      sync.Mutex
	groups  map[uint32]*fecGroupBucket
	shards  int
}

func (c *Client) fecInbound() *fecInboundState {
	if c == nil {
		return nil
	}
	if c.fecInboundCache == nil {
		shards := c.cfg.FECDataShards
		if shards < 2 {
			shards = 2
		}
		c.fecInboundCache = &fecInboundState{
			groups: make(map[uint32]*fecGroupBucket),
			shards: shards,
		}
	}
	return c.fecInboundCache
}

func stripTransportHeader(payload []byte) []byte {
	if _, _, _, _, _, rest, ok := VpnProto.ParseTransportPrefix(payload); ok {
		return rest
	}
	return payload
}

func (st *fecInboundState) trimLocked() {
	if len(st.groups) <= maxFECInboundGroups {
		return
	}
	for k := range st.groups {
		delete(st.groups, k)
		if len(st.groups) <= maxFECInboundGroups/2 {
			break
		}
	}
}

func (st *fecInboundState) groupFor(packet VpnProto.Packet) (uint32, uint16, []byte) {
	inner := stripTransportHeader(packet.Payload)
	if _, _, seq, _, _, _, ok := VpnProto.ParseTransportPrefix(packet.Payload); ok {
		return VpnProto.FECGroupKey(seq), VpnProto.FECShardIndex(seq), inner
	}
	seq := VpnProto.FECWireSeq(packet.StreamID, packet.SequenceNum, st.shards)
	return VpnProto.FECGroupKey(seq), VpnProto.FECShardIndex(seq), inner
}

// transportHandleFECInbound stores parity or attempts recovery before ARQ handling.
func (c *Client) transportHandleFECInbound(packet VpnProto.Packet) (VpnProto.Packet, bool) {
	if c == nil || c.transport == nil || c.transport.FECWindow() == nil {
		return packet, false
	}
	if packet.PacketType != Enums.PACKET_STREAM_DATA && packet.PacketType != Enums.PACKET_STREAM_RESEND {
		return packet, false
	}

	state := c.fecInbound()
	if state == nil {
		return packet, false
	}

	groupID, shardIdx, inner := state.groupFor(packet)

	if transport.IsParityPayload(inner) {
		state.mu.Lock()
		bucket := state.groups[groupID]
		if bucket == nil {
			bucket = &fecGroupBucket{shards: make(map[uint16][]byte)}
			state.groups[groupID] = bucket
		}
		cp := make([]byte, len(inner))
		copy(cp, inner)
		bucket.parity = cp
		state.trimLocked()
		state.mu.Unlock()
		return packet, true
	}

	state.mu.Lock()
	bucket := state.groups[groupID]
	if bucket == nil {
		bucket = &fecGroupBucket{shards: make(map[uint16][]byte)}
		state.groups[groupID] = bucket
	}
	cp := make([]byte, len(inner))
	copy(cp, inner)
	bucket.shards[shardIdx] = cp
	parity := bucket.parity
	shards := make(map[uint16][]byte, len(bucket.shards))
	for k, p := range bucket.shards {
		shards[k] = p
	}
	state.trimLocked()
	state.mu.Unlock()

	if parity == nil {
		return packet, false
	}

	missing := fecMissingShardIndex(shards, state.shards)
	if missing < 0 {
		return packet, false
	}
	recovered := c.transport.FECWindow().TryRecover(parity, shards, uint16(missing))
	if recovered == nil {
		c.transport.RecordFECRecoveryAttempt(false)
		return packet, false
	}
	c.transport.RecordFECRecoveryAttempt(true)
	out := packet
	out.Payload = recovered
	return out, false
}

func fecMissingShardIndex(shards map[uint16][]byte, total int) int {
	if total < 2 {
		total = 2
	}
	for i := 0; i < total; i++ {
		if _, ok := shards[uint16(i)]; !ok {
			return i
		}
	}
	return -1
}
