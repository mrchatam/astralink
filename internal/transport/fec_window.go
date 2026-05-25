package transport

import (
	"sync"

	VpnProto "github.com/astralink/astralink-go/internal/vpnproto"
)

const (
	FECMagic0 = 0xAF
	FECMagic1 = 0xEC
	maxFECGroups = 256
)

type fecGroupState struct {
	shards map[uint16][]byte
}

// FECWindow tracks coding windows keyed by explicit FEC group identity.
type FECWindow struct {
	enabled bool
	shards  int
	mu      sync.Mutex
	groups  map[uint32]*fecGroupState
}

// NewFECWindow creates a FEC window tracker.
func NewFECWindow(cfg Config) *FECWindow {
	return &FECWindow{
		enabled: cfg.FECEnabled,
		shards:  maxInt(2, cfg.FECDataShards),
		groups:  make(map[uint32]*fecGroupState),
	}
}

// Push adds a shard to its FEC group; returns parity when the group is complete.
func (w *FECWindow) Push(streamID, seq uint16, payload []byte) []byte {
	if w == nil || !w.enabled || len(payload) == 0 {
		return nil
	}
	wireSeq := VpnProto.FECWireSeq(streamID, seq, w.shards)
	return w.PushGroup(VpnProto.FECGroupKey(wireSeq), VpnProto.FECShardIndex(wireSeq), payload)
}

// PushGroup records one shard for groupID at shardIdx.
func (w *FECWindow) PushGroup(groupID uint32, shardIdx uint16, payload []byte) []byte {
	if w == nil || !w.enabled || len(payload) == 0 {
		return nil
	}
	if int(shardIdx) >= w.shards {
		return nil
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	st := w.groups[groupID]
	if st == nil {
		st = &fecGroupState{shards: make(map[uint16][]byte, w.shards)}
		w.groups[groupID] = st
	}
	cp := make([]byte, len(payload))
	copy(cp, payload)
	st.shards[shardIdx] = cp
	if len(st.shards) < w.shards {
		w.trimGroupsLocked()
		return nil
	}
	shardList := make([][]byte, w.shards)
	for i := 0; i < w.shards; i++ {
		p, ok := st.shards[uint16(i)]
		if !ok {
			delete(w.groups, groupID)
			w.trimGroupsLocked()
			return nil
		}
		shardList[i] = p
	}
	delete(w.groups, groupID)
	w.trimGroupsLocked()
	engine := NewFECEngine(Config{FECEnabled: true, FECDataShards: w.shards, FECParityShards: 1})
	parity := engine.EncodeGroup(shardList)
	if parity == nil {
		return nil
	}
	out := make([]byte, 2+len(parity))
	out[0] = FECMagic0
	out[1] = FECMagic1
	copy(out[2:], parity)
	return out
}

// TryRecover attempts XOR recovery for the missing shard index.
func (w *FECWindow) TryRecover(parityPayload []byte, shards map[uint16][]byte, missingIdx uint16) []byte {
	if w == nil || !w.enabled || len(parityPayload) < 2 {
		return nil
	}
	if parityPayload[0] != FECMagic0 || parityPayload[1] != FECMagic1 {
		return nil
	}
	if int(missingIdx) >= w.shards {
		return nil
	}
	parity := parityPayload[2:]
	list := make([][]byte, w.shards)
	for i := 0; i < w.shards; i++ {
		if uint16(i) == missingIdx {
			list[i] = nil
			continue
		}
		p, ok := shards[uint16(i)]
		if !ok {
			return nil
		}
		list[i] = p
	}
	engine := NewFECEngine(Config{FECEnabled: true, FECDataShards: w.shards, FECParityShards: 1})
	return engine.RecoverShard(list, int(missingIdx), parity)
}

// IsParityPayload reports whether payload is a FEC parity frame.
func IsParityPayload(payload []byte) bool {
	return len(payload) >= 2 && payload[0] == FECMagic0 && payload[1] == FECMagic1
}

func (w *FECWindow) trimGroupsLocked() {
	if len(w.groups) <= maxFECGroups {
		return
	}
	for k := range w.groups {
		delete(w.groups, k)
		if len(w.groups) <= maxFECGroups/2 {
			break
		}
	}
}
