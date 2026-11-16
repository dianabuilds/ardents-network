package state

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	"github.com/dianabuilds/ardents-network/internal/successor/installation/cgroup"
	"github.com/dianabuilds/ardents-network/internal/successor/installation/process"
	"github.com/dianabuilds/ardents-network/internal/successor/installation/systemd"
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
	if err := sourceRepairStoppedPolicy(facts.Unit, facts.Service, cfg.root, cfg.localRoles); err != nil {
		return err
	}
	if !sourceRepairDeclaration(facts.Service, declaration) {
		return ErrSourceRepairUnavailable
	}
	if err := cgroup.CheckSourceRecoveryQuiescent(ctx); err != nil {
		return err
	}
	return process.CheckSourceRecoveryQuiescent(ctx)
}

func sourceRepairDeclaration(service systemd.Properties, declaration string) bool {
	value := service["ExecStartEx"]
	var rows [][]json.RawMessage
	if value.Type != "a(sasasttttuii)" || systemd.Decode(value.Data, &rows) != nil || len(rows) != 1 || len(rows[0]) != 10 {
		return false
	}
	var program string
	var args, flags []string
	if systemd.Decode(rows[0][0], &program) != nil || systemd.Decode(rows[0][1], &args) != nil || systemd.Decode(rows[0][2], &flags) != nil {
		return false
	}
	if !strings.HasPrefix(program, "/usr/lib/ardents/") || filepath.Clean(program) != program || len(args) != 4 || args[0] != program || args[1] != "source" || args[2] != "serve-retained" || args[3] != declaration {
		return false
	}
	return len(flags) == 1 && flags[0] == "no-env-expand"
}

func sourceRepairStoppedPolicy(unit, service systemd.Properties, stateRoot, participation string) error {
	for name, value := range map[string]string{"Id": "ardents-source.service", "LoadState": "loaded", "FragmentPath": "/etc/systemd/system/ardents-source.service"} {
		if !systemd.Matches(unit, name, "s", value) {
			return ErrSourceRepairUnavailable
		}
	}
	if !(systemd.Matches(unit, "ActiveState", "s", "inactive") && systemd.Matches(unit, "SubState", "s", "dead")) &&
		!(systemd.Matches(unit, "ActiveState", "s", "failed") && systemd.Matches(unit, "SubState", "s", "failed")) {
		return ErrSourceRepairUnavailable
	}
	if !systemd.Matches(unit, "DropInPaths", "as", []string{}) || !systemd.Matches(unit, "TriggeredBy", "as", []string{}) {
		return ErrSourceRepairUnavailable
	}
	for name, value := range map[string]string{"User": "ardents-source", "Group": "ardents-source", "KillMode": "control-group", "Restart": "no", "ProtectSystem": "strict"} {
		if !systemd.Matches(service, name, "s", value) {
			return ErrSourceRepairUnavailable
		}
	}
	for _, name := range []string{"MainPID", "ControlPID"} {
		if !systemd.Matches(service, name, "u", uint32(0)) {
			return ErrSourceRepairUnavailable
		}
	}
	for _, name := range []string{"NoNewPrivileges", "ProtectControlGroups"} {
		if !systemd.Matches(service, name, "b", true) {
			return ErrSourceRepairUnavailable
		}
	}
	for _, name := range []string{"SendSIGKILL", "DynamicUser", "Delegate"} {
		if !systemd.Matches(service, name, "b", false) {
			return ErrSourceRepairUnavailable
		}
	}
	if !systemd.Matches(service, "ReadWritePaths", "as", []string{stateRoot, participation}) {
		return ErrSourceRepairUnavailable
	}
	if !systemd.Matches(service, "ControlGroup", "s", "") && !systemd.Matches(service, "ControlGroup", "s", "/system.slice/ardents-source.service") {
		return ErrSourceRepairUnavailable
	}
	return nil
}
