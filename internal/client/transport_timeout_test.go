package client

import (
	"testing"
	"time"

	"github.com/astralink/astralink-go/internal/transport"
	VpnProto "github.com/astralink/astralink-go/internal/vpnproto"
)

func TestBalancerTimeoutPassesDNSID(t *testing.T) {
	b := NewBalancer(BalancingRoundRobin, nil)
	b.SetAutoDisableConfig(true, 30*time.Second)
	var gotKey string
	var gotDNS uint16
	b.SetResolverTimeoutHandler(func(serverKey string, dnsID uint16) {
		gotKey = serverKey
		gotDNS = dnsID
	})
	packet := []byte{0x00, 0x2A, 0x01, 0x00}
	b.TrackResolverSend(packet, "127.0.0.1:53", "", "resolver-a", time.Now().Add(-60*time.Second), 5*time.Second)
	b.CollectExpiredResolverTimeouts(time.Now(), 5*time.Second)
	if gotKey != "resolver-a" || gotDNS != 42 {
		t.Fatalf("timeout handler got key=%q dns=%d", gotKey, gotDNS)
	}
}

func TestTransportTimeoutOnRealPendingKey(t *testing.T) {
	cfg := defaultTestClientConfig()
	c := &Client{cfg: cfg, balancer: NewBalancer(BalancingRoundRobin, nil)}
	c.initTransportRuntime()
	c.transport.SyncPaths([]string{"r1"}, nil)
	c.transport.OnSent("r1", 77, 150, 2, 3, 0x0F)
	c.transportReportTimedOut("r1", 77, 2)
	c.transport.OnTimedOut("r1", 77, 2)
	if c.transport.OnStreamAcked(2, 3, 0x10, time.Millisecond) {
		t.Fatal("ack after timeout should fail")
	}
}

func TestSingleTransportPlanPerTask(t *testing.T) {
	cfg := defaultTestClientConfig()
	c := &Client{cfg: cfg, balancer: NewBalancer(BalancingRoundRobin, nil)}
	c.initTransportRuntime()
	c.transport.SyncPaths([]string{"r1", "r2"}, nil)
	task := plannerTask{
		opts: VpnProto.BuildOptions{PacketType: 0x0F, StreamID: 1, SequenceNum: 2},
	}
	p1 := c.ensureTransportPlan(&task)
	p2 := c.ensureTransportPlan(&task)
	if p1.Primary != p2.Primary || p1.DupCount != p2.DupCount {
		t.Fatalf("plan drift: %+v vs %+v", p1, p2)
	}
	if !task.hasTransportPlan {
		t.Fatal("expected cached plan")
	}
	_ = transport.PlanResult{}
}
