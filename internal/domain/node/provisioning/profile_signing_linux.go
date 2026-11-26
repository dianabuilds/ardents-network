package provisioning

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"github.com/dianabuilds/ardents-network/internal/platform/artifacts"
	"io"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/dianabuilds/ardents-network/internal/domain/network/closedprofile"
)

// Only this fixed profile purpose may use the original matching State key.
// It authenticates a detached artifact, never accepts State or opens a runtime.
func signNodeProfile(ctx context.Context, role nodeProvisioningRole, args []string, observe func(string, *os.File)) (plan nodeProfilePlan, signed []byte, returned error) {
	if !role.valid() || ctx == nil || ctx.Err() != nil || os.Geteuid() != 0 || len(args) != 5 {
		return plan, nil, artifacts.ErrInput
	}
	for _, index := range []int{0, 1, 3, 4} {
		if !artifacts.AbsolutePath(args[index]) {
			return plan, nil, artifacts.ErrInput
		}
		for _, other := range []int{0, 1, 3, 4} {
			if index != other && (args[index] == args[other] || index == 4 && strings.HasPrefix(args[other], args[index]+"/")) {
				return plan, nil, artifacts.ErrInput
			}
		}
	}
	if _, err := controlPublic32(args[2]); err != nil {
		return plan, nil, err
	}
	inputs := artifacts.NewInputs(ctx)
	var admitted bool
	defer func() {
		returned = errors.Join(returned, inputs.Close(), ctx.Err())
		if admitted {
			returned = errors.Join(returned, plan.current(role, time.Now().UTC()))
		}
		if returned != nil {
			signed = nil
		}
	}()
	raw, err := inputs.Read(args[0], maximumNodeProfilePlan)
	if err != nil {
		return plan, nil, err
	}
	var prepared nodeProfilePreparation
	plan, prepared, err = decodeNodeProfilePlan(role, raw, time.Now().UTC())
	if err != nil {
		return plan, nil, err
	}
	admitted = true
	actual, err := inputs.Read(args[1], closedprofile.MaxSize)
	if err != nil {
		return plan, nil, err
	}
	sum := sha256.Sum256(prepared.body)
	if !bytes.Equal(actual, prepared.body) || args[2] != hex.EncodeToString(sum[:]) {
		return plan, nil, artifacts.ErrInput
	}
	check := func() error {
		return errors.Join(plan.current(role, time.Now().UTC()), inputs.CheckProtected(), ctx.Err())
	}
	if err := check(); err != nil {
		return plan, nil, err
	}
	keyRaw, err := inputs.Read(args[3], 4096)
	if err != nil {
		return plan, nil, err
	}
	private := inputs.Last()
	if private.Info().Mode() != 0600 || private.Info().Sys().(*syscall.Stat_t).Uid != 0 {
		return plan, nil, artifacts.ErrInput
	}
	key, err := artifacts.ParseEd25519Key(keyRaw)
	if err != nil {
		return plan, nil, err
	}
	defer clear(key)
	if !bytes.Equal(key[32:], prepared.context.Authority) {
		return plan, nil, artifacts.ErrInput
	}
	if err := check(); err != nil {
		return plan, nil, err
	}
	prepared.context.Now = time.Now().UTC()
	transcript, err := closedprofile.SigningTranscript(prepared.body, prepared.context)
	if err != nil {
		return plan, nil, err
	}
	signed = append(bytes.Clone(prepared.body), ed25519.Sign(key, transcript)...)
	profile, err := closedprofile.Verify(signed, prepared.context)
	if err != nil {
		return plan, nil, err
	}
	if err := prepared.bind(profile.Digest); err != nil {
		return plan, nil, err
	}
	if observe != nil {
		observe("signed", private.Descriptor())
	}
	if err := check(); err != nil {
		return plan, nil, err
	}
	if err := artifacts.Publish(ctx, args[4], []artifacts.Content{{Name: "profile.bin", Data: signed}}, check, observe); err != nil {
		return plan, nil, err
	}
	return plan, signed, check()
}

func runNodeProfileProvision(role nodeProvisioningRole, args []string, out io.Writer) error {
	if !role.valid() || len(args) == 0 || os.Geteuid() != 0 {
		return role.failure(artifacts.ErrInput)
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	var plan nodeProfilePlan
	var artifact []byte
	var err error
	switch args[0] {
	case "prepare-profile":
		plan, artifact, err = prepareNodeProfile(ctx, role, args[1:], nil)
	case "sign-profile":
		plan, artifact, err = signNodeProfile(ctx, role, args[1:], nil)
	default:
		return role.failure(artifacts.ErrInput)
	}
	if err != nil {
		return role.failure(err)
	}
	check := func() error { return errors.Join(plan.current(role, time.Now().UTC()), ctx.Err()) }
	if err := check(); err != nil {
		return role.failure(err)
	}
	sum := sha256.Sum256(artifact)
	raw, err := json.Marshal(struct {
		Operation string `json:"operation"`
		SHA       string `json:"sha256"`
		State     bool   `json:"state_accepted"`
		Runtime   bool   `json:"runtime_authority"`
		Installed bool   `json:"installed_qualification"`
	}{role.operation(args[0]), hex.EncodeToString(sum[:]), false, false, false})
	if err != nil {
		return err
	}
	return errors.Join(artifacts.WriteReceipt(ctx, out, append(raw, '\n')), check())
}
