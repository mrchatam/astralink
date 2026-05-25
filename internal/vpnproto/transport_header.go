package vpnproto

// Transport v1 logical header (prepended inside encrypted payload when flags set).
const (
	TransportVersion = 1
	TransportFlagFEC = 1 << 0
	TransportFlagDup = 1 << 1
	TransportFlagBundle = 1 << 2
)

// TransportHeaderSize is the fixed transport metadata prefix.
const TransportHeaderSize = 11

// BuildTransportPrefix returns transport metadata bytes.
func BuildTransportPrefix(flags uint8, pathID uint16, seq uint32, class uint8, payloadLen uint16) []byte {
	buf := make([]byte, TransportHeaderSize)
	buf[0] = TransportVersion
	buf[1] = flags
	buf[2] = byte(pathID >> 8)
	buf[3] = byte(pathID)
	buf[4] = byte(seq >> 24)
	buf[5] = byte(seq >> 16)
	buf[6] = byte(seq >> 8)
	buf[7] = byte(seq)
	buf[8] = class
	buf[9] = byte(payloadLen >> 8)
	buf[10] = byte(payloadLen)
	return buf
}

// ParseTransportPrefix parses transport metadata; returns false if too short.
func ParseTransportPrefix(payload []byte) (flags uint8, pathID uint16, seq uint32, class uint8, payloadLen uint16, rest []byte, ok bool) {
	if len(payload) < TransportHeaderSize || payload[0] != TransportVersion {
		return 0, 0, 0, 0, 0, payload, false
	}
	flags = payload[1]
	pathID = uint16(payload[2])<<8 | uint16(payload[3])
	seq = uint32(payload[4])<<24 | uint32(payload[5])<<16 | uint32(payload[6])<<8 | uint32(payload[7])
	class = payload[8]
	payloadLen = uint16(payload[9])<<8 | uint16(payload[10])
	return flags, pathID, seq, class, payloadLen, payload[TransportHeaderSize:], true
}

// PrependTransportHeader prefixes payload with transport metadata.
func PrependTransportHeader(payload []byte, flags uint8, pathID uint16, seq uint32, class uint8) []byte {
	prefix := BuildTransportPrefix(flags, pathID, seq, class, uint16(len(payload)))
	out := make([]byte, len(prefix)+len(payload))
	copy(out, prefix)
	copy(out[len(prefix):], payload)
	return out
}

// FECWireSeq encodes stream ID, FEC group index, and shard index into the 32-bit transport seq field.
// Layout: streamID(16) | groupIndex(8) | shardIndex(8).
func FECWireSeq(streamID, seq uint16, dataShards int) uint32 {
	if dataShards < 2 {
		dataShards = 2
	}
	ds := uint16(dataShards)
	return (uint32(streamID) << 16) | (uint32(seq/ds) << 8) | uint32(seq%ds)
}

// FECGroupKey returns the group identity (stream + group index) from a transport seq field.
func FECGroupKey(seq uint32) uint32 {
	return seq & 0xffffff00
}

// FECShardIndex returns shard index from a transport seq field.
func FECShardIndex(seq uint32) uint16 {
	return uint16(seq & 0xff)
}
