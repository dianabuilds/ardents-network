package infrastructure

// ServingPlan returns detached public declaration bytes under original input
// admission. It grants no State authority and contains no private credential.
func (s SourceInputs) ServingPlan() ([]byte, error) {
	p := s.original
	if p == nil {
		return nil, ErrInput
	}
	if err := p.begin(); err != nil {
		return nil, err
	}
	defer p.uses.Done()
	p.ioMu.Lock()
	defer p.ioMu.Unlock()
	if err := p.local(); err != nil {
		return nil, err
	}
	if len(p.files) < 2 || p.files[1].path != p.request.Plan {
		return nil, ErrInput
	}
	return append([]byte(nil), p.files[1].raw...), nil
}
