package client

import (
	"testing"

	"github.com/astralink/astralink-go/internal/config"
	Enums "github.com/astralink/astralink-go/internal/enums"
	"github.com/astralink/astralink-go/internal/transport"
	VpnProto "github.com/astralink/astralink-go/internal/vpnproto"
)

func TestSelectTransportConnectionsUsesScheduler(t *testing.T) {
	cfg := defaultTestClientConfig()
	cfg.MaxActivePaths = 2
	c := &Client{
		cfg:      cfg,
		balancer: NewBalancer(BalancingRoundRobin, nil),
	}
	c.balancer.connections = []Connection{
		{Key: "r1", Domain: "t.example.com", Resolver: "1.1.1.1", IsValid: true},
		{Key: "r2", Domain: "t.example.com", Resolver: "8.8.8.8", IsValid: true},
	}
	c.balancer.activeIDs = []int{0, 1}
	c.balancer.indexByKey = map[string]int{"r1": 0, "r2": 1}
	c.initTransportRuntime()

	task := plannerTask{
		opts: VpnProto.BuildOptions{
			PacketType: Enums.PACKET_STREAM_DATA_ACK,
			Payload:    []byte{1, 2, 3},
		},
		dupCount: 2,
	}
	conns, err := c.selectTransportConnections(task)
	if err != nil {
		t.Fatalf("selectTransportConnections: %v", err)
	}
	if len(conns) == 0 {
		t.Fatal("expected scheduler-selected connections")
	}
}

func TestTransportPacketClass(t *testing.T) {
	if transportPacketClass(Enums.PACKET_STREAM_SYN) != transport.ClassHandshake {
		t.Fatal("syn should be handshake class")
	}
	if transportPacketClass(Enums.PACKET_STREAM_DATA) != transport.ClassData {
		t.Fatal("data should be data class")
	}
}

func defaultTestClientConfig() config.ClientConfig {
	return config.ClientConfig{
		MaxActivePaths:              2,
		MaxStandbyPaths:             1,
		PacketDuplicationCount:      1,
		SetupPacketDuplicationCount: 2,
		TransportMode:               "advanced",
	}
}
