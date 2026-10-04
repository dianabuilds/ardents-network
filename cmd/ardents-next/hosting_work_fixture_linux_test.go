//go:build linux

package main

import (
	"context"
	"errors"
	"github.com/dianabuilds/ardents-network/internal/successor/admission"
	"github.com/dianabuilds/ardents-network/internal/successor/admission/receiving"
	"github.com/dianabuilds/ardents-network/internal/successor/hosting"
	"time"
)

// hostingWorkFixture supplies installation roots, a token and externally owned authority facts.
// Observe must re-read current facts; this package does not authenticate Network.
type hostingWorkFixture struct {
	Root, Budget       string
	Receiver           receiving.Receiver
	NotAfter, Deadline time.Time
	Token              []byte
	Bytes              uint64
	Observe            func() (admission.AuthorityFacts, time.Time, error)
}

// runHostingFixture composes domain owners only for integration tests.
func runHostingFixture(ctx context.Context, p hostingWorkFixture) error {
	return runHostingFixtureWithTransfer(ctx, p, transferLoopback)
}

func runHostingFixtureWithTransfer(ctx context.Context, p hostingWorkFixture, transfer func(context.Context, uint64, time.Time, func() error) error) (outcome error) {
	if ctx == nil || p.Observe == nil || p.Bytes == 0 || p.Bytes > 64<<10 || !time.Now().Before(p.Deadline) || p.Deadline.After(time.Now().Add(5*time.Second)) {
		return errors.New("invalid local work plan")
	}
	if ctx.Err() != nil {
		return ctx.Err()
	}
	observe := monotonicObserver(p.Observe)
	initial, observedAt, err := observe()
	if err != nil {
		return err
	}
	receiver, err := receiving.Open(p.Root, p.Receiver, func() (receiving.Observation, error) {
		facts, now, err := observe()
		return receiving.Observation{Profile: facts, Receiver: p.Receiver, Now: now, NotAfter: p.NotAfter}, err
	})
	if err != nil {
		return err
	}
	defer func() { outcome = errors.Join(outcome, receiver.Close()) }()
	budget, err := hosting.Open(p.Budget)
	if err != nil {
		return err
	}
	defer func() { outcome = errors.Join(outcome, budget.Close()) }()
	workCtx, cancel := context.WithDeadline(ctx, p.Deadline)
	defer cancel()
	grant, err := receiver.Accept(workCtx, admission.ForwardClass, p.Token, p.Deadline, func() (func() error, error) {
		held, err := budget.Reserve(workCtx, hosting.ReservationRequest{
			Work: hosting.Traffic{Tx: 2 << 20, Rx: 2 << 20}, Termination: hosting.Traffic{Tx: 64 << 10, Rx: 64 << 10}, WorkUntil: p.Deadline, HoldUntil: p.Deadline.Add(time.Second),
		})
		if err != nil {
			return nil, err
		}
		return func() error {
			cleanup, stop := context.WithTimeout(context.Background(), time.Second)
			defer stop()
			return held.Release(cleanup)
		}, nil
	})
	if err != nil {
		return err
	}
	// transferLoopback joins its reader on every path before this release runs.
	defer func() { outcome = errors.Join(outcome, grant.Release()) }()
	if p.Bytes > grant.Allowance().Bytes() {
		return errors.New("work exceeds allowance")
	}
	// Receiving can shorten the requested deadline to its accepted authority.
	deadline := grant.Allowance().Deadline()
	admittedCtx, stop := context.WithDeadline(workCtx, deadline)
	defer stop()
	return transfer(admittedCtx, p.Bytes, deadline, workAuthority(initial, observedAt, deadline, observe))
}

// workAuthority retains the accepted facts and a monotonic observation floor.
func workAuthority(initial admission.AuthorityFacts, floor, deadline time.Time, observe func() (admission.AuthorityFacts, time.Time, error)) func() error {
	return func() error {
		facts, now, err := observe()
		if err != nil || facts != initial || now.Before(floor) || now.Before(initial.NotBefore) || !now.Before(initial.NotAfter) || !now.Before(deadline) {
			return errors.Join(errors.New("work authority unavailable"), err)
		}
		floor = now
		return nil
	}
}

// The same observation floor spans admission, durable spend and active work.
func monotonicObserver(source func() (admission.AuthorityFacts, time.Time, error)) func() (admission.AuthorityFacts, time.Time, error) {
	var floor time.Time
	return func() (admission.AuthorityFacts, time.Time, error) {
		facts, now, err := source()
		if err == nil && now.Before(floor) {
			err = errors.New("work observation moved backward")
		}
		if err == nil {
			floor = now
		}
		return facts, now, err
	}
}
