package udpserver

import (
	"sync"

	Enums "github.com/astralink/astralink-go/internal/enums"
	"github.com/astralink/astralink-go/internal/security"
	"github.com/astralink/astralink-go/internal/transport"
	VpnProto "github.com/astralink/astralink-go/internal/vpnproto"
)

const maxServerFECGroups = 512

type fecGroupBucket struct {
	shards map[uint16][]byte
	parity []byte
}

type serverFECState struct {
	mu      sync.Mutex
	groups  map[uint64]*fecGroupBucket
	fecWin  *transport.FECWindow
	shards  int
}

func sessionFECGroupKey(sessionID uint8, groupID uint32) uint64 {
	return (uint64(sessionID) << 32) | uint64(groupID)
}

func (s *Server) serverFEC() *serverFECState {
	if s == nil {
		return nil
	}
	if s.fecInbound == nil {
		shards := 4
		if s.cfg.TransportFECDataShards >= 2 {
			shards = s.cfg.TransportFECDataShards
		}
		s.fecInbound = &serverFECState{
			groups: make(map[uint64]*fecGroupBucket),
			fecWin: transport.NewFECWindow(transport.Config{
				FECEnabled:    s.cfg.TransportFECEnabled,
				FECDataShards: shards,
			}),
			shards: shards,
		}
	}
	return s.fecInbound
}

func (st *serverFECState) trimLocked() {
	if len(st.groups) <= maxServerFECGroups {
		return
	}
	for k := range st.groups {
		delete(st.groups, k)
		if len(st.groups) <= maxServerFECGroups/2 {
			break
		}
	}
}

func fecGroupFromPacket(pkt VpnProto.Packet, shards int) (groupID uint32, shardIdx uint16, inner []byte) {
	if _, _, seq, _, _, rest, ok := VpnProto.ParseTransportPrefix(pkt.Payload); ok {
		return VpnProto.FECGroupKey(seq), VpnProto.FECShardIndex(seq), rest
	}
	seq := VpnProto.FECWireSeq(pkt.StreamID, pkt.SequenceNum, shards)
	return VpnProto.FECGroupKey(seq), VpnProto.FECShardIndex(seq), pkt.Payload
}

// parseInflatedFromTunnelLabels decodes labels and strips optional transport prefix.
func parseInflatedFromTunnelLabels(labels string, codec *security.Codec) (VpnProto.Packet, bool, error) {
	if codec == nil {
		return VpnProto.Packet{}, false, VpnProto.ErrCodecUnavailable
	}
	raw, err := codec.DecodeStringAndDecrypt(labels)
	if err != nil {
		return VpnProto.Packet{}, false, err
	}
	tunnelBytes, hadTransport := stripTransportPrefixBytes(raw)
	packet, err := VpnProto.Parse(tunnelBytes)
	if err != nil {
		return VpnProto.Packet{}, hadTransport, err
	}
	packet, err = VpnProto.InflatePayload(packet)
	if err != nil {
		return VpnProto.Packet{}, hadTransport, err
	}
	return packet, hadTransport, nil
}

func stripTransportPrefixBytes(raw []byte) ([]byte, bool) {
	if len(raw) < VpnProto.TransportHeaderSize {
		return raw, false
	}
	if raw[0] != VpnProto.TransportVersion {
		return raw, false
	}
	_, _, _, _, _, rest, ok := VpnProto.ParseTransportPrefix(raw)
	if !ok {
		return raw, false
	}
	return rest, true
}

func (s *Server) absorbServerParity(vpnPacket VpnProto.Packet) bool {
	if s == nil || !s.cfg.TransportFECEnabled {
		return false
	}
	if vpnPacket.PacketType != Enums.PACKET_STREAM_DATA && vpnPacket.PacketType != Enums.PACKET_STREAM_RESEND {
		return false
	}
	st := s.serverFEC()
	if st == nil {
		return false
	}
	groupID, shardIdx, inner := fecGroupFromPacket(vpnPacket, st.shards)
	if !transport.IsParityPayload(inner) {
		return false
	}
	key := sessionFECGroupKey(vpnPacket.SessionID, groupID)
	st.mu.Lock()
	bucket := st.groups[key]
	if bucket == nil {
		bucket = &fecGroupBucket{shards: make(map[uint16][]byte)}
		st.groups[key] = bucket
	}
	cp := make([]byte, len(inner))
	copy(cp, inner)
	bucket.parity = cp
	_ = shardIdx
	st.trimLocked()
	st.mu.Unlock()
	return true
}

func (s *Server) tryServerFECRecovery(vpnPacket VpnProto.Packet) (VpnProto.Packet, bool) {
	if s == nil || !s.cfg.TransportFECEnabled {
		return vpnPacket, false
	}
	st := s.serverFEC()
	if st == nil || st.fecWin == nil {
		return vpnPacket, false
	}
	groupID, shardIdx, inner := fecGroupFromPacket(vpnPacket, st.shards)
	if transport.IsParityPayload(inner) {
		return vpnPacket, false
	}
	key := sessionFECGroupKey(vpnPacket.SessionID, groupID)
	st.mu.Lock()
	bucket := st.groups[key]
	if bucket == nil {
		bucket = &fecGroupBucket{shards: make(map[uint16][]byte)}
		st.groups[key] = bucket
	}
	cp := make([]byte, len(inner))
	copy(cp, inner)
	bucket.shards[shardIdx] = cp
	parity := bucket.parity
	shards := make(map[uint16][]byte, len(bucket.shards))
	for k, p := range bucket.shards {
		shards[k] = p
	}
	st.trimLocked()
	st.mu.Unlock()
	if parity == nil {
		return vpnPacket, false
	}
	missing := serverFECMissingShard(shards, st.shards)
	if missing < 0 {
		return vpnPacket, false
	}
	recovered := st.fecWin.TryRecover(parity, shards, uint16(missing))
	if recovered == nil {
		return vpnPacket, false
	}
	out := vpnPacket
	out.Payload = recovered
	return out, true
}

func serverFECMissingShard(shards map[uint16][]byte, total int) int {
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
