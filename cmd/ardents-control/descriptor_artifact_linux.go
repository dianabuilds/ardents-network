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
	"strconv"
	"syscall"
	"time"

	"github.com/dianabuilds/ardents-network/internal/enrollment"
	"github.com/dianabuilds/ardents-network/internal/release/infrastructure"
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
	case "prepare-artifact", "prepare-inventory", "prepare-release-plan", "prepare-forward-release-plan":
	default:
		return &descriptorControlFailure{errSourceControlInput}
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	raw, err := prepareDescriptorArtifact(ctx, args, nil)
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

func prepareDescriptorArtifact(ctx context.Context, args []string, observe func(string, *os.File)) (result []byte, returned error) {
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
	case "prepare-forward-release-plan":
		result, destination, returned = descriptorReleasePurposeArguments(args[1:], inputs, descriptorReleaseForward)
		name = "descriptor-release-plan.json"
	case "prepare-release-plan":
		result, destination, returned = descriptorReleasePlanArguments(args[1:], inputs)
		name = "descriptor-release-plan.json"
	case "prepare-artifact":
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
		result, returned = infrastructure.PrepareDescriptorArtifact(args[1], version, program, template)
		name, destination = infrastructure.DescriptorArtifactName, args[5]
	case "prepare-inventory":
		if len(args) != 3 {
			return nil, errSourceControlInput
		}
		files, checkInventory, err := inputs.readInfrastructureInventory(args[1], args[2],
			infrastructure.ProgramName, infrastructure.DescriptorTemplateName, infrastructure.DescriptorArtifactName)
		if err != nil {
			return nil, err
		}
		d, err := infrastructure.ValidateDescriptorResources(files[infrastructure.DescriptorArtifactName], files[infrastructure.ProgramName], files[infrastructure.DescriptorTemplateName])
		if err != nil {
			return nil, err
		}
		result, returned = enrollment.PrepareInfrastructureInventory(enrollment.InfrastructureFacts{Platform: d.Platform, ReleaseIdentity: d.ReleaseIdentity, ReleaseVersion: d.ReleaseVersion, Invocation: d.Invocation}, files)
		name, destination = enrollment.InfrastructureInventoryName, args[2]
		check = checkInventory
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
