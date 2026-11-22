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
	"strings"
	"syscall"
	"time"

	"github.com/dianabuilds/ardents-network/internal/network/closedprofile"
)

// nodeProfileSigningPlan is the role hook for the fixed profile ceremony: each
// role keeps its own plan type, schema and current validity policy.
type nodeProfileSigningPlan interface {
	current(time.Time) error
}

// nodeProfileSigningPreparation exposes the detached verified facts the fixed
// profile ceremony signs and binds. The State authority pin and schema stay
// role-bound through the plan that produced the preparation.
type nodeProfileSigningPreparation interface {
	profileBody() []byte
	profileContext() closedprofile.Context
	bind([32]byte) error
}

// Only this fixed profile purpose may use the original matching State key.
// It authenticates a detached artifact, never accepts State or opens a runtime.
func signNodeProfile[P nodeProfileSigningPlan, Q nodeProfileSigningPreparation](ctx context.Context, args []string, observe func(string, *os.File), maximum int, decode func([]byte, time.Time) (P, Q, error)) (plan P, signed []byte, returned error) {
	if ctx == nil || ctx.Err() != nil || os.Geteuid() != 0 || len(args) != 5 {
		return plan, nil, errSourceControlInput
	}
	for _, index := range []int{0, 1, 3, 4} {
		if !sourceControlPath(args[index]) {
			return plan, nil, errSourceControlInput
		}
		for _, other := range []int{0, 1, 3, 4} {
			if index != other && (args[index] == args[other] || index == 4 && strings.HasPrefix(args[other], args[index]+"/")) {
				return plan, nil, errSourceControlInput
			}
		}
	}
	if _, err := controlPublic32(args[2]); err != nil {
		return plan, nil, err
	}
	inputs := &sourceControlInputs{ctx: ctx}
	var admitted bool
	defer func() {
		returned = errors.Join(returned, inputs.close(), ctx.Err())
		if admitted {
			returned = errors.Join(returned, plan.current(time.Now().UTC()))
		}
		if returned != nil {
			signed = nil
		}
	}()
	raw, err := inputs.read(args[0], int64(maximum))
	if err != nil {
		return plan, nil, err
	}
	var prepared Q
	plan, prepared, err = decode(raw, time.Now().UTC())
	if err != nil {
		return plan, nil, err
	}
	admitted = true
	actual, err := inputs.read(args[1], closedprofile.MaxSize)
	if err != nil {
		return plan, nil, err
	}
	sum := sha256.Sum256(prepared.profileBody())
	if !bytes.Equal(actual, prepared.profileBody()) || args[2] != hex.EncodeToString(sum[:]) {
		return plan, nil, errSourceControlInput
	}
	check := func() error { return errors.Join(plan.current(time.Now().UTC()), inputs.checkProtected(), ctx.Err()) }
	if err := check(); err != nil {
		return plan, nil, err
	}
	keyRaw, err := inputs.read(args[3], 4096)
	if err != nil {
		return plan, nil, err
	}
	private := inputs.files[len(inputs.files)-1]
	if private.info.Mode() != 0600 || private.info.Sys().(*syscall.Stat_t).Uid != 0 {
		return plan, nil, errSourceControlInput
	}
	key, err := parseSourceSigningKey(keyRaw)
	if err != nil {
		return plan, nil, err
	}
	defer clear(key)
	if !bytes.Equal(key[32:], prepared.profileContext().Authority) {
		return plan, nil, errSourceControlInput
	}
	if err := check(); err != nil {
		return plan, nil, err
	}
	signingContext := prepared.profileContext()
	signingContext.Now = time.Now().UTC()
	transcript, err := closedprofile.SigningTranscript(prepared.profileBody(), signingContext)
	if err != nil {
		return plan, nil, err
	}
	signed = append(bytes.Clone(prepared.profileBody()), ed25519.Sign(key, transcript)...)
	profile, err := closedprofile.Verify(signed, signingContext)
	if err != nil {
		return plan, nil, err
	}
	if err := prepared.bind(profile.Digest); err != nil {
		return plan, nil, err
	}
	if observe != nil {
		observe("signed", private.file)
	}
	if err := check(); err != nil {
		return plan, nil, err
	}
	if err := persistSourceArtifacts(ctx, args[4], []sourceArtifactFile{{"profile.bin", signed}}, check, observe); err != nil {
		return plan, nil, err
	}
	return plan, signed, check()
}

// runNodeProfileProvision dispatches one role's fixed prepare/sign profile
// purposes. The role supplies its own operation prefix, failure classification
// and prepare/sign hooks; no State or runtime authority is granted here.
func runNodeProfileProvision[P nodeProfileSigningPlan](role string, args []string, out io.Writer, fail func(error) error, prepare, sign func(context.Context, []string, func(string, *os.File)) (P, []byte, error)) error {
	if len(args) == 0 || os.Geteuid() != 0 {
		return fail(errSourceControlInput)
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	var plan P
	var artifact []byte
	var err error
	switch args[0] {
	case "prepare-profile":
		plan, artifact, err = prepare(ctx, args[1:], nil)
	case "sign-profile":
		plan, artifact, err = sign(ctx, args[1:], nil)
	default:
		return fail(errSourceControlInput)
	}
	if err != nil {
		return fail(err)
	}
	check := func() error { return errors.Join(plan.current(time.Now().UTC()), ctx.Err()) }
	if err := check(); err != nil {
		return fail(err)
	}
	sum := sha256.Sum256(artifact)
	raw, err := json.Marshal(struct {
		Operation string `json:"operation"`
		SHA       string `json:"sha256"`
		State     bool   `json:"state_accepted"`
		Runtime   bool   `json:"runtime_authority"`
		Installed bool   `json:"installed_qualification"`
	}{role + "." + args[0], hex.EncodeToString(sum[:]), false, false, false})
	if err != nil {
		return err
	}
	return errors.Join(writeSourceControlBytes(ctx, out, append(raw, '\n')), check())
}
