package daemon

const (
	BootIdle     = "idle"
	BootReady    = "ready"
	BootDegraded = "degraded"
)

type BootResult struct {
	Joined bool
	State  string
	Reason string
}

type BootStatus struct {
	sources []string
	res     BootResult
}

func newBootStatus(sources []string) *BootStatus {
	return &BootStatus{sources: append([]string(nil), sources...)}
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
	out := make([]string, len(s.sources))
	copy(out, s.sources)
	return out
}

func (s *BootStatus) Result() BootResult {
	return s.res
}
