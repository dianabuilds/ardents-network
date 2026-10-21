//go:build linux

package main

import (
	"context"
	"crypto/ed25519"
	"crypto/tls"
	"errors"
	"io"
	"os"
	"sync"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/network/state"
	"github.com/dianabuilds/ardents-network/internal/successor/processresource"
)

func runSource(ctx context.Context, args []string, out, diagnostic io.Writer) (code int) {
	if ctx == nil || ctx.Err() != nil || len(args) != 2 || args[0] != "serve" {
		return 2
	}
	inputs, plan, err := acquireSourceInputs(ctx, args[1])
	if err != nil {
		return sourceStartupFailure(diagnostic, "input", err, nil)
	}
	var guard *processresource.Guard
	var operation error
	var config state.Config
	phase := "refused"
	defer func() {
		// Every State user and its original Wait have joined before this point,
		// including partial Open failure. Input/key users finish before pins.
		if key, ok := config.Source.ServeCertificate.PrivateKey.(ed25519.PrivateKey); ok {
			clear(key)
		}
		inputErr := inputs.Close()
		var processErr error
		if guard != nil {
			processErr = guard.Close()
		}
		if operation != nil || inputErr != nil || processErr != nil {
			var terminal *sourceTerminal
			if !errors.As(operation, &terminal) {
				category := "input"
				if operation == nil && inputErr == nil {
					category = "process"
				}
				terminal = &sourceTerminal{firstOwner: category, first: operation}
			}
			terminal.inputCleanup = inputErr
			code = 1
			if sourceFailureReport(diagnostic, phase, terminal, processErr) != nil {
				code = 2
			}
		} else if code == 130 {
			code = sourceReportCode(diagnostic, "joined", "canceled", 130)
		}
	}()
	config, err = networkStateConfig(plan.Network)
	if err != nil {
		operation = err
		return 2
	}
	admissionCtx, cancelAdmission := context.WithCancel(inputs.ctx)
	deny := func() { inputs.deny(); cancelAdmission() }
	defer deny()
	guard, err = processresource.Open(admissionCtx, processresource.SourceProfile)
	if err != nil {
		operation = &sourceTerminal{firstOwner: "process", first: err}
		return 1
	}
	check := func() error {
		inputErr := inputs.Check()
		// Even after input denial, retrieve an independently latched process
		// failure. Its canceled caller prevents a new accepting observation.
		_, processErr := guard.Check()
		if inputErr == nil {
			inputErr = inputs.Check()
		}
		if inputErr != nil {
			if processErr != nil && processErr != admissionCtx.Err() {
				return errors.Join(inputErr, processErr)
			}
			return inputErr
		}
		return processErr
	}
	config.PermitWork = check
	config.LocalRoleStateRoot = plan.ParticipationRoot
	if err = sourceServingInputs(&config, plan, inputs, check); err != nil {
		operation = &sourceTerminal{firstOwner: "input", first: err}
		return 1
	}
	owner, err := state.Open(config)
	// State and its transport own defensive key copies after Open. Clear the
	// command's original only after every State user has joined below.
	if err != nil {
		firstOwner := "state"
		if errors.Is(err, errSourceInput) {
			firstOwner = "input"
		}
		operation = &sourceTerminal{firstOwner: firstOwner, first: err}
		return 1
	}
	operation = superviseSource(inputs.ctx, owner, check, deny, out, diagnostic, inputs.Close)
	phase = "joined"
	return 130
}

func validSourceServePlan(plan sourceServePlan) bool {
	return plan.ProcessProfile == processresource.SourceProfile &&
		validAdmissionAuthority("", plan.Network, plan.ParticipationRoot) &&
		independentRouteRoots(plan.Network.Root, plan.ParticipationRoot) &&
		plan.Network.Threshold > 0 && plan.Network.Threshold <= len(plan.Network.Authorities) &&
		absoluteAdmissionPath(plan.Certificate) && absoluteAdmissionPath(plan.PrivateKey) &&
		absoluteAdmissionPath(plan.ClientRoot) && len(plan.ClientPins) > 0 && len(plan.ClientPins) <= 3 && plan.Address != ""
}

func sourceServingInputs(config *state.Config, plan sourceServePlan, inputs sourceInputs, check func() error) error {
	if err := check(); err != nil {
		return err
	}
	// An independent prepared State root must already exist. Open remains the
	// authority for layout, authentication, native lease and recovery admission.
	info, err := os.Stat(config.Root)
	if err != nil || !info.IsDir() {
		return errors.Join(errors.New("prepared State unavailable"), err)
	}
	config.Source.ServeCertificate, err = tls.X509KeyPair(inputs.files[1].raw, inputs.files[2].raw)
	if err != nil {
		return err
	}
	config.Source.ServeClientRootPEM = append([]byte(nil), inputs.files[3].raw...)
	config.Source.ServeAddress = plan.Address
	config.Source.ServeClientKeyDigests = append([][32]byte(nil), plan.ClientPins...)
	return check()
}

// sourceLifetime is the exact original State contract used by supervision.
// It grants no policy callback or replacement State construction.
type sourceLifetime interface {
	Current() (state.Snapshot, error)
	Wait(context.Context) error
	Close() error
}

type sourceObservation struct {
	owner string
	err   error
}

func superviseSource(ctx context.Context, owner sourceLifetime, check func() error, deny context.CancelFunc, out, diagnostic io.Writer, joinInputs func() error) error {
	// Original cancellation starts the same physical Close even while an
	// observation is held. No callback or result notification is a join.
	closed := make(chan error, 1)
	var closeOnce sync.Once
	beginClose := func() { closeOnce.Do(func() { go func() { closed <- owner.Close() }() }) }
	callbackDone := make(chan struct{})
	stopCallback := context.AfterFunc(ctx, func() { deny(); beginClose(); close(callbackDone) })
	defer func() {
		if !stopCallback() {
			<-callbackDone
		}
	}()
	waitCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	observationCtx, stopObservation := context.WithCancel(ctx)
	defer stopObservation()
	waited := make(chan error, 1)
	go func() { waited <- owner.Wait(waitCtx) }()

	// One original observation at a time, retained through retirement. This
	// keeps denial/interrupt/notice independent of physical Check or output I/O.
	observing := make(chan sourceObservation, 1)
	go func() {
		_, err := owner.Current()
		category := "state"
		if errors.Is(err, errSourceInput) {
			category = "input"
		}
		if err == nil {
			err = check()
			category = sourceCheckOwner(err)
		}
		if err == nil {
			err = sourceReportContext(observationCtx, out, "listening", "original-source")
			category = "output"
		}
		if err == nil {
			err = check()
			category = sourceCheckOwner(err)
		}
		observing <- sourceObservation{category, err}
	}()
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	var first error
	firstOwner := "caller"
	var waitErr error
	for first == nil {
		select {
		case <-ctx.Done():
			firstOwner = "caller"
			goto retire
		case observation := <-observing:
			observing = nil
			first, firstOwner = observation.err, observation.owner
			if first != nil && observation.owner == "state" && ctx.Err() != nil && check() == ctx.Err() {
				first, firstOwner = nil, "caller"
				goto retire
			}
			if first == ctx.Err() && ctx.Err() != nil {
				first, firstOwner = nil, "caller"
				goto retire
			}
		case waitErr = <-waited:
			waited = nil
			firstOwner = "state"
			if waitErr == nil && ctx.Err() != nil {
				firstOwner = "caller"
				goto retire
			}
			first = waitErr
			if first == nil {
				first = errors.New("Source background work ended")
			}
		case <-ticker.C:
			if observing == nil {
				observing = make(chan sourceObservation, 1)
				original := observing
				go func() {
					err := check()
					original <- sourceObservation{sourceCheckOwner(err), err}
				}()
			}
		}
	}
retire:
	deny()
	stopObservation()
	cancel()
	beginClose()
	var closeErr error
	// The failed notice covers *all* outstanding original users, not just
	// Close: held Wait/Check/output still retain every needed input/process pin.
	notice := time.NewTimer(5 * time.Second)
	defer notice.Stop()
	unavailable := func() {
		first = errors.Join(first, errors.New("Source completion unavailable"), sourceReport(diagnostic, "retiring", "completion-unavailable"))
		if firstOwner == "caller" {
			firstOwner = "completion"
		}
	}
	for closed != nil || waited != nil || observing != nil {
		select {
		case closeErr = <-closed:
			closed = nil
		case waitErr = <-waited:
			waited = nil
		case observation := <-observing:
			observing = nil
			if observation.err != nil && observation.err != ctx.Err() && observation.err != observationCtx.Err() {
				// Our own cancellation can close State during Current. Its
				// real terminal failures are retained by the same Close/Wait.
				if !(observation.owner == "state" && ctx.Err() != nil && check() == ctx.Err()) {
					if first == nil {
						firstOwner = observation.owner
					}
					first = errors.Join(first, observation.err)
				}
			}
		case <-notice.C:
			unavailable()
		}
	}
	// This is only physical retirement of command-owned inputs. No accepting
	// callback or State policy crosses here. The same notice spans their native
	// observers and actual descriptor Close; process pins still belong to caller.
	var inputCleanup error
	if joinInputs != nil {
		joinedInputs := make(chan error, 1)
		go func() { joinedInputs <- joinInputs() }()
		for joinedInputs != nil {
			select {
			case inputCleanup = <-joinedInputs:
				joinedInputs = nil
			case <-notice.C:
				unavailable()
			}
		}
	}
	processErr := check()
	if processErr == context.Canceled || processErr == ctx.Err() {
		processErr = nil
	}
	if first == nil && waitErr == nil && closeErr == nil && processErr == nil && inputCleanup == nil {
		return nil
	}
	if first == nil && firstOwner == "caller" {
		switch {
		case closeErr != nil || waitErr != nil:
			firstOwner = "state"
		case inputCleanup != nil:
			firstOwner = "input"
		case processErr != nil:
			firstOwner = sourceCheckOwner(processErr)
		}
	}
	return &sourceTerminal{firstOwner: firstOwner, first: first, process: processErr, wait: waitErr, cleanup: closeErr, inputCleanup: inputCleanup}
}

func sourceCheckOwner(err error) string {
	if errors.Is(err, errSourceInput) {
		return "input"
	}
	return "process"
}

// Retains original causes without publishing native/private error text.
type sourceTerminal struct {
	firstOwner                    string
	first, process, wait, cleanup error
	inputCleanup                  error
}

func (result *sourceTerminal) Error() string { return "original Source failed" }
func (result *sourceTerminal) Unwrap() []error {
	var causes []error
	for _, cause := range []error{result.first, result.process, result.wait, result.cleanup, result.inputCleanup} {
		if cause != nil {
			causes = append(causes, cause)
		}
	}
	return causes
}
