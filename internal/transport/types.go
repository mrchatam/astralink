package transport

import "time"

// Mode controls simple vs advanced transport behavior.
type Mode string

const (
	ModeSimple   Mode = "simple"
	ModeAdvanced Mode = "advanced"
)

// PacketClass categorizes frames for scheduling and redundancy policy.
type PacketClass uint8

const (
	ClassHandshake PacketClass = iota
	ClassControl
	ClassData
)

// Config holds multipath QUIC-over-DNS runtime settings.
type Config struct {
	Mode                   Mode
	MaxActivePaths         int
	MaxStandbyPaths        int
	MaxGlobalInflight      int
	SetupDuplication       int
	DataDuplication        int
	MaxDuplication         int
	FECEnabled             bool
	FECDataShards          int
	FECParityShards        int
	CongestionProfile      string
	BundleControlOnly      bool
	MaxBundleBytes         int
	LossThresholdForDup    float64
	SchedulerLossWeight    float64
	SchedulerRTTWeight     float64
	SchedulerTimeoutWeight float64
}

// MaxGlobalInflightBytes returns global inflight ceiling.
func (c Config) MaxGlobalInflightBytes() int {
	if c.MaxGlobalInflight <= 0 {
		return 256 * 1024
	}
	return c.MaxGlobalInflight
}

// DefaultSimpleConfig returns safe defaults for one-VPS deployments.
func DefaultSimpleConfig() Config {
	return Config{
		Mode:                  ModeSimple,
		MaxActivePaths:        1,
		MaxStandbyPaths:       1,
		MaxGlobalInflight:     128 * 1024,
		MaxDuplication:        4,
		SetupDuplication:      2,
		DataDuplication:       1,
		FECEnabled:            false,
		CongestionProfile:     "conservative",
		BundleControlOnly:     true,
		MaxBundleBytes:        512,
		LossThresholdForDup:   0.25,
		SchedulerLossWeight:   8,
		SchedulerRTTWeight:    1,
		SchedulerTimeoutWeight: 4,
	}
}

// DefaultAdvancedConfig returns defaults for multipath deployments.
func DefaultAdvancedConfig() Config {
	cfg := DefaultSimpleConfig()
	cfg.Mode = ModeAdvanced
	cfg.MaxActivePaths = 4
	cfg.MaxStandbyPaths = 2
	cfg.MaxGlobalInflight = 512 * 1024
	cfg.MaxDuplication = 8
	cfg.DataDuplication = 2
	cfg.FECEnabled = true
	cfg.FECDataShards = 4
	cfg.FECParityShards = 1
	cfg.CongestionProfile = "adaptive"
	cfg.BundleControlOnly = false
	cfg.MaxBundleBytes = 1200
	return cfg
}

// PathStats holds per-resolver-channel telemetry.
type PathStats struct {
	Key           string
	RTT           time.Duration
	LossRate      float64
	TimeoutRate   float64
	InFlightBytes int
	CWND          int
	LastSuccess   time.Time
	Disabled      bool
}

// ScheduleRequest describes an outbound scheduling decision.
type ScheduleRequest struct {
	Class      PacketClass
	PayloadLen int
	DupBudget  int
	StreamID   uint16
}

// ScheduleResult contains selected path keys for transmission.
type ScheduleResult struct {
	Primary string
	Extras  []string
	Bundle  bool
	UseFEC  bool
}

// TransportMetrics exposes transport counters for observability.
type TransportMetrics struct {
	FECRecoveryAttempted uint64
	FECRecoverySuccess   uint64
	FECFallbackARQ       uint64
	BundledFrames        uint64
	BundlePlans          uint64
	BundleFallbacks      uint64
	TinyExchangesAvoided uint64
	SchedulerFallbacks   uint64
}
