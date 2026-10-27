package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func runForwardingEpochProvision(args []string, out io.Writer) error {
	if len(args) == 0 || os.Geteuid() != 0 {
		return &forwardingControlFailure{errSourceControlInput}
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	if args[0] == "prepare-state-key" {
		if len(args) != 2 {
			return &forwardingControlFailure{errSourceControlInput}
		}
		public, err := prepareForwardingPrivateKey(ctx, args[1], "state-key.pem", nil)
		if err != nil {
			return &forwardingControlFailure{err}
		}
		raw, err := json.Marshal(struct {
			Operation string `json:"operation"`
			Public    string `json:"state_public_key"`
			State     bool   `json:"state_accepted"`
			Installed bool   `json:"installed_qualification"`
			Runtime   bool   `json:"runtime_authority"`
		}{"forwarding.prepare-state-key", hex.EncodeToString(public[:]), false, false, false})
		if err != nil {
			return err
		}
		return writeSourceControlBytes(ctx, out, append(raw, '\n'))
	}
	var artifact []byte
	var err error
	check := func() error { return ctx.Err() }
	switch args[0] {
	case "prepare-epoch":
		var plan forwardingEpochPlan
		plan, artifact, err = prepareForwardingEpoch(ctx, args[1:], nil)
		check = func() error { return errors.Join(plan.current(time.Now().UTC()), ctx.Err()) }
	case "sign-epoch":
		var signed *forwardingEpochResult
		signed, err = signForwardingEpoch(ctx, args[1:], nil)
		if err == nil {
			artifact = signed.bundle
			check = func() error { return errors.Join(signed.plan.current(time.Now().UTC()), ctx.Err()) }
		}
	default:
		return &forwardingControlFailure{errSourceControlInput}
	}
	if err != nil {
		return &forwardingControlFailure{err}
	}
	if err := check(); err != nil {
		return &forwardingControlFailure{err}
	}
	sum := sha256.Sum256(artifact)
	raw, err := json.Marshal(struct {
		Operation string `json:"operation"`
		SHA       string `json:"sha256"`
		State     bool   `json:"state_accepted"`
		Installed bool   `json:"installed_qualification"`
		Runtime   bool   `json:"runtime_authority"`
	}{"forwarding." + args[0], hex.EncodeToString(sum[:]), false, false, false})
	if err != nil {
		return err
	}
	return errors.Join(writeSourceControlBytes(ctx, out, append(raw, '\n')), check())
}
