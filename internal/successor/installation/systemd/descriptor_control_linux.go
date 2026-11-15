package systemd

import "context"

// DescriptorJob records only whether the finite fixed-unit request was attempted
// and its canonical native reply. It proves neither a process nor completion;
// an attempted request with an error may already have affected the manager.
type DescriptorJob struct {
	Attempted bool
	Object    string
}

// Start and Stop use this role's original, fixed-unit control lifetime.
func (s *DescriptorControl) Start() (DescriptorJob, error) {
	if s == nil || s.original == nil {
		return DescriptorJob{}, ErrInput
	}
	job, err := s.original.fixedUnitControl.start()
	return DescriptorJob{Attempted: job.Attempted, Object: job.Object}, err
}

func (s *DescriptorControl) Stop(ctx context.Context) (DescriptorJob, error) {
	if s == nil || s.original == nil {
		return DescriptorJob{}, ErrInput
	}
	job, err := s.original.fixedUnitControl.stop(ctx)
	return DescriptorJob{Attempted: job.Attempted, Object: job.Object}, err
}
