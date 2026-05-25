package transport

import (
	"testing"

	Enums "github.com/astralink/astralink-go/internal/enums"
)

func TestSendRegistryStreamAckCorrelation(t *testing.T) {
	r := NewSendRegistry()
	r.RegisterSend(SendRecord{
		PathKey: "p1", DNSID: 99, StreamID: 7, SeqNum: 3, PacketType: Enums.PACKET_STREAM_DATA, Bytes: 100,
	})
	recs, ok := r.ConfirmStreamAck(7, 3, Enums.PACKET_STREAM_DATA_ACK, streamDataAckMatch)
	if !ok || len(recs) != 1 || recs[0].PathKey != "p1" || recs[0].DNSID != 99 {
		t.Fatalf("confirm failed ok=%v recs=%+v", ok, recs)
	}
}

func TestSendRegistryMultipathGroupAck(t *testing.T) {
	r := NewSendRegistry()
	r.RegisterSend(SendRecord{PathKey: "p1", DNSID: 10, StreamID: 5, SeqNum: 9, PacketType: Enums.PACKET_STREAM_DATA, Bytes: 80})
	r.RegisterSend(SendRecord{PathKey: "p2", DNSID: 11, StreamID: 5, SeqNum: 9, PacketType: Enums.PACKET_STREAM_DATA, Bytes: 80})
	recs, ok := r.ConfirmStreamAck(5, 9, Enums.PACKET_STREAM_DATA_ACK, streamDataAckMatch)
	if !ok || len(recs) != 2 {
		t.Fatalf("expected group ack for both paths, ok=%v recs=%+v", ok, recs)
	}
	if _, ok := r.LookupByDNS("p1", 10); ok {
		t.Fatal("p1 should be cleared")
	}
	if _, ok := r.LookupByDNS("p2", 11); ok {
		t.Fatal("p2 should be cleared")
	}
}
