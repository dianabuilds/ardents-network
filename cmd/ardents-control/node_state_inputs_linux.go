package main

import (
	"context"
	"errors"
	"io"
	"os"
	"os/user"
	"strconv"
	"strings"
	"sync"
	"syscall"

	"github.com/dianabuilds/ardents-network/internal/successor/network/source"
	"github.com/dianabuilds/ardents-network/internal/successor/network/state"
)

// prepareNodeState retains the original input and State lifetimes. Its caller
// selects a role-specific decoder and roots before any State effect.
func nodeStatePrincipal(invalid, binding error) (uint32, error) {
	uid := os.Geteuid()
	if uid < 100 || uid > 999 || os.Getuid() != uid || os.Getegid() != uid || os.Getgid() != uid {
		return 0, invalid
	}
	groups, err := os.Getgroups()
	if err != nil {
		return 0, err
	}
	for _, group := range groups {
		if group != uid {
			return 0, binding
		}
	}
	numeric := strconv.Itoa(uid)
	account, err := user.LookupId(numeric)
	if err != nil || account.Username != "ardents-node" || account.Uid != numeric || account.Gid != numeric {
		return 0, errors.Join(binding, err)
	}
	group, err := user.LookupGroupId(numeric)
	if err != nil || group.Name != "ardents-node" || group.Gid != numeric {
		return 0, errors.Join(binding, err)
	}
	return uint32(uid), nil
}

func prepareNodeState(ctx context.Context, planPath, bundlePath, profilePath string, invalid, binding error, decode func([]byte) (state.Config, [32]byte, []string, error)) (returned error) {
	if ctx == nil {
		return invalid
	}
	uid, err := nodeStatePrincipal(invalid, binding)
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
				return invalid
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
	config, nodeID, roots, err := decode(raw[0])
	if err != nil {
		return err
	}
	for _, path := range paths {
		for _, root := range roots {
			if path == root || strings.HasPrefix(path, root+"/") || strings.HasPrefix(root, path+"/") {
				return binding
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
		current, err := nodeStatePrincipal(invalid, binding)
		if err != nil || current != uid {
			return errors.Join(binding, err)
		}
		for _, file := range inputs.files {
			info, err := file.file.Stat()
			if err != nil {
				return err
			}
			native, ok := info.Sys().(*syscall.Stat_t)
			if !ok {
				return binding
			}
			if file == root {
				if !info.IsDir() || info.Mode() != os.ModeDir|0700 || native.Uid != uid || native.Gid != uid {
					return binding
				}
			} else if native.Uid != 0 || info.Mode().Perm()&0022 != 0 {
				return binding
			}
		}
		return inputs.check()
	}
	if err := check(); err != nil {
		return err
	}
	entries, err := root.file.ReadDir(1)
	if err != io.EOF || len(entries) != 0 {
		return binding
	}
	config.PermitWork = check
	owner, err := state.Open(config)
	if err != nil {
		return err
	}
	defer func() { returned = errors.Join(returned, owner.Close(), check()) }()
	if _, err := owner.Current(); !errors.Is(err, state.ErrNoCurrentGeneration) {
		return errors.Join(binding, err)
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
	if _, err := view.Member(nodeID, view.ObservedAt()); err != nil {
		return err
	}
	return check()
}
