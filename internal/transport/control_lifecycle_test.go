package transport

import (
	"testing"
	"time"

	Enums "github.com/astralink/astralink-go/internal/enums"
)

func TestControlDNSDeliveredTerminal(t *testing.T) {
	cfg := DefaultAdvancedConfig()
	rt := NewRuntime(cfg, nil)
	rt.SyncPaths([]string{"p1"}, nil)

	rt.OnSent("p1", 10, 96, 0, 0, Enums.PACKET_PING)
	if !rt.OnDNSDelivered("p1", 10, 12*time.Millisecond) {
		t.Fatal("expected control DNS delivery to terminal ack")
	}
	st := rt.TelemetrySnapshot()["p1"]
	if st.RTT != 12*time.Millisecond {
		t.Fatalf("expected RTT after control delivery: %+v", st)
	}
	if rt.OnDNSDelivered("p1", 10, time.Millisecond) {
		t.Fatal("duplicate delivery must not double-ack")
	}
	rt.OnTimedOut("p1", 10, 0)
	if _, ok := rt.LookupSend("p1", 10); ok {
		t.Fatal("terminal control must not leave registry entry for timeout")
	}
}

func TestDataWaitsForStreamAckNotDNSDelivered(t *testing.T) {
	cfg := DefaultAdvancedConfig()
	rt := NewRuntime(cfg, nil)
	rt.SyncPaths([]string{"p1"}, nil)
	rt.OnSent("p1", 20, 120, 5, 3, Enums.PACKET_STREAM_DATA)
	if rt.OnDNSDelivered("p1", 20, 5*time.Millisecond) {
		t.Fatal("data must not terminal on DNS delivery alone")
	}
	if !rt.OnStreamAcked(5, 3, Enums.PACKET_STREAM_DATA_ACK, 8*time.Millisecond) {
		t.Fatal("expected stream ack terminal")
	}
}

func TestTimeoutClearsRegistryEntry(t *testing.T) {
	cfg := DefaultAdvancedConfig()
	rt := NewRuntime(cfg, nil)
	rt.SyncPaths([]string{"a"}, nil)
	rt.OnSent("a", 1, 64, 9, 2, Enums.PACKET_STREAM_DATA)
	rt.OnTimedOut("a", 1, 0)
	if _, ok := rt.LookupSend("a", 1); ok {
		t.Fatal("timeout must remove registry entry")
	}
}
