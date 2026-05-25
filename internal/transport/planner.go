package transport

// PlanRequest is the unified outbound planning input.
type PlanRequest struct {
	Class      PacketClass
	StreamID   uint16
	SequenceNum uint16
	PayloadLen int
	DupBudget  int
}

// PlanResult is the transport-owned outbound decision.
type PlanResult struct {
	Primary   string
	Extras    []string
	Bundle    bool
	UseFEC    bool
	DupCount  int
}

// PlanOutbound returns path selection and policy for one frame.
func (r *Runtime) PlanOutbound(req PlanRequest) PlanResult {
	if r == nil {
		return PlanResult{DupCount: 1}
	}
	schedReq := ScheduleRequest{
		Class:      req.Class,
		PayloadLen: req.PayloadLen,
		DupBudget:  req.DupBudget,
		StreamID:   req.StreamID,
	}
	res := r.sched.Schedule(schedReq)
	out := PlanResult{
		Primary:  res.Primary,
		Extras:   append([]string(nil), res.Extras...),
		Bundle:   res.Bundle,
		UseFEC:   r.shouldUseFEC(schedReq),
		DupCount: 1 + len(res.Extras),
	}
	if out.DupCount < 1 {
		out.DupCount = 1
	}
	return out
}

func (r *Runtime) shouldUseFEC(req ScheduleRequest) bool {
	if r == nil || !r.cfg.FECEnabled || r.fec == nil || !r.fec.Enabled() {
		return false
	}
	if req.Class != ClassData {
		return false
	}
	if r.cfg.Mode == ModeSimple {
		return false
	}
	avgLoss := r.sched.avgActiveLoss()
	return avgLoss >= r.cfg.LossThresholdForDup*0.25
}
