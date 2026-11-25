package command

import (
	"context"
	"errors"
	"io"
	"os"

	"github.com/dianabuilds/ardents-network/internal/custody"
)

var errUsage = errors.New("usage: ardents-custody <create-service-authority|issue-service-credential|create-admission-authority|issue-admission-permission|verify-record|export-recovery-bundle|restore-recovery-bundle|purge-record> [flags]")

// Run executes the custody command with the supplied terminal and output streams.
func Run(ctx context.Context, arguments []string, output, prompts io.Writer, terminal *os.File) error {
	return run(ctx, arguments, output, terminalSecretInput{terminal: terminal, prompts: prompts})
}

func run(ctx context.Context, arguments []string, output io.Writer, input custody.SecretInput) error {
	if len(arguments) == 0 {
		return errUsage
	}
	switch arguments[0] {
	case "create-service-authority", "issue-service-credential":
		return serviceAuthority(ctx, arguments[0], arguments[1:], output, input)
	case "create-admission-authority", "issue-admission-permission":
		return admissionAuthority(ctx, arguments[0], arguments[1:], output, input)
	case "verify-record":
		return verifyRecord(ctx, arguments[1:], output, input)
	case "export-recovery-bundle", "restore-recovery-bundle":
		return recoveryBundle(ctx, arguments[0], arguments[1:], output, input)
	case "purge-record":
		return purgeRecord(ctx, arguments[1:], output, input)
	default:
		return errUsage
	}
}
