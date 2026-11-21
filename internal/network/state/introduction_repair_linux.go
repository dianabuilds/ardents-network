//go:build linux

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

	"github.com/dianabuilds/ardents-network/internal/platform/cgroup"
	"github.com/dianabuilds/ardents-network/internal/platform/systemd"
)

// RepairRetainedIntroductionProducer explicitly abandons a dead platform
// Introduction's sole node-duty participation fence. It does not attest successful
// original completion. The original read-only manager connection, actual empty
// kernel scope and both exclusive retained leases are mandatory. No caller-supplied
// proof callback or boolean can authorize the removal. A new invocation rechecks
// normal admission. Slot claims and spend obligations are never repaired here.
//
// Unlike Source, Introduction shares the ardents-node account with sibling Node
// role units, so quiescence is per-unit cgroup only — never an account-wide process
// scan that would refuse legitimate siblings. writableRoots are the exact ordered
// platform write paths (network, participation, spend, hosting, introduction).
func RepairRetainedIntroductionProducer(ctx context.Context, input Config, original *systemd.IntroductionRuntime, declaration string, writableRoots []string) (result error) {
	if ctx == nil || ctx.Err() != nil || original == nil {
		if ctx != nil {
			return errors.Join(ErrIntroductionRepairUnavailable, ctx.Err())
		}
		return ErrIntroductionRepairUnavailable
	}
	resolved, err := validateConfig(input)
	if err != nil {
		return err
	}
	// Read-only Network configuration, not a serving constructor. No transport
	// credential or private key is needed to abandon the exact participation fence.
	if resolved.sourceInfo.Configured || resolved.sourceInfo.Serving || resolved.automatic != 0 {
		return ErrIntroductionRepairUnavailable
	}
	if !strings.HasPrefix(declaration, "/etc/ardents/") || filepath.Clean(declaration) != declaration {
		return ErrIntroductionRepairUnavailable
	}
	if resolved.permitWork != nil {
		if err := resolved.permitWork(); err != nil {
			return err
		}
	}
	for _, root := range append([]string{resolved.root, resolved.localRoles}, writableRoots...) {
		if !strings.HasPrefix(root, "/var/lib/ardents/roles/introduction/") || filepath.Clean(root) != root {
			return ErrIntroductionRepairUnavailable
		}
	}
	principal, err := user.Lookup("ardents-node")
	if err != nil || principal.Uid != strconv.Itoa(os.Geteuid()) || principal.Gid != strconv.Itoa(os.Getegid()) {
		return ErrIntroductionRepairUnavailable
	}
	if err := checkIntroductionRepairRoots(resolved, writableRoots); err != nil {
		return err
	}
	r, err := openIntroductionRepair(ctx, resolved)
	if err != nil {
		return err
	}
	defer func() { result = errors.Join(result, r.close()) }()
	if err := checkIntroductionRepairQuiescence(ctx, original, resolved, declaration, writableRoots); err != nil {
		return err
	}
	if resolved.permitWork != nil {
		if err := resolved.permitWork(); err != nil {
			return err
		}
	}
	// Reobserve after input I/O. Holding both leases fences new State/producer
	// effects; the stopped per-unit cgroup proves the old invocation has no users.
	if err := checkIntroductionRepairQuiescence(ctx, original, resolved, declaration, writableRoots); err != nil {
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
	return r.roles.Remove(r.producer)
}

func checkIntroductionRepairRoots(cfg config, writableRoots []string) error {
	// writableRoots is the complete ordered platform write set (network,
	// participation, spend, hosting, introduction); it already includes the State
	// and local-role roots, so no extra roots are appended.
	roots := writableRoots
	for i, root := range roots {
		for _, other := range roots[i+1:] {
			if root == other || strings.HasPrefix(root, other+"/") || strings.HasPrefix(other, root+"/") {
				return ErrIntroductionRepairUnavailable
			}
		}
		info, err := os.Lstat(root)
		if err != nil || info.Mode() != os.ModeDir|0700 {
			return ErrIntroductionRepairUnavailable
		}
		native, ok := info.Sys().(*syscall.Stat_t)
		if !ok || native.Uid != uint32(os.Geteuid()) || native.Gid != uint32(os.Getegid()) {
			return ErrIntroductionRepairUnavailable
		}
		for parent := filepath.Dir(root); ; parent = filepath.Dir(parent) {
			info, err := os.Lstat(parent)
			if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 || info.Mode().Perm()&0022 != 0 {
				return ErrIntroductionRepairUnavailable
			}
			native, ok := info.Sys().(*syscall.Stat_t)
			if !ok || native.Uid != 0 {
				return ErrIntroductionRepairUnavailable
			}
			if parent == "/" {
				break
			}
		}
	}
	return nil
}

func checkIntroductionRepairQuiescence(ctx context.Context, original *systemd.IntroductionRuntime, cfg config, declaration string, writableRoots []string) error {
	facts, err := original.Observe()
	if err != nil {
		return err
	}
	if err := introductionRepairStoppedPolicy(facts.Unit, facts.Service, writableRoots); err != nil {
		return err
	}
	if !introductionRepairDeclaration(facts.Service, declaration) {
		return ErrIntroductionRepairUnavailable
	}
	// Per-unit cgroup only: the shared ardents-node account forbids an account-wide
	// process scan (sibling Node roles legitimately run under the same principal).
	return cgroup.CheckIntroductionRecoveryQuiescent(ctx)
}

func introductionRepairDeclaration(service systemd.Properties, declaration string) bool {
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
	if !strings.HasPrefix(program, "/usr/lib/ardents/") || filepath.Clean(program) != program || len(args) != 5 || args[0] != program || args[1] != "node" || args[2] != "introduction" || args[3] != "--retained-config" || args[4] != declaration {
		return false
	}
	return len(flags) == 1 && flags[0] == "no-env-expand"
}

func introductionRepairStoppedPolicy(unit, service systemd.Properties, writableRoots []string) error {
	for name, value := range map[string]string{"Id": "ardents-introduction.service", "LoadState": "loaded", "FragmentPath": "/etc/systemd/system/ardents-introduction.service"} {
		if !systemd.Matches(unit, name, "s", value) {
			return ErrIntroductionRepairUnavailable
		}
	}
	if !(systemd.Matches(unit, "ActiveState", "s", "inactive") && systemd.Matches(unit, "SubState", "s", "dead")) &&
		!(systemd.Matches(unit, "ActiveState", "s", "failed") && systemd.Matches(unit, "SubState", "s", "failed")) {
		return ErrIntroductionRepairUnavailable
	}
	if !systemd.Matches(unit, "DropInPaths", "as", []string{}) || !systemd.Matches(unit, "TriggeredBy", "as", []string{}) {
		return ErrIntroductionRepairUnavailable
	}
	for name, value := range map[string]string{"User": "ardents-node", "Group": "ardents-node", "KillMode": "control-group", "Restart": "no", "ProtectSystem": "strict"} {
		if !systemd.Matches(service, name, "s", value) {
			return ErrIntroductionRepairUnavailable
		}
	}
	for _, name := range []string{"MainPID", "ControlPID"} {
		if !systemd.Matches(service, name, "u", uint32(0)) {
			return ErrIntroductionRepairUnavailable
		}
	}
	for _, name := range []string{"NoNewPrivileges", "ProtectControlGroups"} {
		if !systemd.Matches(service, name, "b", true) {
			return ErrIntroductionRepairUnavailable
		}
	}
	for _, name := range []string{"SendSIGKILL", "DynamicUser", "Delegate"} {
		if !systemd.Matches(service, name, "b", false) {
			return ErrIntroductionRepairUnavailable
		}
	}
	if !systemd.Matches(service, "ReadWritePaths", "as", writableRoots) {
		return ErrIntroductionRepairUnavailable
	}
	if !systemd.Matches(service, "ControlGroup", "s", "") && !systemd.Matches(service, "ControlGroup", "s", "/system.slice/ardents-introduction.service") {
		return ErrIntroductionRepairUnavailable
	}
	return nil
}
