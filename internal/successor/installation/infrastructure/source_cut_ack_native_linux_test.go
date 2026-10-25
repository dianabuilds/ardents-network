//go:build linux && amd64 && network_signing_native

package infrastructure

import (
	"errors"
	"syscall"
)

// Go may resume a SIGURG-interrupted preemption prologue at function entry.
// The semantic ACK counter retains original signal/caller/image provenance.
type sourceCutACKObserver struct {
	previous    map[uint64]sourceCutGoEntry
	interrupted map[uint64]sourceCutGoEntry
}

func (o *sourceCutACKObserver) interrupt(tid int, registers syscall.PtraceRegs, journal string) error {
	prior, ok := o.previous[registers.R14]
	delete(o.interrupted, registers.R14)
	if !ok {
		return nil
	}
	current, err := readSourceCutGoEntry(tid, registers, journal)
	if err != nil {
		return err
	}
	if prior.same(current) {
		if o.interrupted == nil {
			o.interrupted = make(map[uint64]sourceCutGoEntry)
		}
		o.interrupted[registers.R14] = current
	}
	return nil
}

func (o *sourceCutACKObserver) enter(tid int, registers syscall.PtraceRegs, journal string) (bool, error) {
	current, err := readSourceCutGoEntry(tid, registers, journal)
	if err != nil {
		return false, err
	}
	prior, interrupted := o.interrupted[registers.R14]
	delete(o.interrupted, registers.R14)
	if interrupted && prior.same(current) {
		return true, nil
	}
	if o.previous == nil {
		o.previous = make(map[uint64]sourceCutGoEntry)
	}
	if _, exists := o.previous[registers.R14]; !exists && len(o.previous) >= 256 {
		return false, errors.New("original ACK caller bound exceeded")
	}
	o.previous[registers.R14] = current
	return false, nil
}
