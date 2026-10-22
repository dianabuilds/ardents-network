package main

import (
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"crypto/tls"
	"errors"
	"io"
	"sync"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/installation/infrastructure"
	distribution "github.com/dianabuilds/ardents-network/internal/successor/network/source"
	"github.com/dianabuilds/ardents-network/internal/successor/network/state"
	"github.com/dianabuilds/ardents-network/internal/successor/processresource"
)

// sourceCommandFailure retains the complete local result while automatic
// command output stays finite and contains no native paths or private material.
type sourceCommandFailure struct{ cause error }

func (e *sourceCommandFailure) Error() string { return "source command failed" }
func (e *sourceCommandFailure) Unwrap() error { return e.cause }

// Only a complete original command result plus a successful one-use root
// retirement exchange can produce this terminal marker. The cancellation cause
// remains inspectable; main exits cleanly only for this exact completed stop.
type sourceExplicitStop struct{ cause error }

func (e *sourceExplicitStop) Error() string { return "source explicitly stopped" }
func (e *sourceExplicitStop) Unwrap() error { return e.cause }

func completedExplicitSourceStop(err error) bool {
	completed, ok := err.(*sourceExplicitStop)
	return ok && completed != nil && sourceCancellationOnly(completed.cause)
}

func runInstalledSource(ctx context.Context, binding string, out io.Writer) error {
	var startup *infrastructure.SourceStartup
	result := waitSourceCommand(ctx, out, "source.start-installed", func() error {
		var err error
		startup, err = serveInstalledSource(ctx, binding, out)
		return err
	})
	outcome := "retirement-failed"
	if sourceCancellationOnly(result) {
		outcome = "retired"
	}
	// The complete result includes every original join and any earlier unavailable
	// notice. Reporting grants no authority and cannot erase that result.
	if ctx != nil {
		report := writeSourceCommandOutcome(context.WithoutCancel(ctx), out, "source.start-installed", outcome)
		if report != nil {
			return &sourceCommandFailure{errors.Join(result, report)}
		}
	}
	if ctx != nil && startup != nil && sourceCancellationOnly(result) {
		completion, cancel := context.WithTimeout(context.WithoutCancel(ctx), 3*time.Second)
		reported, err := startup.ReportRetirement(completion, result)
		cancel()
		if err != nil {
			return &sourceCommandFailure{errors.Join(result, err)}
		}
		if reported {
			return &sourceExplicitStop{result}
		}
	}
	return result
}

func sourceCancellationOnly(err error) bool {
	if err == nil || err == context.Canceled {
		return true
	}
	if joined, ok := err.(interface{ Unwrap() []error }); ok {
		children := joined.Unwrap()
		if len(children) == 0 {
			return false
		}
		for _, child := range children {
			if child == nil || !sourceCancellationOnly(child) {
				return false
			}
		}
		return true
	}
	if wrapped, ok := err.(interface{ Unwrap() error }); ok {
		child := wrapped.Unwrap()
		return child != nil && sourceCancellationOnly(child)
	}
	return false
}

func waitSourceCommand(ctx context.Context, out io.Writer, operation string, work func() error) error {
	if ctx == nil {
		return &sourceCommandFailure{infrastructure.ErrInput}
	}
	done := make(chan error, 1)
	go func() { done <- work() }()
	select {
	case err := <-done:
		return err
	case <-ctx.Done():
	}
	notice := time.NewTimer(5 * time.Second)
	defer notice.Stop()
	select {
	case err := <-done:
		return err
	case <-notice.C:
		report := writeSourceCommandOutcome(context.WithoutCancel(ctx), out, operation, "completion-unavailable")
		return &sourceCommandFailure{errors.Join(ctx.Err(), errors.New("source completion unavailable"), report, <-done)}
	}
}

func serveInstalledSource(ctx context.Context, binding string, out io.Writer) (startup *infrastructure.SourceStartup, result error) {
	var err error
	startup, err = infrastructure.OpenSourceStartup(ctx, binding)
	if err != nil {
		return nil, &sourceCommandFailure{err}
	}
	defer startup.Stop()
	var guard *processresource.Guard
	var material infrastructure.SourceRuntimeInputs
	var config state.Config
	defer func() {
		// State.Open owns partial work, and the supervisor joins Close, Wait and all
		// original observations before input/key/proc retirement reaches this point.
		inputErr := startup.Close()
		clear(material.Plan)
		clear(material.Certificate)
		clear(material.PrivateKey)
		clear(material.ClientCA)
		if key, ok := config.Source.ServeCertificate.PrivateKey.(ed25519.PrivateKey); ok {
			clear(key)
		}
		var physical error
		if guard != nil {
			physical = guard.Close()
		}
		if result != nil || inputErr != nil || physical != nil {
			result = &sourceCommandFailure{errors.Join(result, inputErr, physical)}
		}
	}()
	material, err = startup.Inputs()
	if err != nil {
		return startup, err
	}
	plan, err := distribution.DecodeServingDeclaration(material.Plan)
	if err != nil {
		return startup, err
	}
	config, err = installedSourceStateConfig(plan)
	if err != nil {
		return startup, err
	}
	guard, err = processresource.Open(startup.Context(), processresource.SourceProfile)
	if err != nil {
		return startup, err
	}
	check := func() error {
		original := startup.Check()
		_, pressure := guard.Check()
		if original == nil && pressure == nil {
			original = startup.Check()
		}
		joined := errors.Join(original, pressure)
		if joined != nil {
			startup.Stop()
		}
		return joined
	}
	config.PermitWork = check
	if err = check(); err != nil {
		return startup, err
	}
	config.Source.ServeCertificate, err = tls.X509KeyPair(material.Certificate, material.PrivateKey)
	if err != nil {
		return startup, err
	}
	config.Source.ServeClientRootPEM = append([]byte(nil), material.ClientCA...)
	config.Source.ServeAddress = plan.Address
	config.Source.ServeClientKeyDigests = append([][32]byte(nil), plan.ClientPins...)
	if err = check(); err != nil {
		return startup, err
	}
	openState := state.Open
	if material.Invocation != 0 {
		openState = state.OpenRetained
	}
	owner, err := openState(config)
	if err != nil {
		return startup, err
	} // State joins every partial acquisition itself.
	return startup, superviseInstalledSource(startup.Context(), owner, check, startup.Stop, out)
}

// This declaration adaptation creates no accepted State or trusted time.
// State independently validates pins, original roots, signed evidence and clock.
func installedSourceStateConfig(plan distribution.ServingDeclaration) (state.Config, error) {
	n := plan.Network
	if n == nil || n.NetworkID == [32]byte{} || n.ProfileAuthority == [32]byte{} || plan.ProcessProfile != processresource.SourceProfile ||
		n.Threshold < 1 || n.Threshold > len(n.Authorities) || len(n.Authorities) > 16 {
		return state.Config{}, infrastructure.ErrInput
	}
	pins := make(map[[32]byte]ed25519.PublicKey, len(n.Authorities))
	for _, public := range n.Authorities {
		id := sha256.Sum256(public[:])
		if _, exists := pins[id]; exists || public == [32]byte{} {
			return state.Config{}, infrastructure.ErrBinding
		}
		pins[id] = append(ed25519.PublicKey(nil), public[:]...)
	}
	return state.Config{Root: n.Root, NetworkID: n.NetworkID, Authorities: pins, Threshold: n.Threshold,
		ClosedProfileAuthority: append(ed25519.PublicKey(nil), n.ProfileAuthority[:]...), AcceptedProfile: "ardents-route-v3",
		Clock: time.Now, ClockObservationFile: n.ClockObservationFile, LocalRoleStateRoot: plan.ParticipationRoot}, nil
}

type installedSourceState interface {
	Current() (state.Snapshot, error)
	Wait(context.Context) error
	Close() error
}

// A nil State.Wait after cancellation is ordinary completion; while the
// original caller is live it is an unexpected end of background work.
func sourceWaitOutcome(ctx context.Context, waitErr error) error {
	if original := ctx.Err(); original != nil {
		return errors.Join(original, waitErr)
	}
	if waitErr == nil {
		return errors.New("source background work ended")
	}
	return waitErr
}

// The supervisor owns only original ordering. State retains authority, serving
// and exposure removal; Installation and Guard retain native admission.
// Neither cancellation nor a bounded notice substitutes for physical join.
func superviseInstalledSource(ctx context.Context, owner installedSourceState, check func() error, deny context.CancelFunc, out io.Writer) error {
	closed := make(chan error, 1)
	var once sync.Once
	closeOriginal := func() { once.Do(func() { go func() { closed <- owner.Close() }() }) }
	callbackDone := make(chan struct{})
	stop := context.AfterFunc(ctx, func() { deny(); closeOriginal(); close(callbackDone) })
	defer func() {
		if !stop() {
			<-callbackDone
		}
	}()
	waitCtx, cancelWait := context.WithCancel(ctx)
	defer cancelWait()
	waited := make(chan error, 1)
	go func() { waited <- owner.Wait(waitCtx) }()
	observed := make(chan error, 1)
	go func() {
		_, err := owner.Current()
		if err == nil {
			err = check()
		}
		if err == nil {
			err = writeInstalledSourceOutcome(ctx, out, "listening")
		}
		if err == nil {
			err = check()
		}
		observed <- err
	}()
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	var first, waitErr, closeErr, late error
retained:
	for {
		select {
		case <-ctx.Done():
			first = ctx.Err()
			break retained
		case err := <-observed:
			observed = nil
			if err != nil {
				first = err
				break retained
			}
		case waitErr = <-waited:
			waited = nil
			first = sourceWaitOutcome(ctx, waitErr)
			break retained
		case <-ticker.C:
			if observed == nil {
				observed = make(chan error, 1)
				original := observed
				go func() { original <- check() }()
			}
		}
	}
	deny()
	cancelWait()
	closeOriginal()
	for closed != nil || waited != nil || observed != nil {
		select {
		case closeErr = <-closed:
			closed = nil
		case waitErr = <-waited:
			waited = nil
		case err := <-observed:
			late = errors.Join(late, err)
			observed = nil
		}
	}
	return errors.Join(first, late, waitErr, closeErr)
}
