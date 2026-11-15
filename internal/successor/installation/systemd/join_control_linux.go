package systemd

import "context"

// JoinJob records only whether the finite fixed-unit request was attempted
// and its canonical native reply. It proves neither a process nor completion;
// an attempted request with an error may already have affected the manager.
type JoinJob struct {
	Attempted bool
	Object    string
}

// Start and Stop use this role's original, fixed-unit control lifetime.
func (s *JoinControl) Start() (JoinJob, error) {
	if s == nil || s.original == nil {
		return JoinJob{}, ErrInput
	}
	job, err := s.original.fixedUnitControl.start()
	return JoinJob{Attempted: job.Attempted, Object: job.Object}, err
}

func (s *JoinControl) Stop(ctx context.Context) (JoinJob, error) {
	if s == nil || s.original == nil {
		return JoinJob{}, ErrInput
	}
	job, err := s.original.fixedUnitControl.stop(ctx)
	return JoinJob{Attempted: job.Attempted, Object: job.Object}, err
}
