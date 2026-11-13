package main

import (
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"io"
	"net"
	"os"
	"runtime"
	"syscall"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/installation/join"
	"github.com/dianabuilds/ardents-network/internal/successor/network"
	"github.com/dianabuilds/ardents-network/internal/successor/network/state"
	"github.com/dianabuilds/ardents-network/internal/successor/node"
	"github.com/dianabuilds/ardents-network/internal/successor/route/role"
)

// Only original joined command results can later participate in the separate
// root retirement protocol. Public output never serializes private causes.
type joinCommandFailure struct{ cause error }
type joinExplicitStop struct{ cause error }

func (e *joinExplicitStop) Error() string { return "join explicitly stopped" }
func (e *joinExplicitStop) Unwrap() error { return e.cause }
func completedJoinExplicitStop(err error) bool {
	e, ok := err.(*joinExplicitStop)
	return ok && e != nil && joinCancellationOnly(e.cause)
}

func (e *joinCommandFailure) Error() string { return "join command failed" }
func (e *joinCommandFailure) Unwrap() error { return e.cause }

func joinCancellationOnly(err error) bool {
	if err == nil || err == context.Canceled {
		return true
	}
	if joined, ok := err.(interface{ Unwrap() []error }); ok {
		children := joined.Unwrap()
		if len(children) == 0 {
			return false
		}
		for _, child := range children {
			if child == nil || !joinCancellationOnly(child) {
				return false
			}
		}
		return true
	}
	return false
}

type installedJoinState interface {
	CurrentRuntime() (network.RuntimeView, error)
	Close() error
}

// The canonical consumer directly composes new owners. It neither invokes a
// development command nor borrows Source startup, State or completion rights.
func runInstalledJoin(parent context.Context, binding string, out io.Writer) (returned error) {
	if parent == nil || out == nil {
		return &joinCommandFailure{join.ErrInput}
	}
	var startup *join.Startup
	var owner installedJoinState
	var original *node.Join
	var privatePlan []byte
	stage, startupComplete := "installation", false
	defer func() {
		if startup != nil {
			startup.Stop()
		}
		if original != nil {
			original.Stop()
			returned = errors.Join(returned, original.Close())
		}
		if owner != nil {
			returned = errors.Join(returned, owner.Close())
		}
		returned = errors.Join(returned, startup.Close())
		if startup != nil {
			raw, err := json.Marshal(struct {
				Operation    string                  `json:"operation"`
				Phase        string                  `json:"phase"`
				Measurements join.ObservationSummary `json:"measurements"`
			}{"join.start-installed", "startup-observation-summary", startup.Observations()})
			returned = errors.Join(returned, err)
			if err == nil {
				returned = errors.Join(returned, writeJoinRuntimeEvent(context.WithoutCancel(parent), out, append(raw, '\n')))
			}
		}
		clear(privatePlan)
		if startup != nil && joinCancellationOnly(returned) {
			completion, cancel := context.WithTimeout(context.WithoutCancel(parent), 3*time.Second)
			reported, err := startup.ReportRetirement(completion, returned)
			cancel()
			returned = errors.Join(returned, err)
			if reported && err == nil {
				returned = &joinExplicitStop{returned}
				return
			}
		}
		if returned != nil {
			if !startupComplete {
				returned = errors.Join(returned, writeJoinStartupRefusal(context.WithoutCancel(parent), out, stage, node.StartupFailure(returned), returned))
			}
			returned = &joinCommandFailure{returned}
		}
	}()
	var err error
	startup, err = join.OpenStartup(parent, binding)
	if err != nil {
		return err
	}
	stage = "runtime-inputs"
	material, err := startup.Inputs()
	if err != nil {
		return err
	}
	privatePlan = material.Plan
	stage = "plan"
	plan, err := join.DecodePlan(material.Plan)
	if err != nil {
		return err
	}
	config, err := installedJoinStateConfig(plan)
	if err != nil {
		return err
	}
	config.PermitWork = startup.Check
	// Both State openers invoke this original Startup admission before root I/O.
	openState := state.Open
	if material.ExistingHistory {
		openState = state.OpenRetained
	}
	stage = "state"
	opened, err := openState(config)
	if err != nil {
		return err
	}
	owner = opened
	// The returned view remains the genuine State owner's opaque observation.
	// State invokes PermitWork before its lock and authority read; the explicit
	// after-check completes the Installation bracket outside Node's mutex.
	current := func() (network.RuntimeView, error) {
		view, err := owner.CurrentRuntime()
		if err != nil {
			return network.RuntimeView{}, err
		}
		if err := startup.Check(); err != nil {
			return network.RuntimeView{}, err
		}
		return view, nil
	}
	view, err := current()
	if err != nil {
		return err
	}
	stage = "duty"
	duty, err := view.RetainDuty(plan.NodeID, view.ObservedAt())
	if err != nil {
		return err
	}
	listen := node.ListenJoin
	if material.ExistingHistory {
		listen = node.ListenJoinRetained
	}
	stage = "node"
	original, err = listen(startup.Context(), node.JoinConfig{
		Authority:         role.Authority{Current: current, Duty: duty, Profile: view.Profile().ProfileBinding},
		ParticipationRoot: plan.ParticipationRoot, SpendRoot: plan.SpendRoot, HostingRoot: plan.HostingRoot,
		Certificate: plan.Certificate, PrivateKey: plan.PrivateKey, Work: plan.Work, Termination: plan.Termination,
	})
	if err != nil {
		return err
	}
	stage = "listening-check"
	if err := startup.Check(); err != nil {
		return err
	}
	stage = "listening-output"
	if err := writeJoinRuntimeEvent(startup.Context(), out, []byte("{\"operation\":\"join.start-installed\",\"phase\":\"listening\"}\n")); err != nil {
		return err
	}
	stage = "listening-check"
	if err := startup.Check(); err != nil {
		return err
	}
	startupComplete = true
	return superviseInstalledJoin(startup, owner, original, time.Duration(plan.RetirementWaitMS)*time.Millisecond, out)
}

// Only joined rejection facts leave startup. Neither a private error string nor
// an arbitrary stage value may enter this bounded public diagnostic.
func writeJoinStartupRefusal(ctx context.Context, out io.Writer, stage string, fact *node.FailureDiagnostic, cause error) error {
	switch stage {
	case "installation", "runtime-inputs", "plan", "state", "duty", "node", "listening-check", "listening-output":
	default:
		stage = "unavailable"
	}
	raw, err := json.Marshal(struct {
		Operation string                  `json:"operation"`
		Phase     string                  `json:"phase"`
		Stage     string                  `json:"stage"`
		Failure   *node.FailureDiagnostic `json:"first_failure,omitempty"`
		Errors    []joinErrorDiagnostic   `json:"error_shape"`
	}{"join.start-installed", "startup-refused", stage, fact, joinErrorShape(cause)})
	if err != nil {
		return err
	}
	return writeJoinRuntimeEvent(ctx, out, append(raw, '\n'))
}

func installedJoinStateConfig(plan join.Plan) (state.Config, error) {
	n := plan.Network
	if n.NetworkID == [32]byte{} || n.ProfileAuthority == [32]byte{} || n.Threshold < 1 || n.Threshold > len(n.Authorities) || len(n.Authorities) > 16 {
		return state.Config{}, join.ErrInput
	}
	pins := make(map[[32]byte]ed25519.PublicKey, len(n.Authorities))
	for _, public := range n.Authorities {
		id := sha256.Sum256(public[:])
		if _, duplicate := pins[id]; duplicate || public == [32]byte{} {
			return state.Config{}, join.ErrBinding
		}
		pins[id] = append(ed25519.PublicKey(nil), public[:]...)
	}
	return state.Config{Root: n.Root, NetworkID: n.NetworkID, Authorities: pins, Threshold: n.Threshold,
		ClosedProfileAuthority: append(ed25519.PublicKey(nil), n.ProfileAuthority[:]...), AcceptedProfile: "ardents-route-v3",
		Clock: time.Now, ClockObservationFile: n.ClockObservationFile, LocalRoleStateRoot: plan.ParticipationRoot}, nil
}

func superviseInstalledJoin(startup *join.Startup, owner installedJoinState, original *node.Join, notice time.Duration, out io.Writer) error {
	// This closed State configuration has no Source server or automatic
	// refresh worker. Wait would report absent background work, not failure
	// of its accepted State. Node reobserves genuine CurrentRuntime at effects
	// and in its original supervisor; command composition retains State last.
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	var first error
	trigger := "caller"
running:
	for {
		select {
		case <-startup.Context().Done():
			first = startup.Context().Err()
			break running
		case <-original.Done():
			trigger = "node"
			first = original.Close()
			if first == nil && startup.Context().Err() == nil {
				first = errors.New("original Join lifetime ended")
			}
			break running
		case <-ticker.C:
			if err := startup.Check(); err != nil {
				trigger = "installation"
				first = err
				break running
			}
		}
	}
	startup.Stop()
	original.Stop()
	// State remains owned until the original Node's Route/Receiving/Hosting/
	// producer/Guard/key users have actually joined. A notice releases nothing.
	joined := make(chan error, 1)
	go func() { joined <- errors.Join(original.Close(), owner.Close()) }()
	timer := time.NewTimer(notice)
	defer timer.Stop()
	var physical, unavailable error
	select {
	case physical = <-joined:
	case <-timer.C:
		unavailable = errors.New("original Join completion unavailable at notice bound")
		outputErr := writeJoinRuntimeEvent(context.WithoutCancel(startup.Context()), out, []byte("{\"operation\":\"join.start-installed\",\"phase\":\"retiring\",\"outcome\":\"completion-unavailable\"}\n"))
		unavailable = errors.Join(unavailable, outputErr)
		physical = <-joined // retain original process and every remaining owner
	}
	result := errors.Join(first, physical, unavailable)
	if result != nil && !joinCancellationOnly(result) {
		// These existing Node projections contain fixed local categories and
		// numeric observations only. Never print the private error chain or
		// treat a diagnostic as Installation retirement evidence.
		firstOwner, cleanupOwner := original.JoinedFailureOwners()
		raw, reportErr := json.Marshal(struct {
			Operation      string                  `json:"operation"`
			Phase          string                  `json:"phase"`
			Outcome        string                  `json:"outcome"`
			Trigger        string                  `json:"trigger"`
			FirstOwner     string                  `json:"first_owner,omitempty"`
			CleanupOwner   string                  `json:"cleanup_owner,omitempty"`
			FirstFailure   *node.FailureDiagnostic `json:"first_failure,omitempty"`
			RetainedErrors []joinErrorDiagnostic   `json:"retained_errors,omitempty"`
		}{"join.start-installed", "joined", "failed", trigger, firstOwner, cleanupOwner, original.FirstFailure(), joinErrorShape(result)})
		if reportErr == nil {
			reportErr = writeJoinRuntimeEvent(context.WithoutCancel(startup.Context()), out, append(raw, '\n'))
		}
		result = errors.Join(result, reportErr)
	}
	return result
}

type joinErrorDiagnostic struct {
	Kind      string `json:"kind"`
	Errno     int    `json:"errno,omitempty"`
	OverdueNS uint64 `json:"overdue_ns,omitempty"`
}

// Shape is a finite diagnostic, never an outcome classifier. Preserve wrappers
// as wrappers; call no Error method and reveal no private text or native path.
func joinErrorShape(original error) []joinErrorDiagnostic {
	var result []joinErrorDiagnostic
	var visit func(error)
	visit = func(cause error) {
		if cause == nil {
			return
		}
		if len(result) >= 64 {
			if len(result) == 64 {
				result = append(result, joinErrorDiagnostic{Kind: "truncated"})
			}
			return
		}
		fact := joinErrorDiagnostic{Kind: "other"}
		switch cause {
		case context.Canceled:
			fact.Kind = "canceled"
		case context.DeadlineExceeded:
			fact.Kind = "deadline"
		case io.EOF:
			fact.Kind = "eof"
		case io.ErrUnexpectedEOF:
			fact.Kind = "unexpected-eof"
		case io.ErrClosedPipe:
			fact.Kind = "closed-pipe"
		case io.ErrShortWrite:
			fact.Kind = "short-write"
		case os.ErrClosed:
			fact.Kind = "file-closed"
		case os.ErrDeadlineExceeded:
			fact.Kind = "io-deadline"
		case net.ErrClosed:
			fact.Kind = "net-closed"
		default:
			if native, ok := cause.(syscall.Errno); ok {
				fact.Kind, fact.Errno = "native-errno", int(native)
			} else if _, ok := cause.(interface{ Unwrap() []error }); ok {
				fact.Kind = "joined"
			} else if _, ok := cause.(interface{ Unwrap() error }); ok {
				fact.Kind = "wrapped"
			}
		}
		if refusal, ok := cause.(interface{ PreOutputDeadline() time.Time }); ok {
			fact.Kind = "frame-before-output-expiry"
			if elapsed := time.Since(refusal.PreOutputDeadline()); elapsed > 0 {
				fact.OverdueNS = uint64(elapsed)
			}
		}
		result = append(result, fact)
		if joined, ok := cause.(interface{ Unwrap() []error }); ok {
			for _, child := range joined.Unwrap() {
				visit(child)
				if len(result) > 64 {
					break
				}
			}
		} else if wrapped, ok := cause.(interface{ Unwrap() error }); ok {
			visit(wrapped.Unwrap())
		}
	}
	visit(original)
	return result
}

func writeJoinRuntimeEvent(ctx context.Context, out io.Writer, raw []byte) error {
	if ctx == nil || len(raw) == 0 || len(raw) > 8192 {
		return errors.New("join output requires an original context and bounded event")
	}
	bounded, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()
	if err := bounded.Err(); err != nil {
		return err
	}
	file, native := out.(*os.File)
	if !native || file == nil {
		return errors.New("join output requires an interruptible native descriptor")
	}
	info, err := file.Stat()
	if err != nil {
		return err
	}
	if info.Mode()&(os.ModeNamedPipe|os.ModeCharDevice|os.ModeSocket) == 0 {
		return errors.New("join output requires an interruptible native descriptor")
	}
	// Obtain the original descriptor once: another File.Fd call could restore
	// blocking mode. Its caller retains the file until this synchronous join.
	fd := int(file.Fd())
	defer runtime.KeepAlive(file)
	if err := syscall.SetNonblock(fd, true); err != nil {
		return err
	}
	ticker := time.NewTicker(5 * time.Millisecond)
	defer ticker.Stop()
	for len(raw) != 0 {
		if err := bounded.Err(); err != nil {
			return err
		}
		count, err := syscall.Write(fd, raw)
		if count > 0 {
			raw = raw[count:]
		}
		if err == nil {
			continue
		}
		if !errors.Is(err, syscall.EAGAIN) && !errors.Is(err, syscall.EWOULDBLOCK) && !errors.Is(err, syscall.EINTR) {
			return err
		}
		select {
		case <-bounded.Done():
			return bounded.Err()
		case <-ticker.C:
		}
	}
	return bounded.Err()
}
