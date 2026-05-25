package transport

// BundlePlanner groups frames into EDNS-safe DNS exchanges.
type BundlePlanner struct {
	maxBytes int
	cache    map[string]int // path key -> edns safe size
}

// NewBundlePlanner creates a bundling planner.
func NewBundlePlanner(cfg Config) *BundlePlanner {
	return &BundlePlanner{
		maxBytes: cfg.MaxBundleBytes,
		cache:    make(map[string]int),
	}
}

// SetEDNSSafeSize records per-path EDNS capability.
func (b *BundlePlanner) SetEDNSSafeSize(pathKey string, size int) {
	if b == nil || size <= 0 {
		return
	}
	if b.cache == nil {
		b.cache = make(map[string]int)
	}
	b.cache[pathKey] = size
}

// MaxBundleSize returns allowed bundle bytes for a path.
func (b *BundlePlanner) MaxBundleSize(pathKey string) int {
	if b == nil {
		return 512
	}
	if sz, ok := b.cache[pathKey]; ok && sz > 0 {
		limit := sz - 128 // header overhead reserve
		if limit < 256 {
			limit = 256
		}
		if limit < b.maxBytes {
			return limit
		}
	}
	return b.maxBytes
}

// CanBundle reports whether frames fit in one DNS exchange.
func (b *BundlePlanner) CanBundle(pathKey string, frameSizes []int) bool {
	if b == nil || len(frameSizes) == 0 {
		return false
	}
	limit := b.MaxBundleSize(pathKey)
	total := 0
	for _, n := range frameSizes {
		total += n
		if total > limit {
			return false
		}
	}
	return len(frameSizes) > 1
}

// PlanBundle returns indices of frames to bundle together.
func (b *BundlePlanner) PlanBundle(pathKey string, frameSizes []int) []int {
	if !b.CanBundle(pathKey, frameSizes) {
		if len(frameSizes) > 0 {
			return []int{0}
		}
		return nil
	}
	limit := b.MaxBundleSize(pathKey)
	indices := make([]int, 0, len(frameSizes))
	total := 0
	for i, n := range frameSizes {
		if total+n > limit && len(indices) > 0 {
			break
		}
		indices = append(indices, i)
		total += n
	}
	return indices
}
