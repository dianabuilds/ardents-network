package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/dianabuilds/ardents-network/internal/entry"
)

// runEntryImport adapts a single signed Entry Invite into the retained
// operator command. Entry retains validation and replay-state ownership; this
// command only selects the explicit import operation and renders its receipt.
func runEntryImport(ctx context.Context, arguments []string, output io.Writer) (runErr error) {
	if len(arguments) != 3 || arguments[1] != "import" || arguments[2] == "" {
		return errors.New("usage: ardents entry import <entry-import-plan.json>")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	runtime, err := loadImportPlan(arguments[2], time.Now)
	if err != nil {
		return fmt.Errorf("load import plan: %w", err)
	}
	defer func() { runErr = errors.Join(runErr, runtime.close()) }()
	invite, err := readOperatorInput(runtime.inviteFile, 4096)
	if err != nil && !errors.Is(err, errOperatorInputTooLarge) {
		return fmt.Errorf("read Bridge Invite: %w", err)
	}
	owner, err := entry.Open(runtime.config)
	if err != nil {
		return fmt.Errorf("open Entry state: %w", err)
	}
	result, importErr := owner.Import(invite)
	closeErr := owner.Close()
	if importErr != nil {
		return errors.Join(fmt.Errorf("import Entry Invite: %w", importErr), closeErr)
	}
	if closeErr != nil {
		return fmt.Errorf("close Bridge state: %w", closeErr)
	}
	encoder := json.NewEncoder(output)
	encoder.SetEscapeHTML(false)
	return encoder.Encode(result)
}
