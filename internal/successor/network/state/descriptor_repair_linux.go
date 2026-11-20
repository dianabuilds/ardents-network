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

	"github.com/dianabuilds/ardents-network/internal/successor/installation/cgroup"
	"github.com/dianabuilds/ardents-network/internal/successor/installation/systemd"
	"github.com/dianabuilds/ardents-network/internal/successor/reachability"
)

// RepairRetainedDescriptorProducer explicitly abandons a dead platform
// Descriptor's sole node-duty participation fence. It does not attest successful
// original completion. The original read-only manager connection, actual empty
// kernel scope and both exclusive retained leases are mandatory. No
// caller-supplied proof callback or boolean can authorize the removal. A new
// invocation rechecks normal admission. Spend obligations and publication floors
// are never repaired here.
//
// Descriptor shares the ardents-node account with sibling Node role units, so
// quiescence is per-unit cgroup only — never an account-wide process scan that
// would refuse legitimate siblings. writableRoots are the exact ordered platform
// write paths (network, participation, spend, hosting, descriptor); the fifth
// entry is the Store root.
//
// The Store lease is flock and dies with the process, so the Store root carries
// no durable fence to remove. A crash mid-Publish can instead leave the reserved
// uncertain staging name, which a retained opening refuses. After the first
// stopped-unit proof and while holding both retained leases, this adapter
// invokes the bounded reachability.RepairRetainedStoreStage to validate every
// committed floor and remove only that staging under the exclusive Store lease,
// then reobserves quiescence before the one participation commit. It never
// initializes a missing Store root, resets or evicts a floor, promotes staging
// or deletes foreign entries.
func RepairRetainedDescriptorProducer(ctx context.Context, input Config, original *systemd.DescriptorRuntime, declaration string, writableRoots []string) (result error) {
	if ctx == nil || ctx.Err() != nil || original == nil {
		if ctx != nil {
			return errors.Join(ErrDescriptorRepairUnavailable, ctx.Err())
		}
		return ErrDescriptorRepairUnavailable
	}
	resolved, err := validateConfig(input)
	if err != nil {
		return err
	}
	// Read-only Network configuration, not a serving constructor. No transport
	// credential or private key is needed to abandon the exact participation fence.
	if resolved.sourceInfo.Configured || resolved.sourceInfo.Serving || resolved.automatic != 0 {
		return ErrDescriptorRepairUnavailable
	}
	if !strings.HasPrefix(declaration, "/etc/ardents/") || filepath.Clean(declaration) != declaration {
		return ErrDescriptorRepairUnavailable
	}
	if resolved.permitWork != nil {
		if err := resolved.permitWork(); err != nil {
			return err
		}
	}
	for _, root := range append([]string{resolved.root, resolved.localRoles}, writableRoots...) {
		if !strings.HasPrefix(root, "/var/lib/ardents/roles/descriptor/") || filepath.Clean(root) != root {
			return ErrDescriptorRepairUnavailable
		}
	}
	principal, err := user.Lookup("ardents-node")
	if err != nil || principal.Uid != strconv.Itoa(os.Geteuid()) || principal.Gid != strconv.Itoa(os.Getegid()) {
		return ErrDescriptorRepairUnavailable
	}
	if err := checkDescriptorRepairRoots(resolved, writableRoots); err != nil {
		return err
	}
	r, err := openDescriptorRepair(ctx, resolved)
	if err != nil {
		return err
	}
	defer func() { result = errors.Join(result, r.close()) }()
	if err := checkDescriptorRepairQuiescence(ctx, original, resolved, declaration, writableRoots); err != nil {
		return err
	}
	if resolved.permitWork != nil {
		if err := resolved.permitWork(); err != nil {
			return err
		}
	}
	// Bounded Store stage recovery under the same proofs and both retained
	// leases. The current accepted generation supplies the Store network
	// identity; a missing or invalid Store root refuses without initialization.
	current, err := r.checkHistory()
	if err != nil {
		return err
	}
	if err := reachability.RepairRetainedStoreStage(reachability.StoreConfig{Root: writableRoots[len(writableRoots)-1], Network: current.Snapshot.NetworkID}); err != nil {
		return err
	}
	// Reobserve after input and Store I/O. Holding both leases fences new
	// State/producer effects; the stopped per-unit cgroup proves the old
	// invocation has no users.
	if err := checkDescriptorRepairQuiescence(ctx, original, resolved, declaration, writableRoots); err != nil {
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

func checkDescriptorRepairRoots(cfg config, writableRoots []string) error {
	// writableRoots is the complete ordered platform write set (network,
	// participation, spend, hosting, descriptor); it already includes the State
	// and local-role roots, so no extra roots are appended. The fifth entry is
	// the Store root whose uncertain stage the bounded repair may clear.
	if len(writableRoots) != 5 {
		return ErrDescriptorRepairUnavailable
	}
	roots := writableRoots
	for i, root := range roots {
		for _, other := range roots[i+1:] {
			if root == other || strings.HasPrefix(root, other+"/") || strings.HasPrefix(other, root+"/") {
				return ErrDescriptorRepairUnavailable
			}
		}
		info, err := os.Lstat(root)
		if err != nil || info.Mode() != os.ModeDir|0700 {
			return ErrDescriptorRepairUnavailable
		}
		native, ok := info.Sys().(*syscall.Stat_t)
		if !ok || native.Uid != uint32(os.Geteuid()) || native.Gid != uint32(os.Getegid()) {
			return ErrDescriptorRepairUnavailable
		}
		for parent := filepath.Dir(root); ; parent = filepath.Dir(parent) {
			info, err := os.Lstat(parent)
			if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 || info.Mode().Perm()&0022 != 0 {
				return ErrDescriptorRepairUnavailable
			}
			native, ok := info.Sys().(*syscall.Stat_t)
			if !ok || native.Uid != 0 {
				return ErrDescriptorRepairUnavailable
			}
			if parent == "/" {
				break
			}
		}
	}
	return nil
}

func checkDescriptorRepairQuiescence(ctx context.Context, original *systemd.DescriptorRuntime, cfg config, declaration string, writableRoots []string) error {
	facts, err := original.Observe()
	if err != nil {
		return err
	}
	if err := descriptorRepairStoppedPolicy(facts.Unit, facts.Service, writableRoots); err != nil {
		return err
	}
	if !descriptorRepairDeclaration(facts.Service, declaration) {
		return ErrDescriptorRepairUnavailable
	}
	// Per-unit cgroup only: the shared ardents-node account forbids an account-wide
	// process scan (sibling Node roles legitimately run under the same principal).
	return cgroup.CheckDescriptorRecoveryQuiescent(ctx)
}

func descriptorRepairDeclaration(service systemd.Properties, declaration string) bool {
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
	if !strings.HasPrefix(program, "/usr/lib/ardents/") || filepath.Clean(program) != program || len(args) != 5 || args[0] != program || args[1] != "node" || args[2] != "descriptor" || args[3] != "--retained-config" || args[4] != declaration {
		return false
	}
	return len(flags) == 1 && flags[0] == "no-env-expand"
}

func descriptorRepairStoppedPolicy(unit, service systemd.Properties, writableRoots []string) error {
	for name, value := range map[string]string{"Id": "ardents-descriptor.service", "LoadState": "loaded", "FragmentPath": "/etc/systemd/system/ardents-descriptor.service"} {
		if !systemd.Matches(unit, name, "s", value) {
			return ErrDescriptorRepairUnavailable
		}
	}
	if !(systemd.Matches(unit, "ActiveState", "s", "inactive") && systemd.Matches(unit, "SubState", "s", "dead")) &&
		!(systemd.Matches(unit, "ActiveState", "s", "failed") && systemd.Matches(unit, "SubState", "s", "failed")) {
		return ErrDescriptorRepairUnavailable
	}
	if !systemd.Matches(unit, "DropInPaths", "as", []string{}) || !systemd.Matches(unit, "TriggeredBy", "as", []string{}) {
		return ErrDescriptorRepairUnavailable
	}
	for name, value := range map[string]string{"User": "ardents-node", "Group": "ardents-node", "KillMode": "control-group", "Restart": "no", "ProtectSystem": "strict"} {
		if !systemd.Matches(service, name, "s", value) {
			return ErrDescriptorRepairUnavailable
		}
	}
	for _, name := range []string{"MainPID", "ControlPID"} {
		if !systemd.Matches(service, name, "u", uint32(0)) {
			return ErrDescriptorRepairUnavailable
		}
	}
	for _, name := range []string{"NoNewPrivileges", "ProtectControlGroups"} {
		if !systemd.Matches(service, name, "b", true) {
			return ErrDescriptorRepairUnavailable
		}
	}
	for _, name := range []string{"SendSIGKILL", "DynamicUser", "Delegate"} {
		if !systemd.Matches(service, name, "b", false) {
			return ErrDescriptorRepairUnavailable
		}
	}
	if !systemd.Matches(service, "ReadWritePaths", "as", writableRoots) {
		return ErrDescriptorRepairUnavailable
	}
	if !systemd.Matches(service, "ControlGroup", "s", "") && !systemd.Matches(service, "ControlGroup", "s", "/system.slice/ardents-descriptor.service") {
		return ErrDescriptorRepairUnavailable
	}
	return nil
}
