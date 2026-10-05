//go:build linux

package transport

import "context"

// A Source keeps this original opening until its physical result has either
// joined or transferred to the separately owned Responder. It grants no ready
// Source identity and cannot publish into a replacement.
type responderOpening struct {
	source *Prefix
	cancel context.CancelFunc
}

func (p *Prefix) beginResponder(cancel context.CancelFunc) (*responderOpening, error) {
	p.registrationMu.Lock()
	defer p.registrationMu.Unlock()
	if err := p.localCurrent(); err != nil {
		return nil, err
	}
	opening := &responderOpening{source: p, cancel: cancel}
	if p.responderSetups == nil {
		p.responderSetups = make(map[*responderOpening]struct{})
	}
	p.responderSetups[opening] = struct{}{}
	p.openings.Add(1)
	return opening, nil
}

func (o *responderOpening) finish() {
	o.source.registrationMu.Lock()
	delete(o.source.responderSetups, o)
	o.source.registrationMu.Unlock()
	o.source.openings.Done()
}
