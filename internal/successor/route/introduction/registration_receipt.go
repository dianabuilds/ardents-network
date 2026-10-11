package introduction

import (
	"errors"
	"time"
)

// RegistrationFacts are detached public facts of the actual REGISTER. They
// grant neither a live registration nor Descriptor acknowledgement/readiness.
// Created precedes REGISTER output; ACK delay never moves the refresh origin.
type RegistrationFacts struct {
	Network, Profile, Node, Slot [32]byte
	Revision                     uint64
	Created, Expiry              time.Time
	Acknowledgement              [32]byte
}

// Receipt retains the immutable original channel identity. Only a completed
// genuine REGISTER can produce one. Copies do not prolong its lifetime.
type Receipt struct {
	owner *HolderRegistration
	facts RegistrationFacts
}

func (receipt Receipt) Facts() RegistrationFacts { return receipt.facts }

// Receipt checks the original channel before handing out its public binding.
// The consumer must CheckReceipt again at its own effects and handoffs.
func (r *HolderRegistration) Receipt() (Receipt, error) {
	if r == nil {
		return Receipt{}, errors.New("route registration unavailable")
	}
	receipt := r.receipt
	if err := r.CheckReceipt(receipt); err != nil {
		return Receipt{}, err
	}
	return receipt, nil
}

// CheckReceipt reobserves the genuine original Prefix/duty/caller and then
// synchronously checks withdrawal, retirement and expiry. No domain lock is
// held during Network observation. A detached Facts value cannot pass it.
func (r *HolderRegistration) CheckReceipt(receipt Receipt) error {
	if r == nil || receipt.owner != r || receipt != r.receipt || r.check == nil || r.ctx == nil {
		return errors.New("route registration receipt differs")
	}
	r.mu.Lock()
	unavailable := r.stopped || r.withdrawing
	r.mu.Unlock()
	if unavailable {
		return errors.New("route registration unavailable")
	}
	if err := r.check(); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.stopped || r.withdrawing || !time.Now().Before(receipt.facts.Expiry) {
		return errors.New("route registration unavailable")
	}
	return r.ctx.Err()
}
