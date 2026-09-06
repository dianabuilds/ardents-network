package main

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/dianabuilds/ardents-network/internal/entry"
)

func runEntry(ctx context.Context, arguments []string, output io.Writer) error {
	if len(arguments) != 3 || arguments[2] == "" {
		return errors.New("usage: ardents entry <recipient|import> <entry-import-plan.json>")
	}
	if arguments[1] == "recipient" {
		return runEntryRecipient(ctx, arguments[2], output)
	}
	if arguments[1] != "import" {
		return errors.New("usage: ardents entry <recipient|import> <entry-import-plan.json>")
	}
	return runEntryImport(ctx, arguments[2], output)
}

// runEntryImport adapts a single signed Entry Invite into the retained
// operator command. Entry retains validation and replay-state ownership; this
// command only selects the explicit import operation and renders its receipt.
func runEntryImport(ctx context.Context, planPath string, output io.Writer) (runErr error) {
	if err := ctx.Err(); err != nil {
		return err
	}
	runtime, err := loadImportPlan(planPath, time.Now)
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

func runEntryRecipient(ctx context.Context, planPath string, output io.Writer) (runErr error) {
	if err := ctx.Err(); err != nil {
		return err
	}
	root, err := loadEntryRecipientRoot(planPath)
	if err != nil {
		return fmt.Errorf("load recipient plan: %w", err)
	}
	recipient, err := entry.RecipientPublicKey(root)
	if err != nil {
		return fmt.Errorf("read Entry recipient: %w", err)
	}
	encoder := json.NewEncoder(output)
	encoder.SetEscapeHTML(false)
	return encoder.Encode(struct {
		RecipientPublicKey string `json:"recipient_public_key"`
	}{RecipientPublicKey: hex.EncodeToString(recipient[:])})
}
