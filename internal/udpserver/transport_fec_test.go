package udpserver

import (
	"testing"

	Enums "github.com/astralink/astralink-go/internal/enums"
	"github.com/astralink/astralink-go/internal/transport"
	VpnProto "github.com/astralink/astralink-go/internal/vpnproto"
)

func TestServerFECParityPath(t *testing.T) {
	s := &Server{cfg: defaultTestServerConfig()}
	win := transport.NewFECWindow(transport.Config{FECEnabled: true, FECDataShards: 2})
	_ = win.Push(2, 0, []byte("shard-a"))
	parity := win.Push(2, 1, []byte("shard-b"))
	if parity == nil {
		t.Fatal("expected parity from FEC window")
	}
	if !s.absorbServerParity(VpnProto.Packet{
		PacketType:  Enums.PACKET_STREAM_DATA,
		StreamID:    2,
		SequenceNum: 0,
		Payload:     parity,
	}) {
		t.Fatal("expected parity absorb")
	}
	pkt := VpnProto.Packet{
		PacketType:  Enums.PACKET_STREAM_DATA,
		StreamID:    2,
		SequenceNum: 1,
		Payload:     []byte("shard-b"),
	}
	_, _ = s.tryServerFECRecovery(pkt)
}
