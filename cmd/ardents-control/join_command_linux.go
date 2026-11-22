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

type joinControlFailure struct{ cause error }

func (e *joinControlFailure) Error() string { return "JOIN artifact preparation refused" }
func (e *joinControlFailure) Unwrap() error { return e.cause }

// JOIN has its own closed public producers. Shared native readers/writers own
// only physical input/output custody; no Source or Forwarding right is reused.
func runJoinControl(args []string, out io.Writer) error {
	if len(args) == 0 {
		return &joinControlFailure{errSourceControlInput}
	}
	switch args[0] {
	case "prepare-profile", "sign-profile":
		return runJoinProfileProvision(args, out)
	case "prepare-token-keys":
		return runJoinTokenMaterial(args[1:], out)
	case "prepare-state-key", "prepare-epoch", "sign-epoch":
		return runJoinEpochProvision(args, out)
	case "prepare-node-key", "prepare-node-record", "sign-node-record", "prepare-node-certificate":
		return runJoinNodeProvision(args, out)
	case "prepare-release-keys":
		return runJoinReleaseKeys(args[1:], out)
	case "sign-initial-release":
		return runJoinReleaseSigning(args[1:], out)
	case "sign-forward-release":
		return runJoinReleasePurpose(args[1:], out, joinReleaseForward)
	case "prepare-release-plan", "prepare-forward-release-plan":
	default:
		return &joinControlFailure{errSourceControlInput}
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	raw, err := writeJoinReleasePlan(ctx, args)
	if err != nil {
		return &joinControlFailure{err}
	}
	sum := sha256.Sum256(raw)
	receipt, err := json.Marshal(struct {
		Operation string `json:"operation"`
		SHA256    string `json:"sha256"`
	}{"join." + args[0], hex.EncodeToString(sum[:])})
	if err != nil {
		return err
	}
	return writeSourceControlBytes(ctx, out, append(receipt, '\n'))
}

func writeJoinReleasePlan(ctx context.Context, args []string) (result []byte, returned error) {
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
		result, destination, returned = joinReleasePlanArguments(args[1:], inputs)
	case "prepare-forward-release-plan":
		result, destination, returned = joinReleasePurposeArguments(args[1:], inputs, joinReleaseForward)
	default:
		return nil, errSourceControlInput
	}
	if returned != nil {
		return nil, returned
	}
	if returned = persistSourceArtifacts(ctx, destination, []sourceArtifactFile{{"join-release-plan.json", result}}, inputs.checkProtected, nil); returned != nil {
		return nil, returned
	}
	return bytes.Clone(result), inputs.checkProtected()
}
