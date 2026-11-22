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
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/dianabuilds/ardents-network/internal/enrollment"
	"github.com/dianabuilds/ardents-network/internal/release/infrastructure"
)

// Forwarding has a separate closed artifact consumer. The shared native input
// and output mechanisms supply physical custody only, never Source permission.
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
	case "prepare-descriptor", "prepare-inventory", "prepare-release-plan", "prepare-forward-release-plan":
	default:
		return &forwardingControlFailure{errSourceControlInput}
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	raw, err := prepareForwardingArtifact(ctx, args, nil)
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

func prepareForwardingArtifact(ctx context.Context, args []string, observe func(string, *os.File)) (result []byte, returned error) {
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
	var name, destination string
	check := inputs.checkProtected
	switch args[0] {
	case "prepare-release-plan":
		result, destination, returned = forwardingReleasePlanArguments(args[1:], inputs)
		name = "forwarding-release-plan.json"
	case "prepare-forward-release-plan":
		result, destination, returned = forwardingReleasePurposeArguments(args[1:], inputs, forwardingReleaseForward)
		name = "forwarding-release-plan.json"
	case "prepare-descriptor":
		if len(args) != 6 {
			return nil, errSourceControlInput
		}
		version, err := strconv.ParseInt(args[2], 10, 64)
		if err != nil || strconv.FormatInt(version, 10) != args[2] {
			return nil, errSourceControlInput
		}
		program, err := inputs.read(args[3], 64<<20)
		if err != nil {
			return nil, err
		}
		template, err := inputs.read(args[4], 64<<20)
		if err != nil {
			return nil, err
		}
		result, returned = infrastructure.PrepareForwardingDescriptor(args[1], version, program, template)
		name, destination = infrastructure.ForwardingDescriptorName, args[5]
	case "prepare-inventory":
		if len(args) != 3 || !sourceControlPath(args[1]) || args[1] == args[2] || strings.HasPrefix(args[2], args[1]+"/") {
			return nil, errSourceControlInput
		}
		directory, err := inputs.directory(args[1])
		if err != nil {
			return nil, err
		}
		wanted := map[string]int64{infrastructure.ProgramName: 64 << 20, infrastructure.ForwardingTemplateName: 64 << 20, infrastructure.ForwardingDescriptorName: 16 << 10,
			"root.json": 1 << 20, "timestamp.json": 1 << 20, "snapshot.json": 1 << 20, "targets.json": 1 << 20}
		inventory := func() error {
			if _, err := directory.file.Seek(0, io.SeekStart); err != nil {
				return err
			}
			entries, err := directory.file.ReadDir(8)
			if err != nil && err != io.EOF || len(entries) != 7 {
				return errSourceControlInput
			}
			for _, entry := range entries {
				if _, ok := wanted[entry.Name()]; !ok {
					return errSourceControlInput
				}
			}
			return nil
		}
		if err := inventory(); err != nil {
			return nil, err
		}
		files := make(map[string][]byte, 7)
		for filename, maximum := range wanted {
			files[filename], err = inputs.read(filepath.Join(args[1], filename), maximum)
			if err != nil {
				return nil, err
			}
		}
		d, err := infrastructure.ValidateForwardingResources(files[infrastructure.ForwardingDescriptorName], files[infrastructure.ProgramName], files[infrastructure.ForwardingTemplateName])
		if err != nil {
			return nil, err
		}
		result, returned = enrollment.PrepareInfrastructureInventory(enrollment.InfrastructureFacts{Platform: d.Platform, ReleaseIdentity: d.ReleaseIdentity, ReleaseVersion: d.ReleaseVersion, Invocation: d.Invocation}, files)
		name, destination = enrollment.InfrastructureInventoryName, args[2]
		check = func() error { return errors.Join(inputs.checkProtected(), inventory()) }
	default:
		return nil, errSourceControlInput
	}
	if returned != nil {
		return nil, returned
	}
	if returned = persistSourceArtifacts(ctx, destination, []sourceArtifactFile{{name, result}}, check, observe); returned != nil {
		return nil, returned
	}
	return bytes.Clone(result), check()
}
