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

	"github.com/dianabuilds/ardents-network/internal/network/epoch"
	"github.com/dianabuilds/ardents-network/internal/platform/systemd"
)

// nodeDutyRepairNative binds one node-duty role's portable repair descriptor to
// its native facts. observe is the original read-only manager observation seam
// supplied by the role wrapper from its systemd.<Role>Runtime; it is nil exactly
// when the caller supplied no original manager connection. quiesce is the
// per-unit cgroup observation for the role's fixed unit. rootCount is the fixed
// writable-root count when the role requires one (Descriptor: five), zero
// otherwise. storeStage is the optional bounded Store stage hook (Descriptor
// only), invoked after the first stopped-unit proof and while holding both
// retained leases. No field grants Start, Stop, domain initialization or
// installed qualification.
type nodeDutyRepairNative struct {
	role       nodeDutyRepairRole
	observe    func() (systemd.Properties, systemd.Properties, error)
	quiesce    func(context.Context) error
	rootCount  int
	storeStage func(*epoch.Decision, []string) error
}

// repairRetainedNodeDutyProducer is the one shared native skeleton for the
// Forwarding, JOIN, Descriptor and Introduction participation repairs. It
// explicitly abandons a dead platform role's sole node-duty participation
// fence. It does not attest successful original completion. The original
// read-only manager connection, actual empty kernel scope and both exclusive
// retained leases are mandatory. No caller-supplied proof callback or boolean
// can authorize the removal. A new invocation rechecks normal admission. Slot
// claims, spend obligations and publication floors are never repaired here.
//
// Every node-duty role shares the ardents-node account with sibling Node role
// units, so quiescence is per-unit cgroup only — never an account-wide process
// scan that would refuse legitimate siblings. writableRoots are the exact
// ordered platform write paths (network, participation, spend, hosting, plus
// the role's own root where present).
func repairRetainedNodeDutyProducer(ctx context.Context, input Config, native nodeDutyRepairNative, declaration string, writableRoots []string) (result error) {
	if ctx == nil || ctx.Err() != nil || native.observe == nil {
		if ctx != nil {
			return errors.Join(native.role.unavailable, ctx.Err())
		}
		return native.role.unavailable
	}
	resolved, err := validateConfig(input)
	if err != nil {
		return err
	}
	// Read-only Network configuration, not a serving constructor. No transport
	// credential or private key is needed to abandon the exact participation fence.
	if resolved.sourceInfo.Configured || resolved.sourceInfo.Serving || resolved.automatic != 0 {
		return native.role.unavailable
	}
	if !strings.HasPrefix(declaration, "/etc/ardents/") || filepath.Clean(declaration) != declaration {
		return native.role.unavailable
	}
	if resolved.permitWork != nil {
		if err := resolved.permitWork(); err != nil {
			return err
		}
	}
	for _, root := range append([]string{resolved.root, resolved.localRoles}, writableRoots...) {
		if !strings.HasPrefix(root, "/var/lib/ardents/roles/"+native.role.name+"/") || filepath.Clean(root) != root {
			return native.role.unavailable
		}
	}
	principal, err := user.Lookup("ardents-node")
	if err != nil || principal.Uid != strconv.Itoa(os.Geteuid()) || principal.Gid != strconv.Itoa(os.Getegid()) {
		return native.role.unavailable
	}
	if err := checkNodeDutyRepairRoots(native, writableRoots); err != nil {
		return err
	}
	r, err := openNodeDutyRepair(ctx, resolved, native.role)
	if err != nil {
		return err
	}
	defer func() { result = errors.Join(result, r.close()) }()
	if err := checkNodeDutyRepairQuiescence(ctx, native, declaration, writableRoots); err != nil {
		return err
	}
	if resolved.permitWork != nil {
		if err := resolved.permitWork(); err != nil {
			return err
		}
	}
	if native.storeStage != nil {
		// Bounded Store stage recovery under the same proofs and both retained
		// leases. The current accepted generation supplies the Store network
		// identity; a missing or invalid Store root refuses without initialization.
		current, err := r.checkHistory()
		if err != nil {
			return err
		}
		if err := native.storeStage(current, writableRoots); err != nil {
			return err
		}
	}
	// Reobserve after input I/O (and Store I/O when the role carries the bounded
	// Store stage hook). Holding both leases fences new State/producer effects;
	// the stopped per-unit cgroup proves the old invocation has no users.
	if err := checkNodeDutyRepairQuiescence(ctx, native, declaration, writableRoots); err != nil {
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

func checkNodeDutyRepairRoots(native nodeDutyRepairNative, writableRoots []string) error {
	// writableRoots is the complete ordered platform write set (network,
	// participation, spend, hosting, plus the role's own root where present); it
	// already includes the State and local-role roots, so no extra roots are
	// appended. Descriptor's fifth entry is the Store root whose uncertain stage
	// the bounded repair may clear.
	if native.rootCount != 0 && len(writableRoots) != native.rootCount {
		return native.role.unavailable
	}
	roots := writableRoots
	for i, root := range roots {
		for _, other := range roots[i+1:] {
			if root == other || strings.HasPrefix(root, other+"/") || strings.HasPrefix(other, root+"/") {
				return native.role.unavailable
			}
		}
		info, err := os.Lstat(root)
		if err != nil || info.Mode() != os.ModeDir|0700 {
			return native.role.unavailable
		}
		stat, ok := info.Sys().(*syscall.Stat_t)
		if !ok || stat.Uid != uint32(os.Geteuid()) || stat.Gid != uint32(os.Getegid()) {
			return native.role.unavailable
		}
		for parent := filepath.Dir(root); ; parent = filepath.Dir(parent) {
			info, err := os.Lstat(parent)
			if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 || info.Mode().Perm()&0022 != 0 {
				return native.role.unavailable
			}
			stat, ok := info.Sys().(*syscall.Stat_t)
			if !ok || stat.Uid != 0 {
				return native.role.unavailable
			}
			if parent == "/" {
				break
			}
		}
	}
	return nil
}

func checkNodeDutyRepairQuiescence(ctx context.Context, native nodeDutyRepairNative, declaration string, writableRoots []string) error {
	unit, service, err := native.observe()
	if err != nil {
		return err
	}
	if err := nodeDutyRepairStoppedPolicy(unit, service, writableRoots, native.role); err != nil {
		return err
	}
	if !nodeDutyRepairDeclaration(service, declaration, native.role) {
		return native.role.unavailable
	}
	// Per-unit cgroup only: the shared ardents-node account forbids an account-wide
	// process scan (sibling Node roles legitimately run under the same principal).
	return native.quiesce(ctx)
}

func nodeDutyRepairDeclaration(service systemd.Properties, declaration string, role nodeDutyRepairRole) bool {
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
	if !strings.HasPrefix(program, "/usr/lib/ardents/") || filepath.Clean(program) != program || len(args) != 5 || args[0] != program || args[1] != "node" || args[2] != role.name || args[3] != "--retained-config" || args[4] != declaration {
		return false
	}
	return len(flags) == 1 && flags[0] == "no-env-expand"
}

func nodeDutyRepairStoppedPolicy(unit, service systemd.Properties, writableRoots []string, role nodeDutyRepairRole) error {
	unitName := "ardents-" + role.name + ".service"
	for name, value := range map[string]string{"Id": unitName, "LoadState": "loaded", "FragmentPath": "/etc/systemd/system/" + unitName} {
		if !systemd.Matches(unit, name, "s", value) {
			return role.unavailable
		}
	}
	if !(systemd.Matches(unit, "ActiveState", "s", "inactive") && systemd.Matches(unit, "SubState", "s", "dead")) &&
		!(systemd.Matches(unit, "ActiveState", "s", "failed") && systemd.Matches(unit, "SubState", "s", "failed")) {
		return role.unavailable
	}
	if !systemd.Matches(unit, "DropInPaths", "as", []string{}) || !systemd.Matches(unit, "TriggeredBy", "as", []string{}) {
		return role.unavailable
	}
	for name, value := range map[string]string{"User": "ardents-node", "Group": "ardents-node", "KillMode": "control-group", "Restart": "no", "ProtectSystem": "strict"} {
		if !systemd.Matches(service, name, "s", value) {
			return role.unavailable
		}
	}
	for _, name := range []string{"MainPID", "ControlPID"} {
		if !systemd.Matches(service, name, "u", uint32(0)) {
			return role.unavailable
		}
	}
	for _, name := range []string{"NoNewPrivileges", "ProtectControlGroups"} {
		if !systemd.Matches(service, name, "b", true) {
			return role.unavailable
		}
	}
	for _, name := range []string{"SendSIGKILL", "DynamicUser", "Delegate"} {
		if !systemd.Matches(service, name, "b", false) {
			return role.unavailable
		}
	}
	if !systemd.Matches(service, "ReadWritePaths", "as", writableRoots) {
		return role.unavailable
	}
	if !systemd.Matches(service, "ControlGroup", "s", "") && !systemd.Matches(service, "ControlGroup", "s", "/system.slice/"+unitName) {
		return role.unavailable
	}
	return nil
}
