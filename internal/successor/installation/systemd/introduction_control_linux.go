package systemd

import "context"

// IntroductionJob records only whether the finite fixed-unit request was attempted
// and its canonical native reply. It proves neither a process nor completion;
// an attempted request with an error may already have affected the manager.
type IntroductionJob struct {
	Attempted bool
	Object    string
}

// Start and Stop use this role's original, fixed-unit control lifetime.
func (s *IntroductionControl) Start() (IntroductionJob, error) {
	if s == nil || s.original == nil {
		return IntroductionJob{}, ErrInput
	}
	job, err := s.original.fixedUnitControl.start()
	return IntroductionJob{Attempted: job.Attempted, Object: job.Object}, err
}

func (s *IntroductionControl) Stop(ctx context.Context) (IntroductionJob, error) {
	if s == nil || s.original == nil {
		return IntroductionJob{}, ErrInput
	}
	job, err := s.original.fixedUnitControl.stop(ctx)
	return IntroductionJob{Attempted: job.Attempted, Object: job.Object}, err
}
