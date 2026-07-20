package recovery

const (
	BootIdle     = "idle"
	BootReady    = "ready"
	BootDegraded = "degraded"
)

type BootConfig struct {
	Sources []string
	Fail    bool
}

type BootResult struct {
	Joined bool
	State  string
	Reason string
}

type BootStatus struct {
	cfg BootConfig
	res BootResult
}

func NewBootStatus(cfg BootConfig) *BootStatus {
	return &BootStatus{cfg: cfg}
}

func BootResultFromTransport(joined bool, state, reason string) BootResult {
	return BootResult{
		Joined: joined,
		State:  NormalizeBootState(state),
		Reason: reason,
	}
}

func StoppedBootResult() BootResult {
	return BootResult{
		State:  BootIdle,
		Reason: "node stopped",
	}
}

func NormalizeBootState(state string) string {
	switch state {
	case BootReady, BootDegraded:
		return state
	default:
		return BootIdle
	}
}

func (s *BootStatus) SetResult(result BootResult) {
	s.res = BootResultFromTransport(result.Joined, result.State, result.Reason)
}

func (s *BootStatus) Sources() []string {
	out := make([]string, len(s.cfg.Sources))
	copy(out, s.cfg.Sources)
	return out
}

func (s *BootStatus) Result() BootResult {
	return s.res
}
