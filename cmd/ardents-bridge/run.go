package main

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/dianabuilds/ardents-network/internal/bridge"
	"github.com/dianabuilds/ardents-network/internal/planfile"
)

func run(ctx context.Context, arguments []string, output io.Writer) (runErr error) {
	if len(arguments) != 2 || arguments[0] != "import" && arguments[0] != "serve" {
		return errors.New("usage: ardents-bridge import|serve <plan>")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if arguments[0] == "serve" {
		return runServe(ctx, arguments[1], output)
	}
	runtime, err := loadImportPlan(arguments[1], time.Now)
	if err != nil {
		return fmt.Errorf("load import plan: %w", err)
	}
	defer func() { runErr = errors.Join(runErr, runtime.close()) }()
	invite, err := planfile.Read(runtime.inviteFile, 4096)
	if err != nil {
		return fmt.Errorf("read Bridge Invite: %w", err)
	}
	runtime.config.ValidateCandidate = candidateCommitment
	owner, err := bridge.Open(runtime.config)
	if err != nil {
		return fmt.Errorf("open Bridge state: %w", err)
	}
	result, importErr := owner.Import(invite)
	closeErr := owner.Close()
	if importErr != nil {
		return errors.Join(fmt.Errorf("import Bridge Invite: %w", importErr), closeErr)
	}
	if closeErr != nil {
		return fmt.Errorf("close Bridge state: %w", closeErr)
	}
	encoder := json.NewEncoder(output)
	encoder.SetEscapeHTML(false)
	return encoder.Encode(struct {
		Class      string `json:"class"`
		InviteID   string `json:"invite_id"`
		Slot       uint8  `json:"slot"`
		Generation uint8  `json:"generation"`
	}{string(result.Class), hex.EncodeToString(result.InviteID[:]), result.Slot, result.Generation})
}
