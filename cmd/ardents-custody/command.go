package main

import (
	"context"
	"errors"
	"io"

	"github.com/dianabuilds/ardents-network/internal/custody"
)

func run(ctx context.Context, arguments []string, output io.Writer, input custody.SecretInput) error {
	if len(arguments) == 0 {
		return errors.New("usage: ardents-custody <inspect-envelope|verify-record|export-recovery-bundle|restore-recovery-bundle|purge-record> [flags]")
	}
	switch arguments[0] {
	case "inspect-envelope":
		return inspectEnvelope(ctx, arguments[1:], output)
	case "verify-record":
		return verifyRecord(ctx, arguments[1:], output, input)
	case "export-recovery-bundle", "restore-recovery-bundle":
		return recoveryBundle(ctx, arguments[0], arguments[1:], output, input)
	case "purge-record":
		return purgeRecord(ctx, arguments[1:], output, input)
	default:
		return errors.New("usage: ardents-custody <inspect-envelope|verify-record|export-recovery-bundle|restore-recovery-bundle|purge-record> [flags]")
	}
}
