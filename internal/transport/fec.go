package transport

// FECEngine provides XOR parity grouping for loss recovery.
type FECEngine struct {
	enabled     bool
	dataShards  int
	parityShards int
}

// NewFECEngine creates an FEC engine.
func NewFECEngine(cfg Config) *FECEngine {
	return &FECEngine{
		enabled:      cfg.FECEnabled,
		dataShards:   maxInt(1, cfg.FECDataShards),
		parityShards: maxInt(0, cfg.FECParityShards),
	}
}

// Enabled reports whether FEC is active.
func (f *FECEngine) Enabled() bool {
	return f != nil && f.enabled && f.parityShards > 0
}

// EncodeGroup computes XOR parity over equal-length shards.
func (f *FECEngine) EncodeGroup(shards [][]byte) []byte {
	if !f.Enabled() || len(shards) == 0 {
		return nil
	}
	maxLen := 0
	for _, s := range shards {
		if len(s) > maxLen {
			maxLen = len(s)
		}
	}
	if maxLen == 0 {
		return nil
	}
	parity := make([]byte, maxLen)
	for _, s := range shards {
		for i := 0; i < len(s); i++ {
			parity[i] ^= s[i]
		}
	}
	return parity
}

// RecoverShard attempts to rebuild one missing shard using parity.
func (f *FECEngine) RecoverShard(shards [][]byte, missingIndex int, parity []byte) []byte {
	if !f.Enabled() || missingIndex < 0 || missingIndex >= len(shards) || parity == nil {
		return nil
	}
	out := make([]byte, len(parity))
	copy(out, parity)
	for i, s := range shards {
		if i == missingIndex || s == nil {
			continue
		}
		for j := 0; j < len(s) && j < len(out); j++ {
			out[j] ^= s[j]
		}
	}
	return out
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
