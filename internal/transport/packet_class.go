package transport

import Enums "github.com/astralink/astralink-go/internal/enums"

// PacketClassFor maps VPN packet types to transport scheduling class.
func PacketClassFor(packetType uint8) PacketClass {
	switch packetType {
	case Enums.PACKET_STREAM_SYN, Enums.PACKET_SOCKS5_SYN,
		Enums.PACKET_STREAM_CLOSE_WRITE, Enums.PACKET_STREAM_CLOSE_READ, Enums.PACKET_STREAM_RST:
		return ClassHandshake
	case Enums.PACKET_STREAM_DATA_ACK, Enums.PACKET_STREAM_DATA_NACK, Enums.PACKET_PACKED_CONTROL_BLOCKS:
		return ClassControl
	default:
		return ClassData
	}
}

// AwaitsStreamAck reports whether delivery must be confirmed by stream DATA_ACK.
func AwaitsStreamAck(packetType uint8) bool {
	return packetType == Enums.PACKET_STREAM_DATA || packetType == Enums.PACKET_STREAM_RESEND
}
