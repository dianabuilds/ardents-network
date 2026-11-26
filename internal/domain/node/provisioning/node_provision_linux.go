package provisioning

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"github.com/dianabuilds/ardents-network/internal/canonicaldigest"
	"github.com/dianabuilds/ardents-network/internal/platform/artifacts"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/dianabuilds/ardents-network/internal/domain/network/epoch"
)

func runNodeProvision(role nodeProvisioningRole, args []string, out io.Writer) error {
	if !role.valid() || len(args) == 0 || os.Geteuid() != 0 {
		return role.failure(artifacts.ErrInput)
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	if args[0] == "prepare-node-key" {
		if len(args) != 2 {
			return role.failure(artifacts.ErrInput)
		}
		public, err := prepareProvisioningKey(ctx, args[1], "node-key.pem", nil)
		if err != nil {
			return role.failure(err)
		}
		raw, err := json.Marshal(struct {
			Operation string `json:"operation"`
			Public    string `json:"node_public_key"`
			State     bool   `json:"state_accepted"`
			Installed bool   `json:"installed_qualification"`
			Runtime   bool   `json:"runtime_authority"`
		}{role.operation("prepare-node-key"), hex.EncodeToString(public[:]), false, false, false})
		if err != nil {
			return err
		}
		return artifacts.WriteReceipt(ctx, out, append(raw, '\n'))
	}
	var result []byte
	var err error
	finalCheck := func() error { return ctx.Err() }
	if args[0] == "prepare-node-record" {
		var plan nodeRecordPlan
		var output string
		plan, output, err = nodeRecordArguments(role, args[1:], time.Now().UTC())
		finalCheck = func() error { return errors.Join(plan.current(role, time.Now().UTC()), ctx.Err()) }
		if err == nil {
			result, err = plan.prepare(role, time.Now().UTC())
		}
		if err == nil {
			var raw []byte
			raw, err = json.Marshal(plan)
			if err == nil {
				err = artifacts.Publish(ctx, output, []artifacts.Content{{Name: "node-record-plan.json", Data: append(raw, '\n')}, {Name: "node-record.signing-input", Data: result}}, func() error { return plan.current(role, time.Now().UTC()) }, nil)
			}
		}
	} else {
		var signed *nodeProvisionResult
		signed, err = prepareNodeSigned(ctx, role, args, nil)
		if err == nil {
			result = signed.artifact
			finalCheck = func() error { return errors.Join(signed.plan.current(role, time.Now().UTC()), ctx.Err()) }
		}
	}
	if err != nil {
		return role.failure(err)
	}
	if err := finalCheck(); err != nil {
		return role.failure(err)
	}
	sum := sha256.Sum256(result)
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
	return errors.Join(artifacts.WriteReceipt(ctx, out, append(raw, '\n')), finalCheck())
}

type nodeProvisionResult struct {
	artifact []byte
	plan     nodeRecordPlan
}

func prepareNodeSigned(ctx context.Context, role nodeProvisioningRole, args []string, observe func(string, *os.File)) (result *nodeProvisionResult, returned error) {
	if !role.valid() || ctx == nil || ctx.Err() != nil || os.Geteuid() != 0 || len(args) != 6 || args[0] != "sign-node-record" && args[0] != "prepare-node-certificate" {
		return nil, artifacts.ErrInput
	}
	for _, index := range []int{1, 2, 4, 5} {
		if !artifacts.AbsolutePath(args[index]) {
			return nil, artifacts.ErrInput
		}
		for _, other := range []int{1, 2, 4, 5} {
			if index != other && (args[index] == args[other] || index == 5 && strings.HasPrefix(args[other], args[index]+string(filepath.Separator))) {
				return nil, artifacts.ErrInput
			}
		}
	}
	if _, ok := canonicaldigest.Decode(args[3]); !ok {
		return nil, artifacts.ErrInput
	}
	inputs := artifacts.NewInputs(ctx)
	var admitted *nodeRecordPlan
	defer func() {
		returned = errors.Join(returned, inputs.Close(), ctx.Err())
		if admitted != nil {
			returned = errors.Join(returned, admitted.current(role, time.Now().UTC()))
		}
		if returned != nil {
			result = nil
		}
	}()
	planRaw, err := inputs.Read(args[1], 16<<10)
	if err != nil {
		return nil, err
	}
	plan, message, err := decodeNodeRecordPlan(role, planRaw, time.Now().UTC())
	if err != nil {
		return nil, err
	}
	admitted = &plan
	prepared, err := inputs.Read(args[2], epoch.MaxRecordBytes)
	if err != nil {
		return nil, err
	}
	sum := sha256.Sum256(prepared)
	if hex.EncodeToString(sum[:]) != args[3] {
		return nil, artifacts.ErrInput
	}
	var facts epoch.InitialClosedRecord
	if args[0] == "sign-node-record" {
		if !bytes.Equal(prepared, message) {
			return nil, artifacts.ErrInput
		}
	} else {
		facts, err = epoch.VerifyInitialClosedRecord(message, prepared)
		if err != nil || facts.Carrier != epoch.CarrierClosedTCP {
			return nil, artifacts.ErrInput
		}
	}
	check := func() error {
		if err := plan.current(role, time.Now().UTC()); err != nil {
			return err
		}
		return inputs.CheckProtected()
	}
	if err := check(); err != nil {
		return nil, err
	}
	keyRaw, err := inputs.Read(args[4], 4096)
	if err != nil {
		return nil, err
	}
	info := inputs.Last().Info()
	if info.Mode() != 0600 || info.Sys().(*syscall.Stat_t).Uid != 0 {
		return nil, artifacts.ErrInput
	}
	key, err := artifacts.ParseEd25519Key(keyRaw)
	if err != nil {
		return nil, err
	}
	defer clear(key)
	public, err := controlPublic32(plan.PublicKey)
	if err != nil || !bytes.Equal(public[:], key[32:]) {
		return nil, artifacts.ErrInput
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
		artifact, err = createNodeCertificate(facts, key)
		if err != nil {
			return nil, err
		}
		name = "node-certificate.pem"
	}
	if err := check(); err != nil {
		return nil, err
	}
	if err := artifacts.Publish(ctx, args[5], []artifacts.Content{{Name: name, Data: artifact}}, check, observe); err != nil {
		return nil, err
	}
	return &nodeProvisionResult{bytes.Clone(artifact), plan}, check()
}
