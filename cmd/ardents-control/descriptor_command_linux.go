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

type descriptorControlFailure struct{ cause error }

func (e *descriptorControlFailure) Error() string { return "Descriptor artifact preparation refused" }
func (e *descriptorControlFailure) Unwrap() error { return e.cause }

func runDescriptorControl(args []string, out io.Writer) error {
	if len(args) == 0 {
		return &descriptorControlFailure{errSourceControlInput}
	}
	switch args[0] {
	case "prepare-profile", "sign-profile":
		return runDescriptorProfileProvision(args, out)
	case "prepare-token-keys":
		return runDescriptorTokenMaterial(args[1:], out)
	case "prepare-state-key", "prepare-epoch", "sign-epoch":
		return runDescriptorEpochProvision(args, out)
	case "prepare-node-key", "prepare-node-record", "sign-node-record", "prepare-node-certificate":
		return runDescriptorNodeProvision(args, out)
	case "prepare-release-keys":
		return runDescriptorReleaseKeys(args[1:], out)
	case "sign-initial-release":
		return runDescriptorReleaseSigning(args[1:], out)
	case "sign-forward-release":
		return runDescriptorReleasePurpose(args[1:], out, descriptorReleaseForward)
	case "prepare-release-plan", "prepare-forward-release-plan":
	default:
		return &descriptorControlFailure{errSourceControlInput}
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	raw, err := writeDescriptorReleasePlan(ctx, args)
	if err != nil {
		return &descriptorControlFailure{err}
	}
	sum := sha256.Sum256(raw)
	receipt, err := json.Marshal(struct {
		Operation string `json:"operation"`
		SHA256    string `json:"sha256"`
	}{"descriptor." + args[0], hex.EncodeToString(sum[:])})
	if err != nil {
		return err
	}
	return writeSourceControlBytes(ctx, out, append(receipt, '\n'))
}

func writeDescriptorReleasePlan(ctx context.Context, args []string) (result []byte, returned error) {
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
	case "prepare-forward-release-plan":
		result, destination, returned = descriptorReleasePurposeArguments(args[1:], inputs, descriptorReleaseForward)
	case "prepare-release-plan":
		result, destination, returned = descriptorReleasePlanArguments(args[1:], inputs)
	default:
		return nil, errSourceControlInput
	}
	if returned != nil {
		return nil, returned
	}
	if returned = persistSourceArtifacts(ctx, destination, []sourceArtifactFile{{"descriptor-release-plan.json", result}}, inputs.checkProtected, nil); returned != nil {
		return nil, returned
	}
	return bytes.Clone(result), inputs.checkProtected()
}
