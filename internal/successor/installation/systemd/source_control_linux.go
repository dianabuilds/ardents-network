package systemd

import "context"

// SourceJob records only whether the finite fixed-unit request was attempted
// and its canonical native reply. It proves neither a process nor completion;
// an attempted request with an error may already have affected the manager.
type SourceJob struct {
	Attempted bool
	Object    string
}

// Start and Stop use this role's original, fixed-unit control lifetime.
func (s *SourceControl) Start() (SourceJob, error) {
	if s == nil || s.original == nil {
		return SourceJob{}, ErrInput
	}
	job, err := s.original.fixedUnitControl.start()
	return SourceJob{Attempted: job.Attempted, Object: job.Object}, err
}

func (s *SourceControl) Stop(ctx context.Context) (SourceJob, error) {
	if s == nil || s.original == nil {
		return SourceJob{}, ErrInput
	}
	job, err := s.original.fixedUnitControl.stop(ctx)
	return SourceJob{Attempted: job.Attempted, Object: job.Object}, err
}
