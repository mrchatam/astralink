package transport

import (
	"testing"

	VpnProto "github.com/astralink/astralink-go/internal/vpnproto"
)

func TestFECWindowGroupIsolation(t *testing.T) {
	win := NewFECWindow(Config{FECEnabled: true, FECDataShards: 2})
	g1 := VpnProto.FECGroupKey(VpnProto.FECWireSeq(1, 0, 2))
	g2 := VpnProto.FECGroupKey(VpnProto.FECWireSeq(1, 1, 2))
	_ = win.PushGroup(g1, 0, []byte("a0"))
	p := win.PushGroup(g1, 1, []byte("a1"))
	if p == nil {
		t.Fatal("expected parity for complete group g1")
	}
	_ = win.PushGroup(g2, 0, []byte("b0"))
	if win.PushGroup(g2, 1, []byte("b1")) == nil {
		t.Fatal("expected parity for group g2")
	}
}

func TestFECWindowTryRecoverMissingShard(t *testing.T) {
	win := NewFECWindow(Config{FECEnabled: true, FECDataShards: 2})
	g := VpnProto.FECGroupKey(VpnProto.FECWireSeq(2, 0, 2))
	_ = win.PushGroup(g, 0, []byte("hello"))
	parity := win.PushGroup(g, 1, []byte("world"))
	if parity == nil {
		t.Fatal("expected parity from push")
	}
	shards := map[uint16][]byte{0: []byte("hello")}
	got := win.TryRecover(parity, shards, 1)
	if string(got) != "world" {
		t.Fatalf("recovered=%q", got)
	}
}
