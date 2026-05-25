package transport

import (
	"testing"
	"time"
)

func TestSchedulerRTTSkewRanking(t *testing.T) {
	cfg := DefaultAdvancedConfig()
	pm := NewPathManager(cfg)
	pm.Upsert("fast", true)
	pm.Upsert("slow", true)
	pm.SetStats("fast", PathStats{Key: "fast", RTT: 5 * time.Millisecond, LossRate: 0.01})
	pm.SetStats("slow", PathStats{Key: "slow", RTT: 200 * time.Millisecond, LossRate: 0.01})
	sched := NewScheduler(cfg, pm)
	res := sched.Schedule(ScheduleRequest{Class: ClassData, PayloadLen: 200, StreamID: 7})
	if res.Primary != "fast" {
		t.Fatalf("primary=%s want fast", res.Primary)
	}
}

func TestStreamStickiness(t *testing.T) {
	cfg := DefaultAdvancedConfig()
	pm := NewPathManager(cfg)
	pm.Upsert("a", true)
	pm.Upsert("b", true)
	pm.SetStats("a", PathStats{Key: "a", RTT: 50 * time.Millisecond, LossRate: 0.1})
	pm.SetStats("b", PathStats{Key: "b", RTT: 10 * time.Millisecond, LossRate: 0.01})
	streams := NewStreamRouteTable()
	streams.NoteOutcome(9, "a", true, false)
	streams.NoteOutcome(9, "a", true, false)
	sched := NewScheduler(cfg, pm)
	sched.SetStreamRoutes(streams)
	res := sched.Schedule(ScheduleRequest{Class: ClassData, PayloadLen: 100, StreamID: 9})
	if res.Primary != "a" {
		t.Fatalf("sticky primary=%s want a", res.Primary)
	}
}

func TestPathPromotionOnTimeout(t *testing.T) {
	cfg := DefaultSimpleConfig()
	rt := NewRuntime(cfg, nil)
	rt.SyncPaths([]string{"bad"}, []string{"good"})
	rt.sched.PromoteStandby("bad")
	active := rt.paths.ActivePaths()
	foundGood := false
	for _, k := range active {
		if k == "good" {
			foundGood = true
		}
	}
	if !foundGood {
		t.Fatalf("expected standby promotion, active=%v", active)
	}
}

func TestFECRecoveryBeforeRetransmit(t *testing.T) {
	fec := NewFECEngine(Config{FECEnabled: true, FECDataShards: 2, FECParityShards: 1})
	shards := [][]byte{[]byte("payload-a"), []byte("payload-b")}
	parity := fec.EncodeGroup(shards)
	got := fec.RecoverShard([][]byte{shards[0], nil}, 1, parity)
	if string(got) != "payload-b" {
		t.Fatalf("recovered=%q", got)
	}
}

func TestSentVsAckedSeparation(t *testing.T) {
	cfg := DefaultAdvancedConfig()
	rt := NewRuntime(cfg, nil)
	rt.SyncPaths([]string{"p1"}, nil)
	rt.OnSent("p1", 42, 80, 3, 9, 0x0F)
	rt.OnDNSRoundTrip("p1", 42, 25*time.Millisecond)
	st := rt.TelemetrySnapshot()["p1"]
	if st.RTT != 0 {
		t.Fatalf("DNS RTT alone must not set path RTT: %+v", st)
	}
	if !rt.OnStreamAcked(3, 9, 0x10, 12*time.Millisecond) {
		t.Fatal("stream ack expected")
	}
	st = rt.TelemetrySnapshot()["p1"]
	if st.RTT != 12*time.Millisecond {
		t.Fatalf("stream ack should set RTT: %+v", st)
	}
	if rt.OnStreamAcked(3, 9, 0x10, time.Millisecond) {
		t.Fatal("duplicate stream ack")
	}
}

func TestStickinessOnlyOnValidatedEvents(t *testing.T) {
	cfg := DefaultAdvancedConfig()
	rt := NewRuntime(cfg, nil)
	rt.SyncPaths([]string{"a", "b"}, nil)
	rt.streams.NoteOutcome(4, "a", true, false)
	rt.OnDNSRoundTrip("b", 1, 5*time.Millisecond)
	res := rt.sched.Schedule(ScheduleRequest{Class: ClassData, PayloadLen: 64, StreamID: 4})
	if res.Primary != "a" {
		t.Fatalf("DNS RTT must not change stickiness: primary=%s", res.Primary)
	}
	rt.OnStreamAcked(4, 1, 0x10, time.Millisecond)
	res = rt.sched.Schedule(ScheduleRequest{Class: ClassData, PayloadLen: 64, StreamID: 4})
	if res.Primary != "a" {
		t.Fatalf("stream ack should keep sticky path: primary=%s", res.Primary)
	}
}

func TestMultipathFanOutUnderLoss(t *testing.T) {
	cfg := DefaultAdvancedConfig()
	cfg.DataDuplication = 2
	rt := NewRuntime(cfg, nil)
	rt.SyncPaths([]string{"p1", "p2"}, nil)
	rt.UpdatePathStats("p1", PathStats{Key: "p1", LossRate: 0.4})
	rt.UpdatePathStats("p2", PathStats{Key: "p2", LossRate: 0.05})
	plan := rt.PlanOutbound(PlanRequest{Class: ClassData, PayloadLen: 128, DupBudget: 4})
	if plan.DupCount < 2 {
		t.Fatalf("expected duplication under loss, plan=%+v", plan)
	}
	if len(plan.Extras) == 0 {
		t.Fatal("expected extra path")
	}
}
