//go:build linux

package main

import (
	"context"
	"crypto/ed25519"
	"crypto/tls"
	"errors"
	"io"
	"os"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/network/state"
	"github.com/dianabuilds/ardents-network/internal/successor/processresource"
)

func runSource(ctx context.Context, args []string, out, diagnostic io.Writer) int {
	if ctx == nil || ctx.Err() != nil || len(args) != 2 || args[0] != "serve" {
		return 2
	}
	raw, err := readBounded(args[1], 16<<10)
	plan, decodeErr := decodeSourceServePlan(raw)
	if err != nil || decodeErr != nil || !validSourceServePlan(plan) {
		return 2
	}
	config, err := networkStateConfig(plan.Network)
	if err != nil {
		return 2
	}
	admissionCtx, deny := context.WithCancel(ctx)
	defer deny()
	guard, err := processresource.Open(admissionCtx, processresource.SourceProfile)
	if err != nil {
		return sourceStartupFailure(diagnostic, "process", err, nil)
	}
	check := func() error { _, err := guard.Check(); return err }
	config.PermitWork = check
	config.LocalRoleStateRoot = plan.ParticipationRoot
	defer func() {
		if key, ok := config.Source.ServeCertificate.PrivateKey.(ed25519.PrivateKey); ok {
			clear(key)
		}
	}()
	if err = sourceServingInputs(&config, plan, check); err != nil {
		return sourceStartupFailure(diagnostic, "input", err, guard.Close())
	}
	owner, err := state.Open(config)
	// State and its transport own defensive key copies after Open. Clear the
	// command's original only after every State user has joined below.
	if err != nil {
		return sourceStartupFailure(diagnostic, "state", err, guard.Close())
	}
	err = superviseSource(ctx, owner, check, deny, out, diagnostic)
	closeErr := guard.Close()
	if err != nil || closeErr != nil {
		if sourceFailureReport(diagnostic, "joined", err, closeErr) != nil {
			return 2
		}
		return 1
	}
	return sourceReportCode(diagnostic, "joined", "canceled", 130)
}

func validSourceServePlan(plan sourceServePlan) bool {
	return plan.ProcessProfile == processresource.SourceProfile &&
		validAdmissionAuthority("", plan.Network, plan.ParticipationRoot) &&
		independentRouteRoots(plan.Network.Root, plan.ParticipationRoot) &&
		plan.Network.Threshold > 0 && plan.Network.Threshold <= len(plan.Network.Authorities) &&
		absoluteAdmissionPath(plan.Certificate) && absoluteAdmissionPath(plan.PrivateKey) &&
		absoluteAdmissionPath(plan.ClientRoot) && len(plan.ClientPins) > 0 && len(plan.ClientPins) <= 3 && plan.Address != ""
}

func sourceServingInputs(config *state.Config, plan sourceServePlan, check func() error) error {
	read := func(path string, bound int64) ([]byte, error) {
		if err := check(); err != nil {
			return nil, err
		}
		raw, err := readBounded(path, bound)
		if next := check(); next != nil {
			clear(raw)
			return nil, errors.Join(err, next)
		}
		return raw, err
	}
	// An independent prepared State root must already exist. Open remains the
	// authority for layout, authentication, native lease and recovery admission.
	info, err := os.Stat(config.Root)
	if err != nil || !info.IsDir() {
		return errors.Join(errors.New("prepared State unavailable"), err)
	}
	certificate, err := read(plan.Certificate, 64<<10)
	if err != nil {
		return err
	}
	defer clear(certificate)
	key, err := read(plan.PrivateKey, 16<<10)
	if err != nil {
		return err
	}
	defer clear(key)
	config.Source.ServeCertificate, err = tls.X509KeyPair(certificate, key)
	if err != nil {
		return err
	}
	config.Source.ServeClientRootPEM, err = read(plan.ClientRoot, 64<<10)
	if err != nil {
		return err
	}
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

func superviseSource(ctx context.Context, owner sourceLifetime, check func() error, deny context.CancelFunc, out, diagnostic io.Writer) error {
	waitCtx, cancel := context.WithCancel(ctx)
	waited := make(chan error, 1)
	go func() { waited <- owner.Wait(waitCtx) }()
	_, first := owner.Current()
	firstOwner := "state"
	if first == nil {
		first = check()
		firstOwner = "process"
	}
	if first == nil {
		first = sourceReportContext(ctx, out, "listening", "original-source")
		firstOwner = "output"
	}
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	joinedWait := false
	for first == nil {
		select {
		case <-ctx.Done():
			// Ordinary caller retirement is separate from actual owner failures.
			cancel()
			firstOwner = "caller"
			goto retire
		case first = <-waited:
			firstOwner = "state"
			joinedWait = true
			if first == nil && ctx.Err() != nil {
				firstOwner = "caller"
				goto retire
			}
			if first == nil {
				first = errors.New("Source background work ended")
			}
		case <-ticker.C:
			first = check()
			firstOwner = "process"
		}
	}
retire:
	// Fence State's borrowed PermitWork synchronously, before physical Close
	// can wait for any original lock or borrower.
	deny()
	cancel()
	if ctx.Err() != nil && first == ctx.Err() {
		first, firstOwner = nil, "caller"
	}
	// Close denies and joins all original State transport/background users,
	// then releases its guards/root. Process pins remain held until this returns.
	closed := make(chan error, 1)
	go func() { closed <- owner.Close() }()
	var closeErr error
	// A finite notice is not physical completion. Keep the original caller,
	// State, keys and process guard while Close remains blocked, and retain
	// the failed outcome even if it eventually joins.
	notice := time.NewTimer(5 * time.Second)
	select {
	case closeErr = <-closed:
		notice.Stop()
	case <-notice.C:
		first = errors.Join(first, errors.New("Source completion unavailable"), sourceReport(diagnostic, "retiring", "completion-unavailable"))
		if firstOwner == "caller" {
			firstOwner = "completion"
		}
		closeErr = <-closed
	}
	var waitErr error
	if !joinedWait {
		waitErr = <-waited
	}
	var processErr error
	if firstOwner != "process" {
		// State users have joined and our admission context is already denied.
		// Check cannot grant work or resample success here. It returns the
		// Guard's original latched failure, or only our exact retirement notice.
		processErr = check()
		if processErr == context.Canceled || processErr == ctx.Err() {
			processErr = nil
		}
	}
	if first == nil && waitErr == nil && closeErr == nil && processErr == nil {
		return nil
	}
	if first == nil && processErr != nil && firstOwner == "caller" {
		firstOwner = "process"
	}
	return &sourceTerminal{firstOwner: firstOwner, first: first, process: processErr, wait: waitErr, cleanup: closeErr}
}

// Retains original causes without publishing native/private error text.
type sourceTerminal struct {
	firstOwner                    string
	first, process, wait, cleanup error
}

func (result *sourceTerminal) Error() string { return "original Source failed" }
func (result *sourceTerminal) Unwrap() []error {
	var causes []error
	for _, cause := range []error{result.first, result.process, result.wait, result.cleanup} {
		if cause != nil {
			causes = append(causes, cause)
		}
	}
	return causes
}
