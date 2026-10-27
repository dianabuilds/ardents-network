package main

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/network/epoch"
)

func runForwardingNodeProvision(args []string, out io.Writer) error {
	if len(args) == 0 || os.Geteuid() != 0 {
		return &forwardingControlFailure{errSourceControlInput}
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	if args[0] == "prepare-node-key" {
		if len(args) != 2 {
			return &forwardingControlFailure{errSourceControlInput}
		}
		public, err := prepareForwardingNodeKey(ctx, args[1], nil)
		if err != nil {
			return &forwardingControlFailure{err}
		}
		raw, err := json.Marshal(struct {
			Operation string `json:"operation"`
			Public    string `json:"node_public_key"`
			State     bool   `json:"state_accepted"`
			Installed bool   `json:"installed_qualification"`
			Runtime   bool   `json:"runtime_authority"`
		}{"forwarding.prepare-node-key", hex.EncodeToString(public[:]), false, false, false})
		if err != nil {
			return err
		}
		return writeSourceControlBytes(ctx, out, append(raw, '\n'))
	}
	var result []byte
	var err error
	finalCheck := func() error { return ctx.Err() }
	if args[0] == "prepare-node-record" {
		var plan forwardingRecordPlan
		var output string
		plan, output, err = forwardingRecordArguments(args[1:], time.Now().UTC())
		finalCheck = func() error { return errors.Join(plan.current(time.Now().UTC()), ctx.Err()) }
		if err == nil {
			result, err = plan.prepare(time.Now().UTC())
		}
		if err == nil {
			var raw []byte
			raw, err = json.Marshal(plan)
			if err == nil {
				err = persistSourceArtifacts(ctx, output, []sourceArtifactFile{{"node-record-plan.json", append(raw, '\n')}, {"node-record.signing-input", result}}, func() error { return plan.current(time.Now().UTC()) }, nil)
			}
		}
	} else {
		var signed *forwardingNodeProvisionResult
		signed, err = prepareForwardingNodeSigned(ctx, args, nil)
		if err == nil {
			result = signed.artifact
			finalCheck = func() error { return errors.Join(signed.plan.current(time.Now().UTC()), ctx.Err()) }
		}
	}
	if err != nil {
		return &forwardingControlFailure{err}
	}
	if err := finalCheck(); err != nil {
		return &forwardingControlFailure{err}
	}
	sum := sha256.Sum256(result)
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
	return errors.Join(writeSourceControlBytes(ctx, out, append(raw, '\n')), finalCheck())
}

type forwardingNodeProvisionResult struct {
	artifact []byte
	plan     forwardingRecordPlan
}

func prepareForwardingNodeSigned(ctx context.Context, args []string, observe func(string, *os.File)) (result *forwardingNodeProvisionResult, returned error) {
	if ctx == nil || ctx.Err() != nil || os.Geteuid() != 0 || len(args) != 6 || args[0] != "sign-node-record" && args[0] != "prepare-node-certificate" {
		return nil, errSourceControlInput
	}
	for _, index := range []int{1, 2, 4, 5} {
		if !sourceControlPath(args[index]) {
			return nil, errSourceControlInput
		}
		for _, other := range []int{1, 2, 4, 5} {
			if index != other && (args[index] == args[other] || index == 5 && strings.HasPrefix(args[other], args[index]+string(filepath.Separator))) {
				return nil, errSourceControlInput
			}
		}
	}
	pin, err := hex.DecodeString(args[3])
	if err != nil || len(pin) != 32 || hex.EncodeToString(pin) != args[3] {
		return nil, errSourceControlInput
	}
	inputs := &sourceControlInputs{ctx: ctx}
	var admitted *forwardingRecordPlan
	defer func() {
		returned = errors.Join(returned, inputs.close(), ctx.Err())
		if admitted != nil {
			returned = errors.Join(returned, admitted.current(time.Now().UTC()))
		}
		if returned != nil {
			result = nil
		}
	}()
	planRaw, err := inputs.read(args[1], 16<<10)
	if err != nil {
		return nil, err
	}
	plan, message, err := decodeForwardingRecordPlan(planRaw, time.Now().UTC())
	if err != nil {
		return nil, err
	}
	admitted = &plan
	prepared, err := inputs.read(args[2], epoch.MaxRecordBytes)
	if err != nil {
		return nil, err
	}
	sum := sha256.Sum256(prepared)
	if hex.EncodeToString(sum[:]) != args[3] {
		return nil, errSourceControlInput
	}
	var facts epoch.InitialClosedRecord
	if args[0] == "sign-node-record" {
		if !bytes.Equal(prepared, message) {
			return nil, errSourceControlInput
		}
	} else {
		facts, err = epoch.VerifyInitialClosedRecord(message, prepared)
		if err != nil || facts.Carrier != epoch.CarrierClosedTCP {
			return nil, errSourceControlInput
		}
	}
	check := func() error {
		if err := plan.current(time.Now().UTC()); err != nil {
			return err
		}
		return inputs.checkProtected()
	}
	if err := check(); err != nil {
		return nil, err
	}
	keyRaw, err := inputs.read(args[4], 4096)
	if err != nil {
		return nil, err
	}
	info := inputs.files[len(inputs.files)-1].info
	if info.Mode() != 0600 || info.Sys().(*syscall.Stat_t).Uid != 0 {
		return nil, errSourceControlInput
	}
	key, err := parseSourceSigningKey(keyRaw)
	if err != nil {
		return nil, err
	}
	defer clear(key)
	public, err := forwardingPublic32(plan.PublicKey)
	if err != nil || !bytes.Equal(public[:], key[32:]) {
		return nil, errSourceControlInput
	}
	if err := check(); err != nil {
		return nil, err
	}
	name := "node-record.bin"
	var artifact []byte
	if args[0] == "sign-node-record" {
		artifact = append(bytes.Clone(message), ed25519.Sign(key, message)...)
		if _, err := epoch.VerifyInitialClosedRecord(message, artifact); err != nil {
			return nil, err
		}
	} else {
		artifact, err = createForwardingNodeCertificate(facts, key)
		if err != nil {
			return nil, err
		}
		name = "node-certificate.pem"
	}
	if err := check(); err != nil {
		return nil, err
	}
	if err := persistSourceArtifacts(ctx, args[5], []sourceArtifactFile{{name, artifact}}, check, observe); err != nil {
		return nil, err
	}
	return &forwardingNodeProvisionResult{bytes.Clone(artifact), plan}, check()
}
