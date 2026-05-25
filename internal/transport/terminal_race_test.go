package transport

import (
	"sync"
	"testing"
	"time"

	Enums "github.com/astralink/astralink-go/internal/enums"
)

func TestConcurrentAckVsTimeoutSingleTerminal(t *testing.T) {
	cfg := DefaultAdvancedConfig()
	rt := NewRuntime(cfg, nil)
	rt.SyncPaths([]string{"p1"}, nil)
	rt.OnSent("p1", 42, 200, 3, 7, Enums.PACKET_STREAM_DATA)

	var wg sync.WaitGroup
	wg.Add(2)
	var acked, timedOut bool
	go func() {
		defer wg.Done()
		acked = rt.OnStreamAcked(3, 7, Enums.PACKET_STREAM_DATA_ACK, time.Millisecond)
	}()
	go func() {
		defer wg.Done()
		rt.OnTimedOut("p1", 42, 3)
		timedOut = true
	}()
	wg.Wait()
	if !acked && !timedOut {
		t.Fatal("expected one terminal outcome")
	}
	if rt.OnStreamAcked(3, 7, Enums.PACKET_STREAM_DATA_ACK, time.Millisecond) {
		t.Fatal("duplicate stream ack after race")
	}
	rt.OnTimedOut("p1", 42, 3)
	if rt.OnStreamAcked(3, 7, Enums.PACKET_STREAM_DATA_ACK, time.Millisecond) {
		t.Fatal("stream ack after terminal timeout should fail")
	}
}

func TestTimeoutReleasesInflight(t *testing.T) {
	cfg := DefaultAdvancedConfig()
	rt := NewRuntime(cfg, nil)
	rt.SyncPaths([]string{"p1"}, nil)
	reserve := 400
	if !rt.paths.ReserveInflight("p1", reserve) {
		t.Fatal("reserve")
	}
	rt.OnSent("p1", 55, reserve, 1, 1, Enums.PACKET_STREAM_DATA)
	rt.OnTimedOut("p1", 55, 1)
	st, ok := rt.paths.Stats("p1")
	if !ok {
		t.Fatal("stats")
	}
	if st.InFlightBytes > reserve/2 {
		t.Fatalf("inflight not released after timeout: %d", st.InFlightBytes)
	}
}
