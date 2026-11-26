//go:build linux || darwin

package serving

import (
	"context"
	"crypto/ed25519"
	"crypto/tls"
	"errors"
	"os"

	"github.com/dianabuilds/ardents-network/internal/domain/network/source/servingdeclaration"
	"github.com/dianabuilds/ardents-network/internal/domain/network/state"
	"github.com/dianabuilds/ardents-network/internal/platform/artifacts"
	"github.com/dianabuilds/ardents-network/internal/platform/processguard"
)

// Run serves one original Source invocation and waits for its physical join.
func Run(ctx context.Context, planPath string, retained bool, report func(context.Context, Event) error) (outcome Outcome) {
	if ctx == nil || ctx.Err() != nil || report == nil {
		return Invalid
	}
	inputs, plan, err := AcquireInputs(ctx, planPath, nil)
	if err != nil {
		if report(context.Background(), Event{Phase: "refused", Outcome: "failed", Failure: &Terminal{FirstOwner: "input", First: err}}) != nil {
			return Invalid
		}
		return Refused
	}
	var guard *processguard.Guard
	var operation error
	var config state.Config
	var completion state.SourceCompletion
	completionRetained := false
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
		var reportErr error
		if operation != nil || inputErr != nil || processErr != nil {
			var terminal *Terminal
			if !errors.As(operation, &terminal) {
				category := "input"
				if operation == nil && inputErr == nil {
					category = "process"
				}
				terminal = &Terminal{FirstOwner: category, First: operation}
			}
			terminal.InputClose = inputErr
			outcome = Refused
			reportErr = report(context.Background(), Event{Phase: phase, Outcome: "failed", Failure: terminal, ProcessClose: processErr})
			if reportErr != nil {
				outcome = Invalid
			}
		} else if outcome == Canceled {
			reportErr = report(context.Background(), Event{Phase: "joined", Outcome: "canceled"})
			if reportErr != nil {
				outcome = Invalid
			}
		}
		if completionRetained {
			terminal := errors.Join(operation, inputErr, processErr, reportErr)
			if err := completion.Finish(terminal); err != nil && terminal == nil {
				outcome = Refused
				if report(context.Background(), Event{Phase: "completion", Outcome: "failed", Failure: &Terminal{FirstOwner: "state", StateClose: err}}) != nil {
					outcome = Invalid
				}
			}
		}
	}()
	config, err = (*state.AuthorityDeclaration)(plan.Network).Config()
	if err != nil {
		operation = err
		return Invalid
	}
	admissionCtx, cancelAdmission := context.WithCancel(inputs.Context())
	deny := func() { inputs.Deny(); cancelAdmission() }
	defer deny()
	guard, err = processguard.Open(admissionCtx, processguard.SourceProfile)
	if err != nil {
		operation = &Terminal{FirstOwner: "process", First: err}
		return Refused
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
	if err = ConfigureInputs(&config, plan, inputs, check); err != nil {
		operation = &Terminal{FirstOwner: "input", First: err}
		return Refused
	}
	openState := state.Open
	if retained {
		openState = state.OpenRetained
	}
	owner, err := openState(config)
	// State and its transport own defensive key copies after Open. Clear the
	// command's original only after every State user has joined below.
	if err != nil {
		firstOwner := "state"
		if errors.Is(err, artifacts.ErrObservedInput) {
			firstOwner = "input"
		}
		operation = &Terminal{FirstOwner: firstOwner, First: err}
		return Refused
	}
	completion, err = owner.RetainSourceCompletion()
	if err != nil {
		operation = errors.Join(err, owner.Close())
		return Refused
	}
	completionRetained = true
	operation = Supervise(inputs.Context(), owner, check, deny, report, inputs.Close)
	phase = "joined"
	return Canceled
}

// ConfigureInputs binds the retained original transport credentials.
func ConfigureInputs(config *state.Config, plan servingdeclaration.ServingDeclaration, inputs artifacts.ObservedInputs, check func() error) error {
	if err := check(); err != nil {
		return err
	}
	// An independent prepared State root must already exist. Open remains the
	// authority for layout, authentication, native lease and recovery admission.
	info, err := os.Stat(config.Root)
	if err != nil || !info.IsDir() {
		return errors.Join(errors.New("prepared State unavailable"), err)
	}
	config.Source.ServeCertificate, err = tls.X509KeyPair(inputs.Files()[1].Bytes(), inputs.Files()[2].Bytes())
	if err != nil {
		return err
	}
	config.Source.ServeClientRootPEM = append([]byte(nil), inputs.Files()[3].Bytes()...)
	config.Source.ServeAddress = plan.Address
	config.Source.ServeClientKeyDigests = append([][32]byte(nil), plan.ClientPins...)
	return check()
}
