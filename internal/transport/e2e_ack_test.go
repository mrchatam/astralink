package transport

import (
	"testing"
	"time"

	Enums "github.com/astralink/astralink-go/internal/enums"
)

// TestE2EClientServerAckLoop models client send -> server would ack -> client stream ACK.
func TestE2EClientServerAckLoop(t *testing.T) {
	clientRT := NewRuntime(DefaultAdvancedConfig(), nil)
	clientRT.SyncPaths([]string{"resolver-a", "resolver-b"}, nil)

	streamID := uint16(42)
	seq := uint16(7)
	clientRT.OnSent("resolver-a", 100, 200, streamID, seq, Enums.PACKET_STREAM_DATA)
	clientRT.OnSent("resolver-b", 101, 200, streamID, seq, Enums.PACKET_STREAM_DATA)

	if clientRT.OnDNSDelivered("resolver-a", 100, 20*time.Millisecond) {
		t.Fatal("data path must not ack on DNS alone")
	}

	if !clientRT.OnStreamAcked(streamID, seq, Enums.PACKET_STREAM_DATA_ACK, 15*time.Millisecond) {
		t.Fatal("stream ack should clear both multipath sends")
	}
	if _, ok := clientRT.LookupSend("resolver-a", 100); ok {
		t.Fatal("registry must be empty after stream ack")
	}
	if _, ok := clientRT.LookupSend("resolver-b", 101); ok {
		t.Fatal("registry must be empty after stream ack")
	}
}
