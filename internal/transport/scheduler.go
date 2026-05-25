package transport

import (
	"math"
	"sort"
	"sync"
)

// Scheduler performs adaptive multipath selection.
type Scheduler struct {
	cfg     Config
	path    *PathManager
	streams *StreamRouteTable
	mu      sync.Mutex
}

// NewScheduler creates a scheduler bound to a path manager.
func NewScheduler(cfg Config, pm *PathManager) *Scheduler {
	return &Scheduler{
		cfg:     cfg,
		path:    pm,
		streams: NewStreamRouteTable(),
	}
}

// SetStreamRoutes attaches shared stream route table from runtime.
func (s *Scheduler) SetStreamRoutes(t *StreamRouteTable) {
	if s == nil || t == nil {
		return
	}
	s.streams = t
}

// Schedule selects path keys for an outbound request.
func (s *Scheduler) Schedule(req ScheduleRequest) ScheduleResult {
	active := s.path.ActivePaths()
	standby := s.path.StandbyPaths()

	if len(active) == 0 && len(standby) > 0 {
		active = []string{standby[0]}
	}
	if len(active) == 0 {
		return ScheduleResult{}
	}

	ranked := s.rankPaths(active)
	primary := ranked[0]
	if req.StreamID != 0 && req.Class == ClassData && s.streams != nil {
		if sticky := s.streams.PreferredPath(req.StreamID, ranked, s.path); sticky != "" {
			primary = sticky
		}
	}

	dup := s.duplicationCount(req)
	extras := make([]string, 0, dup-1)
	for i := 1; i < len(ranked) && len(extras) < dup-1; i++ {
		if ranked[i] != primary {
			extras = append(extras, ranked[i])
		}
	}

	if s.cfg.Mode == ModeSimple && len(extras) == 0 && len(standby) > 0 {
		for _, sb := range standby {
			if sb != primary {
				extras = append(extras, sb)
				break
			}
		}
	}

	// Bundling applies to control frames only; data uses ARQ frames (legacy packed adapter on client).
	bundle := req.Class == ClassControl && s.cfg.BundleControlOnly

	return ScheduleResult{Primary: primary, Extras: extras, Bundle: bundle}
}

func (s *Scheduler) duplicationCount(req ScheduleRequest) int {
	maxDup := s.cfg.MaxDuplication
	if maxDup < 1 {
		maxDup = 8
	}

	var n int
	switch req.Class {
	case ClassHandshake, ClassControl:
		n = s.cfg.SetupDuplication
		if n < 1 {
			n = 1
		}
	default:
		n = 1
		if s.cfg.DataDuplication > 1 {
			avgLoss := s.avgActiveLoss()
			if s.cfg.Mode == ModeAdvanced {
				if avgLoss >= s.cfg.LossThresholdForDup*0.5 {
					n = s.cfg.DataDuplication
				}
			} else if avgLoss >= s.cfg.LossThresholdForDup {
				n = s.cfg.DataDuplication
			}
		}
	}

	active := len(s.path.ActivePaths())
	if active > 0 && n > active {
		n = active
	}
	if n > maxDup {
		n = maxDup
	}
	if n < 1 {
		n = 1
	}
	return n
}

func (s *Scheduler) avgActiveLoss() float64 {
	active := s.path.ActivePaths()
	if len(active) == 0 {
		return 0
	}
	var sum float64
	for _, k := range active {
		if st, ok := s.path.Stats(k); ok {
			sum += st.LossRate
		}
	}
	return sum / float64(len(active))
}

func (s *Scheduler) rankPaths(keys []string) []string {
	byRole := s.path.PathsByRole()
	roleOrder := []PathRole{PathRolePrimary, PathRoleSecondary, PathRoleStandby, PathRoleProbationary}
	ordered := make([]string, 0, len(keys))
	seen := make(map[string]struct{}, len(keys))
	for _, role := range roleOrder {
		for _, k := range byRole[role] {
			seen[k] = struct{}{}
			for _, want := range keys {
				if want == k {
					ordered = append(ordered, k)
				}
			}
		}
	}
	for _, k := range keys {
		if _, ok := seen[k]; !ok {
			ordered = append(ordered, k)
		}
	}
	if len(ordered) == 0 {
		ordered = keys
	}

	type scored struct {
		key   string
		score float64
	}
	items := make([]scored, 0, len(ordered))
	for _, k := range ordered {
		st, ok := s.path.Stats(k)
		if !ok {
			items = append(items, scored{key: k, score: math.MaxFloat64})
			continue
		}
		rttMs := float64(st.RTT.Milliseconds())
		score := s.cfg.SchedulerLossWeight*st.LossRate +
			s.cfg.SchedulerRTTWeight*rttMs +
			s.cfg.SchedulerTimeoutWeight*st.TimeoutRate
		if !s.path.CanSend(k, 64) {
			score += 1000
		}
		items = append(items, scored{key: k, score: score})
	}
	sort.Slice(items, func(i, j int) bool { return items[i].score < items[j].score })
	out := make([]string, len(items))
	for i := range items {
		out[i] = items[i].key
	}
	return out
}

// PromoteStandby activates a standby path when primary degrades.
func (s *Scheduler) PromoteStandby(failedKey string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	standby := s.path.StandbyPaths()
	for _, k := range standby {
		if k != failedKey {
			s.path.Upsert(k, true)
			break
		}
	}
	s.path.Upsert(failedKey, false)
}
