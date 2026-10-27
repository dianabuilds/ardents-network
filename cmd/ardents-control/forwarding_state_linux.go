package main

import (
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"errors"
	"io"
	"os"
	"os/signal"
	"os/user"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/installation/forwarding"
	"github.com/dianabuilds/ardents-network/internal/successor/network/source"
	"github.com/dianabuilds/ardents-network/internal/successor/network/state"
)

// A finite offline State producer, not an installed Node or Source runtime.
// Signed bytes are supplied publicly; State alone authenticates and persists
// them. No private key, participation, spend or Hosting root is opened.
func runForwardingStatePreparation(args []string, out io.Writer) error {
	if len(args) != 3 || out == nil {
		return errors.New("usage: ardents-control forwarding prepare-state PLAN BUNDLE SIGNED_PROFILE")
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	if err := prepareForwardingState(ctx, args[0], args[1], args[2]); err != nil {
		return &forwardingControlFailure{err}
	}
	_, err := io.WriteString(out, "{\"operation\":\"forwarding.prepare-state\",\"state_accepted\":true,\"runtime_authority\":false}\n")
	return errors.Join(err, ctx.Err())
}

func forwardingStatePrincipal() (uint32, error) {
	uid := os.Geteuid()
	if uid < 100 || uid > 999 || os.Getuid() != uid || os.Getegid() != uid || os.Getgid() != uid {
		return 0, forwarding.ErrInput
	}
	groups, err := os.Getgroups()
	if err != nil {
		return 0, err
	}
	for _, group := range groups {
		if group != uid {
			return 0, forwarding.ErrBinding
		}
	}
	numeric := strconv.Itoa(uid)
	account, err := user.LookupId(numeric)
	if err != nil || account.Username != "ardents-node" || account.Uid != numeric || account.Gid != numeric {
		return 0, errors.Join(forwarding.ErrBinding, err)
	}
	group, err := user.LookupGroupId(numeric)
	if err != nil || group.Name != "ardents-node" || group.Gid != numeric {
		return 0, errors.Join(forwarding.ErrBinding, err)
	}
	return uint32(uid), nil
}

func prepareForwardingState(ctx context.Context, planPath, bundlePath, profilePath string) (returned error) {
	if ctx == nil {
		return forwarding.ErrInput
	}
	uid, err := forwardingStatePrincipal()
	if err != nil {
		return err
	}
	inputs := &sourceControlInputs{ctx: ctx} // physical descriptors only; no Source permission
	defer func() { returned = errors.Join(returned, inputs.close(), ctx.Err()) }()
	paths := []string{planPath, bundlePath, profilePath}
	var raw [3][]byte
	for i, path := range paths {
		for _, previous := range paths[:i] {
			if path == previous {
				return forwarding.ErrInput
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
	plan, err := forwarding.DecodePlan(raw[0])
	if err != nil {
		return err
	}
	config, err := forwardingPreparationStateConfig(plan)
	if err != nil {
		return err
	}
	for _, path := range paths {
		for _, root := range []string{config.Root, plan.ParticipationRoot, plan.SpendRoot, plan.HostingRoot} {
			if path == root || strings.HasPrefix(path, root+"/") || strings.HasPrefix(root, path+"/") {
				return forwarding.ErrBinding
			}
		}
	}
	bundle, err := source.DecodeBundle(raw[1])
	if err != nil {
		return err
	}
	root, err := inputs.directory(config.Root)
	if err != nil {
		return err
	}
	var inputIO sync.Mutex
	check := func() error {
		inputIO.Lock()
		defer inputIO.Unlock()
		current, err := forwardingStatePrincipal()
		if err != nil || current != uid {
			return errors.Join(forwarding.ErrBinding, err)
		}
		for _, file := range inputs.files {
			info, err := file.file.Stat()
			if err != nil {
				return err
			}
			native, ok := info.Sys().(*syscall.Stat_t)
			if !ok {
				return forwarding.ErrBinding
			}
			if file == root {
				if !info.IsDir() || info.Mode() != os.ModeDir|0700 || native.Uid != uid || native.Gid != uid {
					return forwarding.ErrBinding
				}
			} else if native.Uid != 0 || info.Mode().Perm()&0022 != 0 {
				return forwarding.ErrBinding
			}
		}
		return inputs.check()
	}
	if err := check(); err != nil {
		return err
	}
	entries, err := root.file.ReadDir(1)
	if err != io.EOF || len(entries) != 0 {
		return forwarding.ErrBinding
	}
	config.PermitWork = check
	owner, err := state.Open(config)
	if err != nil {
		return err
	}
	defer func() { returned = errors.Join(returned, owner.Close(), check()) }()
	if _, err := owner.Current(); !errors.Is(err, state.ErrNoCurrentGeneration) {
		return errors.Join(forwarding.ErrBinding, err)
	}
	if _, err := owner.Accept(ctx, bundle.Epoch, bundle.Inputs, bundle.Materials); err != nil {
		return err
	}
	if _, err := owner.AcceptClosedProfile(raw[2]); err != nil {
		return err
	}
	view, err := owner.CurrentRuntime()
	if err != nil {
		return err
	}
	if _, err := view.Member(plan.NodeID, view.ObservedAt()); err != nil {
		return err
	}
	return check()
}

func forwardingPreparationStateConfig(plan forwarding.Plan) (state.Config, error) {
	n := plan.Network
	if plan.Schema != "ardents-forwarding-runtime-v1" || plan.ProcessProfile != "ardents-forwarding-process-v1" || plan.NodeID == [32]byte{} || n.NetworkID == [32]byte{} || n.ProfileAuthority == [32]byte{} || n.Threshold < 1 || n.Threshold > len(n.Authorities) || len(n.Authorities) > 16 {
		return state.Config{}, forwarding.ErrInput
	}
	roots := []string{n.Root, n.ClockObservationFile, plan.ParticipationRoot, plan.SpendRoot, plan.HostingRoot, plan.Certificate, plan.PrivateKey}
	for i, path := range roots {
		if !sourceControlPath(path) {
			return state.Config{}, forwarding.ErrInput
		}
		for _, previous := range roots[:i] {
			if path == previous || strings.HasPrefix(path, previous+"/") || strings.HasPrefix(previous, path+"/") {
				return state.Config{}, forwarding.ErrBinding
			}
		}
	}
	pins := make(map[[32]byte]ed25519.PublicKey, len(n.Authorities))
	for _, public := range n.Authorities {
		id := sha256.Sum256(public[:])
		if public == [32]byte{} || pins[id] != nil {
			return state.Config{}, forwarding.ErrBinding
		}
		pins[id] = append(ed25519.PublicKey(nil), public[:]...)
	}
	return state.Config{Root: n.Root, NetworkID: n.NetworkID, Authorities: pins, Threshold: n.Threshold, ClosedProfileAuthority: append(ed25519.PublicKey(nil), n.ProfileAuthority[:]...), AcceptedProfile: "ardents-route-v3", Clock: time.Now, ClockObservationFile: n.ClockObservationFile}, nil
}
