package transport

import (
	"testing"
	"time"
)

func TestSchedulerSimpleModeStandby(t *testing.T) {
	cfg := DefaultSimpleConfig()
	pm := NewPathManager(cfg)
	pm.Upsert("a", true)
	pm.Upsert("b", false)
	sched := NewScheduler(cfg, pm)
	res := sched.Schedule(ScheduleRequest{Class: ClassControl, PayloadLen: 64})
	if res.Primary != "a" {
		t.Fatalf("primary=%q want a", res.Primary)
	}
	if len(res.Extras) == 0 {
		t.Fatal("expected standby extra in simple mode")
	}
}

func TestSchedulerDuplicationCap(t *testing.T) {
	cfg := DefaultAdvancedConfig()
	cfg.DataDuplication = 4
	cfg.MaxDuplication = 2
	pm := NewPathManager(cfg)
	pm.Upsert("a", true)
	pm.Upsert("b", true)
	pm.Upsert("c", true)
	sched := NewScheduler(cfg, pm)
	pm.SetStats("a", PathStats{Key: "a", LossRate: 0.9})
	pm.SetStats("b", PathStats{Key: "b", LossRate: 0.9})
	res := sched.Schedule(ScheduleRequest{Class: ClassData, PayloadLen: 128})
	total := 1 + len(res.Extras)
	if total > cfg.MaxDuplication {
		t.Fatalf("dup total=%d max=%d", total, cfg.MaxDuplication)
	}
}

func TestRuntimeSelectPathsAndReserve(t *testing.T) {
	cfg := DefaultAdvancedConfig()
	rt := NewRuntime(cfg, nil)
	rt.SyncPaths([]string{"p1", "p2"}, nil)
	rt.UpdatePathStats("p1", PathStats{Key: "p1", RTT: 10 * time.Millisecond})
	rt.UpdatePathStats("p2", PathStats{Key: "p2", RTT: 50 * time.Millisecond})
	keys := rt.SelectPaths(ScheduleRequest{Class: ClassControl, PayloadLen: 64})
	if len(keys) == 0 || keys[0] != "p1" {
		t.Fatalf("keys=%v", keys)
	}
	allowed := rt.ReserveSend(keys, 512)
	if len(allowed) == 0 {
		t.Fatal("expected reserve success")
	}
	rt.OnSendSuccess(allowed[0], 5*time.Millisecond, 512)
}

func TestFECRecover(t *testing.T) {
	fec := NewFECEngine(Config{FECEnabled: true, FECDataShards: 2, FECParityShards: 1})
	shards := [][]byte{[]byte("hello"), []byte("world")}
	parity := fec.EncodeGroup(shards)
	missing := fec.RecoverShard([][]byte{shards[0], nil}, 1, parity)
	if string(missing) != "world" {
		t.Fatalf("recovered=%q", missing)
	}
}

func TestFECWindowParityMarker(t *testing.T) {
	win := NewFECWindow(Config{FECEnabled: true, FECDataShards: 2})
	if p := win.Push(1, 0, []byte("aa")); p != nil {
		t.Fatalf("unexpected early parity=%v", p)
	}
	p := win.Push(1, 1, []byte("bb"))
	if p == nil || !IsParityPayload(p) {
		t.Fatalf("parity=%v", p)
	}
}

func TestBundlePlanner(t *testing.T) {
	b := NewBundlePlanner(Config{MaxBundleBytes: 100})
	b.SetEDNSSafeSize("p1", 200)
	if !b.CanBundle("p1", []int{40, 40}) {
		t.Fatal("expected bundle allowed")
	}
	if !b.CanBundle("p1", []int{30, 30}) {
		t.Fatal("expected two-frame bundle")
	}
}

func TestCongestionWindow(t *testing.T) {
	c := NewCongestionState("adaptive")
	if !c.CanSend(1024) {
		t.Fatal("expected initial send allowed")
	}
	c.Reserve(2048)
	c.OnLoss()
	if c.Window >= 32768 {
		t.Fatal("expected window reduction on loss")
	}
	c.Release(1024)
	if c.InFlight < 0 {
		t.Fatal("negative inflight after release")
	}
}

func TestPathManagerInflightGlobalCap(t *testing.T) {
	cfg := DefaultSimpleConfig()
	cfg.MaxGlobalInflight = 1000
	pm := NewPathManager(cfg)
	pm.Upsert("a", true)
	if !pm.ReserveInflight("a", 800) {
		t.Fatal("first reserve failed")
	}
	if pm.ReserveInflight("a", 400) {
		t.Fatal("expected global cap to block second reserve")
	}
	pm.ReleaseInflight("a", 800)
}
