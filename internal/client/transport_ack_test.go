package client

import (
	"testing"
	"time"

	Enums "github.com/astralink/astralink-go/internal/enums"
	"github.com/astralink/astralink-go/internal/transport"
)

func TestTransportConfirmStreamAckUpdatesPath(t *testing.T) {
	cfg := defaultTestClientConfig()
	c := &Client{
		cfg:      cfg,
		balancer: NewBalancer(BalancingRoundRobin, nil),
	}
	c.initTransportRuntime()
	c.transport.SyncPaths([]string{"r1"}, nil)
	c.transportReportSent("r1", []byte{0, 1}, 200, 5, 10, Enums.PACKET_STREAM_DATA)
	if !c.transport.OnStreamAcked(5, 10, Enums.PACKET_STREAM_DATA_ACK, 5*time.Millisecond) {
		t.Fatal("expected stream ack confirmation")
	}
	st, ok := c.transport.TelemetrySnapshot()["r1"]
	if !ok || st.RTT == 0 {
		t.Fatalf("expected rtt after stream ack: %+v", st)
	}
}

func TestMultipathStreamAckClearsAllPaths(t *testing.T) {
	cfg := defaultTestClientConfig()
	cfg.PacketDuplicationCount = 2
	c := &Client{cfg: cfg, balancer: NewBalancer(BalancingRoundRobin, nil)}
	c.initTransportRuntime()
	c.transport.SyncPaths([]string{"p1", "p2"}, nil)
	c.transport.OnSent("p1", 1, 100, 4, 8, Enums.PACKET_STREAM_DATA)
	c.transport.OnSent("p2", 2, 100, 4, 8, Enums.PACKET_STREAM_DATA)
	if !c.transport.OnStreamAcked(4, 8, Enums.PACKET_STREAM_DATA_ACK, time.Millisecond) {
		t.Fatal("expected group ack")
	}
	st1, _ := c.transport.TelemetrySnapshot()["p1"]
	st2, _ := c.transport.TelemetrySnapshot()["p2"]
	if st1.RTT == 0 && st2.RTT == 0 {
		t.Fatal("expected at least one path credited")
	}
}

func TestNoDoubleAckOnPendingTracker(t *testing.T) {
	rt := transport.NewRuntime(transport.DefaultAdvancedConfig(), nil)
	rt.OnSent("a", 7, 100, 1, 2, Enums.PACKET_STREAM_DATA)
	if !rt.OnStreamAcked(1, 2, Enums.PACKET_STREAM_DATA_ACK, time.Millisecond) {
		t.Fatal("first ack")
	}
	if rt.OnStreamAcked(1, 2, Enums.PACKET_STREAM_DATA_ACK, time.Millisecond) {
		t.Fatal("duplicate stream ack should fail")
	}
}
