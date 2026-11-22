package main

import (
	"bytes"
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

type forwardingControlFailure struct{ cause error }

func (e *forwardingControlFailure) Error() string { return "forwarding artifact preparation refused" }
func (e *forwardingControlFailure) Unwrap() error { return e.cause }

func runForwardingControl(args []string, out io.Writer) error {
	if len(args) == 0 {
		return &forwardingControlFailure{errSourceControlInput}
	}
	switch args[0] {
	case "prepare-profile", "sign-profile":
		return runForwardingProfileProvision(args, out)
	case "prepare-token-keys":
		return runForwardingTokenMaterial(args[1:], out)
	case "prepare-state-key", "prepare-epoch", "sign-epoch":
		return runForwardingEpochProvision(args, out)
	case "prepare-node-key", "prepare-node-record", "sign-node-record", "prepare-node-certificate":
		return runForwardingNodeProvision(args, out)
	case "prepare-release-keys":
		return runForwardingReleaseKeys(args[1:], out)
	case "sign-initial-release":
		return runForwardingReleaseSigning(args[1:], out)
	case "sign-forward-release":
		return runForwardingReleasePurpose(args[1:], out, forwardingReleaseForward)
	case "prepare-release-plan", "prepare-forward-release-plan":
	default:
		return &forwardingControlFailure{errSourceControlInput}
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	raw, err := writeForwardingReleasePlan(ctx, args)
	if err != nil {
		return &forwardingControlFailure{err}
	}
	sum := sha256.Sum256(raw)
	receipt, err := json.Marshal(struct {
		Operation string `json:"operation"`
		SHA256    string `json:"sha256"`
	}{"forwarding." + args[0], hex.EncodeToString(sum[:])})
	if err != nil {
		return err
	}
	return writeSourceControlBytes(ctx, out, append(receipt, '\n'))
}

func writeForwardingReleasePlan(ctx context.Context, args []string) (result []byte, returned error) {
	if ctx == nil || ctx.Err() != nil || os.Geteuid() != 0 || len(args) == 0 {
		return nil, errSourceControlInput
	}
	inputs := &sourceControlInputs{ctx: ctx}
	defer func() {
		returned = errors.Join(returned, inputs.close(), ctx.Err())
		if returned != nil {
			result = nil
		}
	}()
	var destination string
	switch args[0] {
	case "prepare-release-plan":
		result, destination, returned = forwardingReleasePlanArguments(args[1:], inputs)
	case "prepare-forward-release-plan":
		result, destination, returned = forwardingReleasePurposeArguments(args[1:], inputs, forwardingReleaseForward)
	default:
		return nil, errSourceControlInput
	}
	if returned != nil {
		return nil, returned
	}
	if returned = persistSourceArtifacts(ctx, destination, []sourceArtifactFile{{"forwarding-release-plan.json", result}}, inputs.checkProtected, nil); returned != nil {
		return nil, returned
	}
	return bytes.Clone(result), inputs.checkProtected()
}
