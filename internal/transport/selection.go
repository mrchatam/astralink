package transport

// SelectPaths returns ordered path keys for transmission (primary first, then extras).
func (r *Runtime) SelectPaths(req ScheduleRequest) []string {
	if r == nil {
		return nil
	}
	if r.streams != nil {
		r.sched.SetStreamRoutes(r.streams)
	}
	res := r.sched.Schedule(req)
	if res.Primary == "" {
		return nil
	}
	out := make([]string, 0, 1+len(res.Extras))
	out = append(out, res.Primary)
	for _, k := range res.Extras {
		if k != "" && k != res.Primary {
			out = append(out, k)
		}
	}
	maxDup := r.cfg.MaxDuplication
	if maxDup < 1 {
		maxDup = 8
	}
	if len(out) > maxDup {
		out = out[:maxDup]
	}
	return out
}

// ShouldBundle reports whether the scheduler wants bundling for this request.
func (r *Runtime) ShouldBundle(req ScheduleRequest) bool {
	if r == nil {
		return false
	}
	return r.sched.Schedule(req).Bundle
}

// ReserveSend attempts to reserve inflight budget on all target paths.
func (r *Runtime) ReserveSend(pathKeys []string, bytesPerPath int) []string {
	if r == nil || bytesPerPath <= 0 {
		return pathKeys
	}
	allowed := make([]string, 0, len(pathKeys))
	for _, k := range pathKeys {
		if r.paths.ReserveInflight(k, bytesPerPath) {
			allowed = append(allowed, k)
		}
	}
	return allowed
}

// ReleaseSend releases inflight reservation when send aborted.
func (r *Runtime) ReleaseSend(pathKeys []string, bytesPerPath int) {
	if r == nil {
		return
	}
	for _, k := range pathKeys {
		r.paths.ReleaseInflight(k, bytesPerPath)
	}
}
