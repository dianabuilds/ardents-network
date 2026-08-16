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
	"github.com/dianabuilds/ardents-network/internal/camouflage"
	"github.com/dianabuilds/ardents-network/internal/planfile"
)

func run(ctx context.Context, arguments []string, output io.Writer) (runErr error) {
	if len(arguments) != 2 || arguments[0] != "import" {
		return errors.New("usage: ardents-bridge import <plan>")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	config, invitePath, closeNetwork, err := loadImportPlan(arguments[1], time.Now)
	if err != nil {
		return fmt.Errorf("load import plan: %w", err)
	}
	defer func() { runErr = errors.Join(runErr, closeNetwork()) }()
	invite, err := planfile.Read(invitePath, 4096)
	if err != nil {
		return fmt.Errorf("read Bridge Invite: %w", err)
	}
	config.ValidateCandidate = func(raw []byte, identity [32]byte) ([32]byte, error) {
		candidate, validateErr := camouflage.Validate(raw, identity)
		return candidate.Commitment(), validateErr
	}
	owner, err := bridge.Open(config)
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
