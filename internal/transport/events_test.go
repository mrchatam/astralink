package transport

import "testing"

func TestPendingTrackerSingleTerminalOutcome(t *testing.T) {
	tk := NewPendingTracker()
	if !tk.MarkSent("p1", 42) {
		t.Fatal("expected first sent")
	}
	if !tk.TryAck("p1", 42) {
		t.Fatal("expected ack")
	}
	if tk.TryAck("p1", 42) {
		t.Fatal("duplicate ack should be rejected")
	}
	if tk.TryLost("p1", 42) {
		t.Fatal("lost after ack should be rejected")
	}
}

func TestOnSentVsOnAckedSemantics(t *testing.T) {
	cfg := DefaultAdvancedConfig()
	rt := NewRuntime(cfg, nil)
	rt.SyncPaths([]string{"a"}, nil)
	rt.OnSent("a", 1, 512, 10, 5, 0x0F)
	st, ok := rt.paths.Stats("a")
	if !ok || st.RTT != 0 {
		t.Fatalf("sent should not set rtt: %+v", st)
	}
	if st.CWND >= 32768 {
		t.Fatal("sent should not grow cwnd to max")
	}
	rt.OnAcked("a", 1, 20*1e6, 512, 10)
	st, ok = rt.paths.Stats("a")
	if !ok || st.RTT == 0 {
		t.Fatal("ack should set rtt")
	}
}
