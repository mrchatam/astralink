package transport

// CongestionState implements per-path window control.
type CongestionState struct {
	profile  string
	Window   int
	InFlight int
	minWindow int
	maxWindow int
}

// NewCongestionState creates congestion state for a path.
func NewCongestionState(profile string) *CongestionState {
	minW, maxW := 2048, 32768
	if profile == "conservative" {
		maxW = 16384
	}
	return &CongestionState{
		profile:   profile,
		Window:    minW,
		minWindow: minW,
		maxWindow: maxW,
	}
}

// CanSend returns true if nbytes fit in the congestion window.
func (c *CongestionState) CanSend(nbytes int) bool {
	if c == nil {
		return true
	}
	return c.InFlight+nbytes <= c.Window
}

// OnAck grows window on success.
func (c *CongestionState) OnAck(bytes int) {
	if c == nil {
		return
	}
	c.InFlight -= bytes
	if c.InFlight < 0 {
		c.InFlight = 0
	}
	grow := bytes / 2
	if grow < 256 {
		grow = 256
	}
	c.Window += grow
	if c.Window > c.maxWindow {
		c.Window = c.maxWindow
	}
}

// OnLoss reduces window on loss.
func (c *CongestionState) OnLoss() {
	if c == nil {
		return
	}
	c.Window /= 2
	if c.Window < c.minWindow {
		c.Window = c.minWindow
	}
}

// Reserve marks bytes in flight before send.
func (c *CongestionState) Reserve(bytes int) {
	if c == nil {
		return
	}
	c.InFlight += bytes
}

// Release releases reserved bytes without ACK (send aborted).
func (c *CongestionState) Release(bytes int) {
	if c == nil {
		return
	}
	c.InFlight -= bytes
	if c.InFlight < 0 {
		c.InFlight = 0
	}
}

// GlobalPacingInterval returns a pacing hint from path RTTs (microseconds).
func GlobalPacingInterval(paths []PathStats) int {
	if len(paths) == 0 {
		return 1000
	}
	minRTT := paths[0].RTT
	for _, p := range paths[1:] {
		if p.RTT > 0 && (minRTT == 0 || p.RTT < minRTT) {
			minRTT = p.RTT
		}
	}
	if minRTT <= 0 {
		return 1000
	}
	ms := int(minRTT.Milliseconds())
	if ms < 1 {
		ms = 1
	}
	return ms * 1000
}
