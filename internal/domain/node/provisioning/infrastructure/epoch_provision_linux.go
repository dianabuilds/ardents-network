package infrastructure

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"github.com/dianabuilds/ardents-network/internal/platform/artifacts"
	"io"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func runNodeEpochProvision(role nodeProvisioningRole, args []string, out io.Writer) error {
	if !role.valid() || len(args) == 0 || os.Geteuid() != 0 {
		return role.failure(artifacts.ErrInput)
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	if args[0] == "prepare-state-key" {
		if len(args) != 2 {
			return role.failure(artifacts.ErrInput)
		}
		public, err := prepareProvisioningKey(ctx, args[1], "state-key.pem", nil)
		if err != nil {
			return role.failure(err)
		}
		raw, err := json.Marshal(struct {
			Operation string `json:"operation"`
			Public    string `json:"state_public_key"`
			State     bool   `json:"state_accepted"`
			Installed bool   `json:"installed_qualification"`
			Runtime   bool   `json:"runtime_authority"`
		}{role.operation("prepare-state-key"), hex.EncodeToString(public[:]), false, false, false})
		if err != nil {
			return err
		}
		return artifacts.WriteReceipt(ctx, out, append(raw, '\n'))
	}
	var artifact []byte
	var err error
	check := func() error { return ctx.Err() }
	switch args[0] {
	case "prepare-epoch":
		var plan nodeEpochPlan
		plan, artifact, err = prepareNodeEpoch(ctx, role, args[1:], nil)
		check = func() error { return errors.Join(plan.current(role, time.Now().UTC()), ctx.Err()) }
	case "sign-epoch":
		var signed *nodeEpochResult
		signed, err = signNodeEpoch(ctx, role, args[1:], nil)
		if err == nil {
			artifact = signed.bundle
			check = func() error { return errors.Join(signed.plan.current(role, time.Now().UTC()), ctx.Err()) }
		}
	default:
		return role.failure(artifacts.ErrInput)
	}
	if err != nil {
		return role.failure(err)
	}
	if err := check(); err != nil {
		return role.failure(err)
	}
	sum := sha256.Sum256(artifact)
	raw, err := json.Marshal(struct {
		Operation string `json:"operation"`
		SHA       string `json:"sha256"`
		State     bool   `json:"state_accepted"`
		Installed bool   `json:"installed_qualification"`
		Runtime   bool   `json:"runtime_authority"`
	}{role.operation(args[0]), hex.EncodeToString(sum[:]), false, false, false})
	if err != nil {
		return err
	}
	return errors.Join(artifacts.WriteReceipt(ctx, out, append(raw, '\n')), check())
}
