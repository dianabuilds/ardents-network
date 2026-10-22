package main

import (
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"errors"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/network/source"
	"github.com/dianabuilds/ardents-network/internal/successor/network/state"
	"github.com/dianabuilds/ardents-network/internal/successor/processresource"
)

// Offline acceptance opens no Source listener, runtime key, process guard or
// Installation Startup. State owns authentication and all semantic floor writes.
func runSourceStatePreparation(args []string, out io.Writer) error {
	if len(args) != 2 {
		return errors.New("usage: ardents-control source prepare-state SERVE_PLAN BUNDLE")
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	if err := prepareSourceState(ctx, args[0], args[1]); err != nil {
		return &sourceControlFailure{err}
	}
	return writeSourceControlOutcome(ctx, out, "state-prepared")
}

func prepareSourceState(ctx context.Context, planPath, bundlePath string) (result error) {
	uid := os.Geteuid()
	if ctx == nil || uid < 100 || uid > 999 || os.Getuid() != uid || os.Getgid() != uid || os.Getegid() != uid {
		return errSourceControlInput
	}
	groups, err := os.Getgroups()
	if err != nil {
		return err
	}
	for _, group := range groups {
		if group != uid {
			return errSourceControlInput
		}
	}
	inputs := &sourceControlInputs{ctx: ctx}
	defer func() { result = errors.Join(result, inputs.close(), ctx.Err()) }()
	paths := []string{planPath, bundlePath}
	var raw [2][]byte
	for i, path := range paths {
		for _, previous := range paths[:i] {
			if path == previous {
				return errSourceControlInput
			}
		}
		maximum := int64(1 << 20)
		if i == 0 {
			maximum = 16 << 10
		}
		raw[i], err = inputs.read(path, maximum)
		if err != nil {
			return err
		}
	}
	plan, err := source.DecodeServingDeclaration(raw[0])
	if err != nil {
		return err
	}
	config, err := sourcePreparationStateConfig(plan)
	if err != nil {
		return err
	}
	for _, path := range paths {
		for _, root := range []string{config.Root, plan.ParticipationRoot} {
			if path == root || len(path) > len(root) && path[:len(root)+1] == root+"/" {
				return errSourceControlInput
			}
		}
	}
	root, err := inputs.directory(config.Root)
	if err != nil {
		return err
	}
	if root.info.Mode().Perm() != 0700 || root.info.Sys().(*syscall.Stat_t).Uid != uint32(uid) || root.info.Sys().(*syscall.Stat_t).Gid != uint32(uid) {
		return errSourceControlInput
	}
	entries, err := root.file.ReadDir(1)
	if err != io.EOF || len(entries) != 0 {
		return errSourceControlInput
	}
	bundle, err := source.DecodeBundle(raw[1])
	if err != nil {
		return err
	}
	config.PermitWork = inputs.check
	if err := inputs.check(); err != nil {
		return err
	}
	owner, err := state.Open(config)
	if err != nil {
		return err
	}
	defer func() { result = errors.Join(result, owner.Close(), inputs.check()) }()
	// A different explicit producer could have completed between the empty
	// directory observation and State's exclusive acquisition.
	if _, err = owner.Current(); !errors.Is(err, state.ErrNoCurrentGeneration) {
		return errors.Join(errSourceControlInput, err)
	}
	if _, err = owner.Accept(ctx, bundle.Epoch, bundle.Inputs, bundle.Materials); err != nil {
		return err
	}
	return inputs.check()
}

// State alone authenticates the signed Epoch and owns all floor writes.
// The public serving declaration supplies independently selected pins and paths;
// Source serving/acquisition stay absent from this offline configuration. This
// producer grants no accepted Route profile, membership, acquisition or duty.
func sourcePreparationStateConfig(plan source.ServingDeclaration) (state.Config, error) {
	n := plan.Network
	if n == nil || n.NetworkID == [32]byte{} || n.ProfileAuthority == [32]byte{} || plan.ProcessProfile != processresource.SourceProfile ||
		n.Threshold < 1 || n.Threshold > len(n.Authorities) || len(n.Authorities) > 16 ||
		!sourceControlPath(n.Root) || !sourceControlPath(n.ClockObservationFile) || !sourceControlPath(plan.ParticipationRoot) ||
		n.Root == plan.ParticipationRoot || filepath.Dir(n.Root) == n.Root {
		return state.Config{}, errSourceControlInput
	}
	pins := make(map[[32]byte]ed25519.PublicKey, len(n.Authorities))
	roots := []string{n.Root, n.ClockObservationFile, plan.ParticipationRoot}
	for i, path := range roots {
		for _, previous := range roots[:i] {
			if path == previous || strings.HasPrefix(path, previous+"/") || strings.HasPrefix(previous, path+"/") {
				return state.Config{}, errSourceControlInput
			}
		}
	}
	for _, public := range n.Authorities {
		id := sha256.Sum256(public[:])
		if public == [32]byte{} || pins[id] != nil {
			return state.Config{}, errSourceControlInput
		}
		pins[id] = append(ed25519.PublicKey(nil), public[:]...)
	}
	return state.Config{Root: n.Root, NetworkID: n.NetworkID, Authorities: pins, Threshold: n.Threshold,
		ClosedProfileAuthority: append(ed25519.PublicKey(nil), n.ProfileAuthority[:]...), AcceptedProfile: "ardents-route-v3",
		Clock: time.Now, ClockObservationFile: n.ClockObservationFile}, nil
}
