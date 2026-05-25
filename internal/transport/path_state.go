package transport

// PathRole describes multipath channel role in the transport pool.
type PathRole uint8

const (
	PathRoleProbationary PathRole = iota
	PathRoleStandby
	PathRoleSecondary
	PathRolePrimary
)

func (r PathRole) String() string {
	switch r {
	case PathRolePrimary:
		return "primary"
	case PathRoleSecondary:
		return "secondary"
	case PathRoleStandby:
		return "standby"
	default:
		return "probationary"
	}
}

// updatePathRole adjusts role from live metrics.
func updatePathRole(m *PathMetrics, active bool) PathRole {
	if m == nil {
		if active {
			return PathRoleSecondary
		}
		return PathRoleStandby
	}
	m.mu.RLock()
	sent := m.Sent
	loss := m.LossRate
	timeout := m.TimeoutRate
	disabled := m.Disabled
	m.mu.RUnlock()

	if disabled {
		return PathRoleProbationary
	}
	if !active {
		return PathRoleStandby
	}
	if sent < 4 {
		return PathRoleSecondary
	}
	if loss > 0.35 || timeout > 0.35 {
		return PathRoleProbationary
	}
	if loss < 0.08 && timeout < 0.08 {
		return PathRolePrimary
	}
	return PathRoleSecondary
}
