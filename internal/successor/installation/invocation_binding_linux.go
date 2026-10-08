package installation

import (
	"context"
	"errors"
	"path/filepath"

	"github.com/dianabuilds/ardents-network/internal/successor/installation/process"
)

// The Installation binding retains its original caller, inspected bytes and
// lease independently of the kernel observation. Kernel facts grant no authority.
type installedProcessPin struct {
	ctx         context.Context
	reader      *installedInspection
	previous    inspectedGeneration
	pid         uint32
	invocation  [16]byte
	observation *process.Invocation
}

func pinInstalledProcess(ctx context.Context, reader *installedInspection, previous inspectedGeneration, pid uint32, invocation [16]byte) (pin *installedProcessPin, returnedErr error) {
	return pinOriginalProcess(ctx, ctx, reader, previous, pid, invocation)
}

// Failure-only cleanup may take physical observations after original
// cancellation. The retained pin still carries the original admission caller;
// the cleanup context can never authorize completion or renew its deadline.
func pinOriginalProcess(original, observationContext context.Context, reader *installedInspection, previous inspectedGeneration, pid uint32, invocation [16]byte) (pin *installedProcessPin, returnedErr error) {
	if original == nil || observationContext == nil || reader == nil || pid == 0 || invocation == [16]byte{} {
		return nil, ErrInput
	}
	if err := reader.observe(observationContext); err != nil {
		return nil, err
	}
	program := filepath.Join(previous.binding.InstallationRoot, "generations", previous.selected.GenerationDigest, "ardents-linux-amd64")
	observation, err := process.Retain(observationContext, process.Expected{
		PID: pid, UID: previous.binding.UID, GID: previous.binding.GID, Invocation: invocation,
		InstallationRoot: previous.binding.InstallationRoot, Generation: previous.selected.GenerationDigest,
		ProgramIdentity: reader.files[program].identity,
	})
	if err != nil {
		return nil, err
	}
	bound := &installedProcessPin{ctx: original, reader: reader, previous: previous, pid: pid, invocation: invocation, observation: observation}
	defer func() {
		if returnedErr != nil {
			returnedErr = errors.Join(returnedErr, bound.close())
			pin = nil
		}
	}()
	if err := bound.observeOriginalProcess(observationContext); err != nil {
		return nil, err
	}
	return bound, nil
}

func (pin *installedProcessPin) observe() error {
	if pin == nil {
		return ErrInput
	}
	return pin.observeOriginalProcess(pin.ctx)
}

// Failure cleanup may inspect the same bytes/lease and process using an
// uncancelled context. It cannot renew the original admission or replace the pin.
func (pin *installedProcessPin) observeOriginalProcess(ctx context.Context) error {
	if pin == nil || ctx == nil || pin.ctx == nil || pin.reader == nil || pin.observation == nil {
		return ErrInput
	}
	if err := pin.reader.observe(ctx); err != nil {
		return err
	}
	if err := pin.observation.Observe(ctx); err != nil {
		return err
	}
	return pin.reader.observe(ctx)
}

func (pin *installedProcessPin) close() error {
	if pin == nil {
		return nil
	}
	return pin.observation.Close()
}
