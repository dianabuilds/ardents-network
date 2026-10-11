package main

import (
	"context"
	"errors"
	"io"

	"github.com/dianabuilds/ardents-network/internal/custody"
	newcustody "github.com/dianabuilds/ardents-network/internal/successor/custody"
)

func run(ctx context.Context, arguments []string, output io.Writer, input custody.SecretInput) error {
	if len(arguments) == 0 {
		return errors.New("usage: ardents-custody <create-service-authority|issue-service-credential|create-admission-authority|issue-admission-permission|inspect-envelope|verify-record|export-recovery-bundle|restore-recovery-bundle|purge-record> [flags]")
	}
	switch arguments[0] {
	case "inspect-envelope":
		return inspectEnvelope(arguments[1:], output)
	case "create-service-authority", "issue-service-credential":
		return serviceAuthority(ctx, arguments[0], arguments[1:], output, input)
	case "create-admission-authority", "issue-admission-permission":
		return admissionAuthority(ctx, arguments[0], arguments[1:], output, custodyTerminal{input})
	case "verify-record":
		return verifyRecord(ctx, arguments[1:], output, custodyTerminal{input})
	case "export-recovery-bundle", "restore-recovery-bundle":
		return recoveryBundle(ctx, arguments[0], arguments[1:], output, custodyTerminal{input})
	case "purge-record":
		return purgeRecord(ctx, arguments[1:], output, custodyTerminal{input})
	default:
		return errors.New("usage: ardents-custody <create-service-authority|issue-service-credential|create-admission-authority|issue-admission-permission|inspect-envelope|verify-record|export-recovery-bundle|restore-recovery-bundle|purge-record> [flags]")
	}
}

// custodyTerminal translates only terminal prompts for the command's selected
// operation. It passes no root, lease, journal, decision or runtime authority.
type custodyTerminal struct{ input custody.SecretInput }

func (terminal custodyTerminal) ReadSecret(ctx context.Context, prompt newcustody.SecretPrompt) ([]byte, error) {
	if terminal.input == nil {
		return nil, newcustody.ErrInvalid
	}
	return terminal.input.ReadSecret(ctx, custody.SecretPrompt(prompt))
}
func (terminal custodyTerminal) Confirm(ctx context.Context, prompt newcustody.ConfirmationPrompt) (bool, error) {
	if terminal.input == nil {
		return false, newcustody.ErrInvalid
	}
	return terminal.input.Confirm(ctx, custody.ConfirmationPrompt(prompt))
}
func (terminal custodyTerminal) ReadAdmissionRequestCommitment(ctx context.Context) ([32]byte, error) {
	input, ok := terminal.input.(admissionRequestCommitmentInput)
	if !ok {
		return [32]byte{}, newcustody.ErrInvalid
	}
	return input.ReadAdmissionRequestCommitment(ctx)
}
