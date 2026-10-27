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

	"github.com/dianabuilds/ardents-network/internal/successor/network/closedprofile"
)

// Only this fixed profile purpose may use the original matching State key.
// It authenticates a detached artifact, never accepts State or opens a runtime.
func signForwardingProfile(ctx context.Context, args []string, observe func(string, *os.File)) (plan forwardingProfilePlan, signed []byte, returned error) {
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
	if _, err := forwardingPublic32(args[2]); err != nil {
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
	raw, err := inputs.read(args[0], maximumForwardingProfilePlan)
	if err != nil {
		return plan, nil, err
	}
	var prepared forwardingProfilePreparation
	plan, prepared, err = decodeForwardingProfilePlan(raw, time.Now().UTC())
	if err != nil {
		return plan, nil, err
	}
	admitted = true
	actual, err := inputs.read(args[1], closedprofile.MaxSize)
	if err != nil {
		return plan, nil, err
	}
	sum := sha256.Sum256(prepared.body)
	if !bytes.Equal(actual, prepared.body) || args[2] != hex.EncodeToString(sum[:]) {
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
	if !bytes.Equal(key[32:], prepared.context.Authority) {
		return plan, nil, errSourceControlInput
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

func runForwardingProfileProvision(args []string, out io.Writer) error {
	if len(args) == 0 || os.Geteuid() != 0 {
		return &forwardingControlFailure{errSourceControlInput}
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	var plan forwardingProfilePlan
	var artifact []byte
	var err error
	switch args[0] {
	case "prepare-profile":
		plan, artifact, err = prepareForwardingProfile(ctx, args[1:], nil)
	case "sign-profile":
		plan, artifact, err = signForwardingProfile(ctx, args[1:], nil)
	default:
		return &forwardingControlFailure{errSourceControlInput}
	}
	if err != nil {
		return &forwardingControlFailure{err}
	}
	check := func() error { return errors.Join(plan.current(time.Now().UTC()), ctx.Err()) }
	if err := check(); err != nil {
		return &forwardingControlFailure{err}
	}
	sum := sha256.Sum256(artifact)
	raw, err := json.Marshal(struct {
		Operation string `json:"operation"`
		SHA       string `json:"sha256"`
		State     bool   `json:"state_accepted"`
		Runtime   bool   `json:"runtime_authority"`
		Installed bool   `json:"installed_qualification"`
	}{"forwarding." + args[0], hex.EncodeToString(sum[:]), false, false, false})
	if err != nil {
		return err
	}
	return errors.Join(writeSourceControlBytes(ctx, out, append(raw, '\n')), check())
}
