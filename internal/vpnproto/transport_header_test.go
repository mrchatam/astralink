package vpnproto

import "testing"

func TestFECWireSeqRoundTrip(t *testing.T) {
	seq := FECWireSeq(9, 5, 4)
	if FECGroupKey(seq) != FECGroupKey(FECWireSeq(9, 4, 4)) {
		t.Fatal("same stream/group should share group key")
	}
	if FECShardIndex(seq) != 5%4 {
		t.Fatalf("shard=%d", FECShardIndex(seq))
	}
}
