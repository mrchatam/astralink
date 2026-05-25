package transport

import "github.com/astralink/astralink-go/internal/config"

// ConfigFromClient maps client configuration to transport runtime settings.
func ConfigFromClient(cfg config.ClientConfig) Config {
	mode := ModeSimple
	if cfg.TransportMode == "advanced" {
		mode = ModeAdvanced
	}
	var base Config
	if mode == ModeAdvanced {
		base = DefaultAdvancedConfig()
	} else {
		base = DefaultSimpleConfig()
	}
	base.Mode = mode
	if cfg.MaxActivePaths > 0 {
		base.MaxActivePaths = cfg.MaxActivePaths
	}
	if cfg.MaxStandbyPaths > 0 {
		base.MaxStandbyPaths = cfg.MaxStandbyPaths
	}
	if cfg.PacketDuplicationCount > 0 {
		base.DataDuplication = cfg.PacketDuplicationCount
	}
	if cfg.SetupPacketDuplicationCount > 0 {
		base.SetupDuplication = cfg.SetupPacketDuplicationCount
	}
	if cfg.FECEnabled {
		base.FECEnabled = true
	}
	if cfg.FECDataShards > 0 {
		base.FECDataShards = cfg.FECDataShards
	}
	if cfg.FECParityShards > 0 {
		base.FECParityShards = cfg.FECParityShards
	}
	if cfg.CongestionProfile != "" {
		base.CongestionProfile = cfg.CongestionProfile
	}
	if cfg.MaxBundleBytes > 0 {
		base.MaxBundleBytes = cfg.MaxBundleBytes
	}
	if cfg.MaxActivePaths > 0 {
		base.MaxDuplication = cfg.MaxActivePaths
	}
	if cfg.PacketDuplicationCount > 0 && cfg.PacketDuplicationCount > base.MaxDuplication {
		base.MaxDuplication = cfg.PacketDuplicationCount
	}
	return base
}
