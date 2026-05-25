package udpserver

import (
	"testing"

	"github.com/astralink/astralink-go/internal/config"
	VpnProto "github.com/astralink/astralink-go/internal/vpnproto"
)

func TestStripTransportPrefixBytes(t *testing.T) {
	raw := VpnProto.PrependTransportHeader([]byte{0x01, 0x02, 0x03}, VpnProto.TransportFlagFEC, 2, 9, 2)
	rest, ok := stripTransportPrefixBytes(raw)
	if !ok {
		t.Fatal("expected transport prefix")
	}
	if len(rest) != 3 {
		t.Fatalf("rest len=%d", len(rest))
	}
}

func TestAbsorbServerParity(t *testing.T) {
	s := &Server{cfg: defaultTestServerConfig()}
	pkt := VpnProto.Packet{
		PacketType: 0x0F,
		StreamID:   1,
		SequenceNum: 2,
		Payload:    []byte{0xAF, 0xEC, 0x01},
	}
	if !s.absorbServerParity(pkt) {
		t.Fatal("expected parity absorb")
	}
}

func defaultTestServerConfig() config.ServerConfig {
	return config.ServerConfig{
		TransportAware:      true,
		TransportFECEnabled: true,
	}
}
