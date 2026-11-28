package state

import (
	"context"
	"errors"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	"github.com/dianabuilds/ardents-network/internal/domain/network/state/retiredservice"
	"github.com/dianabuilds/ardents-network/internal/platform/cgroup"
	"github.com/dianabuilds/ardents-network/internal/platform/process"
	"github.com/dianabuilds/ardents-network/internal/platform/systemd"
)

// RepairRetainedSourceProducer explicitly abandons a dead platform Source's
// sole serving obligation. It does not attest successful original completion.
// The original read-only manager connection, actual empty kernel scope and both
// exclusive retained leases are mandatory. No caller-supplied proof callback or
// boolean can authorize the removal. A new invocation rechecks normal admission.
func RepairRetainedSourceProducer(ctx context.Context, input Config, original *systemd.SourceRuntime, declaration string) (result error) {
	if ctx == nil || ctx.Err() != nil || original == nil {
		if ctx != nil {
			return errors.Join(ErrSourceRepairUnavailable, ctx.Err())
		}
		return ErrSourceRepairUnavailable
	}
	resolved, err := validateConfig(input)
	if err != nil {
		return err
	}
	defer resolved.source.Close()
	// This is read-only Network configuration, not a serving constructor. No
	// transport credential or private key is needed to abandon the exact guard.
	if resolved.sourceInfo.Configured || resolved.sourceInfo.Serving || resolved.automatic != 0 {
		return ErrSourceRepairUnavailable
	}
	if !strings.HasPrefix(declaration, "/etc/ardents/") || filepath.Clean(declaration) != declaration {
		return ErrSourceRepairUnavailable
	}
	resolved.sourceInfo.Serving = true
	if resolved.permitWork != nil {
		if err := resolved.permitWork(); err != nil {
			return err
		}
	}
	for _, root := range []string{resolved.root, resolved.localRoles} {
		if !strings.HasPrefix(root, "/var/lib/ardents/roles/source/") || filepath.Clean(root) != root {
			return ErrSourceRepairUnavailable
		}
	}
	principal, err := user.Lookup("ardents-source")
	if err != nil || principal.Uid != strconv.Itoa(os.Geteuid()) || principal.Gid != strconv.Itoa(os.Getegid()) {
		return ErrSourceRepairUnavailable
	}
	if err := checkSourceRepairRoots(resolved); err != nil {
		return err
	}
	r, err := openSourceRepair(ctx, resolved)
	if err != nil {
		return err
	}
	defer func() { result = errors.Join(result, r.close()) }()
	if err := checkSourceRepairQuiescence(ctx, original, resolved, declaration); err != nil {
		return err
	}
	if resolved.permitWork != nil {
		if err := resolved.permitWork(); err != nil {
			return err
		}
	}
	// Reobserve after input I/O. Holding both leases fences new State/producer
	// effects; the stopped cgroup proves the old invocation has no physical users.
	if err := checkSourceRepairQuiescence(ctx, original, resolved, declaration); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if _, err := r.checkHistory(); err != nil {
		return err
	}
	if resolved.permitWork != nil {
		if err := resolved.permitWork(); err != nil {
			return err
		}
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	return r.roles.Remove(sourceProducer("server", resolved.root))
}

func checkSourceRepairRoots(cfg config) error {
	if cfg.root == cfg.localRoles || strings.HasPrefix(cfg.root, cfg.localRoles+"/") || strings.HasPrefix(cfg.localRoles, cfg.root+"/") {
		return ErrSourceRepairUnavailable
	}
	for _, root := range []string{cfg.root, cfg.localRoles} {
		info, err := os.Lstat(root)
		if err != nil || info.Mode() != os.ModeDir|0700 {
			return ErrSourceRepairUnavailable
		}
		native, ok := info.Sys().(*syscall.Stat_t)
		if !ok || native.Uid != uint32(os.Geteuid()) || native.Gid != uint32(os.Getegid()) {
			return ErrSourceRepairUnavailable
		}
		for parent := filepath.Dir(root); ; parent = filepath.Dir(parent) {
			info, err := os.Lstat(parent)
			if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 || info.Mode().Perm()&0022 != 0 {
				return ErrSourceRepairUnavailable
			}
			native, ok := info.Sys().(*syscall.Stat_t)
			if !ok || native.Uid != 0 {
				return ErrSourceRepairUnavailable
			}
			if parent == "/" {
				break
			}
		}
	}
	return nil
}

func checkSourceRepairQuiescence(ctx context.Context, original *systemd.SourceRuntime, cfg config, declaration string) error {
	facts, err := original.Observe()
	if err != nil {
		return err
	}
	if !retiredservice.SourceStopped(facts.Unit, facts.Service, cfg.root, cfg.localRoles) {
		return ErrSourceRepairUnavailable
	}
	if !retiredservice.SourceDeclaration(facts.Service, declaration) {
		return ErrSourceRepairUnavailable
	}
	if err := cgroup.CheckSourceRecoveryQuiescent(ctx); err != nil {
		return err
	}
	return process.CheckSourceRecoveryQuiescent(ctx)
}
