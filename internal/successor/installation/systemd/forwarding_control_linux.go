package systemd

import "context"

// ForwardingJob records only whether the finite fixed-unit request was attempted
// and its canonical native reply. It proves neither a process nor completion;
// an attempted request with an error may already have affected the manager.
type ForwardingJob struct {
	Attempted bool
	Object    string
}

// Start and Stop use this role's original, fixed-unit control lifetime.
func (s *ForwardingControl) Start() (ForwardingJob, error) {
	if s == nil || s.original == nil {
		return ForwardingJob{}, ErrInput
	}
	job, err := s.original.fixedUnitControl.start()
	return ForwardingJob{Attempted: job.Attempted, Object: job.Object}, err
}

func (s *ForwardingControl) Stop(ctx context.Context) (ForwardingJob, error) {
	if s == nil || s.original == nil {
		return ForwardingJob{}, ErrInput
	}
	job, err := s.original.fixedUnitControl.stop(ctx)
	return ForwardingJob{Attempted: job.Attempted, Object: job.Object}, err
}
