package main

import (
	"context"
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

	"github.com/dianabuilds/ardents-network/internal/successor/deployment"
	"github.com/dianabuilds/ardents-network/internal/successor/installation/descriptor"
	"github.com/dianabuilds/ardents-network/internal/successor/network/source"
	"github.com/dianabuilds/ardents-network/internal/successor/network/state"
)

// A finite offline State producer, not an installed Node or Source runtime.
// Signed bytes are supplied publicly; State alone authenticates and persists
// them. No private key, participation, spend or Hosting root is opened.
func runDescriptorStatePreparation(args []string, out io.Writer) error {
	if len(args) != 3 || out == nil {
		return errors.New("usage: ardents-control descriptor prepare-state PLAN BUNDLE SIGNED_PROFILE")
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	if err := prepareDescriptorState(ctx, args[0], args[1], args[2]); err != nil {
		return &descriptorControlFailure{err}
	}
	_, err := io.WriteString(out, "{\"operation\":\"descriptor.prepare-state\",\"state_accepted\":true,\"runtime_authority\":false}\n")
	return errors.Join(err, ctx.Err())
}

func descriptorStatePrincipal() (uint32, error) {
	uid := os.Geteuid()
	if uid < 100 || uid > 999 || os.Getuid() != uid || os.Getegid() != uid || os.Getgid() != uid {
		return 0, descriptor.ErrInput
	}
	groups, err := os.Getgroups()
	if err != nil {
		return 0, err
	}
	for _, group := range groups {
		if group != uid {
			return 0, descriptor.ErrBinding
		}
	}
	numeric := strconv.Itoa(uid)
	account, err := user.LookupId(numeric)
	if err != nil || account.Username != "ardents-node" || account.Uid != numeric || account.Gid != numeric {
		return 0, errors.Join(descriptor.ErrBinding, err)
	}
	group, err := user.LookupGroupId(numeric)
	if err != nil || group.Name != "ardents-node" || group.Gid != numeric {
		return 0, errors.Join(descriptor.ErrBinding, err)
	}
	return uint32(uid), nil
}

func prepareDescriptorState(ctx context.Context, planPath, bundlePath, profilePath string) (returned error) {
	if ctx == nil {
		return descriptor.ErrInput
	}
	uid, err := descriptorStatePrincipal()
	if err != nil {
		return err
	}
	inputs := &sourceControlInputs{ctx: ctx} // physical joins only; no Source permission
	defer func() { returned = errors.Join(returned, inputs.close(), ctx.Err()) }()
	paths := []string{planPath, bundlePath, profilePath}
	var raw [3][]byte
	for i, path := range paths {
		for _, previous := range paths[:i] {
			if path == previous {
				return descriptor.ErrInput
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
	plan, err := deployment.DecodeDescriptorPlan(raw[0])
	if err != nil {
		return descriptor.ErrInput
	}
	config, err := descriptorPreparationStateConfig(plan)
	if err != nil {
		return err
	}
	for _, path := range paths {
		for _, root := range []string{config.Root, plan.ParticipationRoot, plan.SpendRoot, plan.HostingRoot, plan.DescriptorRoot} {
			if path == root || strings.HasPrefix(path, root+"/") || strings.HasPrefix(root, path+"/") {
				return descriptor.ErrBinding
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
		current, err := descriptorStatePrincipal()
		if err != nil || current != uid {
			return errors.Join(descriptor.ErrBinding, err)
		}
		for _, file := range inputs.files {
			info, err := file.file.Stat()
			if err != nil {
				return err
			}
			native, ok := info.Sys().(*syscall.Stat_t)
			if !ok {
				return descriptor.ErrBinding
			}
			if file == root {
				if !info.IsDir() || info.Mode() != os.ModeDir|0700 || native.Uid != uid || native.Gid != uid {
					return descriptor.ErrBinding
				}
			} else if native.Uid != 0 || info.Mode().Perm()&0022 != 0 {
				return descriptor.ErrBinding
			}
		}
		return inputs.check()
	}
	if err := check(); err != nil {
		return err
	}
	entries, err := root.file.ReadDir(1)
	if err != io.EOF || len(entries) != 0 {
		return descriptor.ErrBinding
	}
	config.PermitWork = check
	owner, err := state.Open(config)
	if err != nil {
		return err
	}
	defer func() { returned = errors.Join(returned, owner.Close(), check()) }()
	if _, err := owner.Current(); !errors.Is(err, state.ErrNoCurrentGeneration) {
		return errors.Join(descriptor.ErrBinding, err)
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

func descriptorPreparationStateConfig(plan deployment.DescriptorPlan) (state.Config, error) {
	if plan.Schema != "ardents-descriptor-runtime-v1" || plan.ProcessProfile != "ardents-descriptor-process-v1" || plan.NodeID == [32]byte{} {
		return state.Config{}, descriptor.ErrInput
	}
	return nodePreparationStateConfig(plan.Network, []string{plan.ParticipationRoot, plan.SpendRoot, plan.HostingRoot, plan.Certificate, plan.PrivateKey}, descriptor.ErrInput, descriptor.ErrBinding)
}
