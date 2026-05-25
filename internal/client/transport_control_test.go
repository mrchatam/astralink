package client

import (
	"testing"
	"time"

	Enums "github.com/astralink/astralink-go/internal/enums"
	"github.com/astralink/astralink-go/internal/transport"
	VpnProto "github.com/astralink/astralink-go/internal/vpnproto"
)

func TestTransportReportDNSDeliveredBridge(t *testing.T) {
	cfg := defaultTestClientConfig()
	c := &Client{cfg: cfg, balancer: NewBalancer(BalancingRoundRobin, nil)}
	c.initTransportRuntime()
	c.transport.SyncPaths([]string{"r1"}, nil)
	c.transport.OnSent("r1", 77, 64, 0, 0, Enums.PACKET_PING)
	c.transportReportDNSDelivered("r1", 77, 8*time.Millisecond)
	st, ok := c.transport.TelemetrySnapshot()["r1"]
	if !ok || st.RTT == 0 {
		t.Fatalf("expected RTT after DNS delivered: %+v", st)
	}
}

func TestTransportSchedulerFallbackMetric(t *testing.T) {
	cfg := defaultTestClientConfig()
	c := &Client{cfg: cfg, balancer: NewBalancer(BalancingRoundRobin, nil)}
	c.balancer.connections = []Connection{
		{Key: "r1", Domain: "t.example.com", Resolver: "1.1.1.1", IsValid: true},
	}
	c.balancer.activeIDs = []int{0}
	c.balancer.indexByKey = map[string]int{"r1": 0}
	c.initTransportRuntime()

	before := c.transport.Metrics().SchedulerFallbacks
	task := plannerTask{
		opts: VpnProto.BuildOptions{
			PacketType: Enums.PACKET_STREAM_DATA,
			StreamID:   1,
		},
		dupCount:           1,
		hasTransportPlan:   true,
		transportPlan:      transport.PlanResult{Primary: "ghost-path", DupCount: 1},
	}
	if _, err := c.selectTransportConnections(task); err != nil {
		t.Fatalf("balancer fallback should return targets: %v", err)
	}
	after := c.transport.Metrics().SchedulerFallbacks
	if after <= before {
		t.Fatalf("expected scheduler fallback increment: before=%d after=%d", before, after)
	}
}
