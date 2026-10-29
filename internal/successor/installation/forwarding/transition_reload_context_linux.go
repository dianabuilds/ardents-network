package forwarding

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
)

// Only the actual initial/replacement caller uses this after its successful own
// Reload, refreshed native observers and stopped policy rounds. It grants no
// reload, accepts no replacement observer and never changes the original hash.
func observeTransitionReload(ctx context.Context, namespace *startNamespace) error {
	c, _ := ctx.Value(transitionContextKey{}).(*TransitionCoordinator)
	if c == nil {
		return nil
	}
	if c.closed || c.terminal != nil || c.refreshed || c.operation != "initial" && c.operation != "replacement" || c.sequence == 0 || namespace == nil {
		return ErrBinding
	}
	raw, err := nativeContextAfterReload(c.boundary.OriginalContext, namespace)
	if err != nil {
		return err
	}
	c.boundary.NativeContext = raw
	c.refreshed = true
	return nil
}

// Called only by an actual one-use reload owner after fresh namespace and
// stopped-policy observations. Byte reconstruction itself grants no effect.
func nativeContextAfterReload(original []byte, namespace *startNamespace) ([]byte, error) {
	if namespace == nil || namespace.parents == nil {
		return nil, ErrInput
	}
	snapshot, err := decodeNativeContext(original)
	if err != nil {
		return nil, err
	}
	if !slices.Equal(namespace.paths, snapshot.UnitPaths) {
		return nil, ErrBinding
	}
	if err := namespace.check(); err != nil {
		return nil, err
	}
	snapshot.Ancestors = slices.DeleteFunc(snapshot.Ancestors, func(object nativeObject) bool { return generatorDirectory(object.Path) })
	snapshot.Absent = slices.DeleteFunc(snapshot.Absent, generatorDirectory)
	for _, path := range []string{"/run/systemd/generator", "/run/systemd/generator.early", "/run/systemd/generator.late"} {
		if directory := namespace.parents.byPath[path]; directory != nil {
			info, err := directory.file.Stat()
			native, ok := infoSys(info)
			if err != nil || !ok || !trustedInputDirectory(info) {
				return nil, errors.Join(ErrBinding, err)
			}
			snapshot.Ancestors = append(snapshot.Ancestors, nativeObject{path, uint64(native.Dev), native.Ino, uint32(info.Mode()), native.Uid, native.Gid})
		} else if slices.Contains(namespace.absent, path) {
			snapshot.Absent = append(snapshot.Absent, path)
		} else {
			return nil, ErrBinding
		}
	}
	slices.SortFunc(snapshot.Ancestors, func(a, b nativeObject) int {
		if a.Path < b.Path {
			return -1
		}
		if a.Path > b.Path {
			return 1
		}
		return 0
	})
	slices.Sort(snapshot.Absent)
	raw, err := json.Marshal(snapshot)
	if err != nil || len(raw)+1 > 64<<10 {
		return nil, errors.Join(ErrBinding, err)
	}
	raw = append(raw, '\n')
	if err := errors.Join(compareReloadContexts(original, raw), namespace.check(), namespace.parents.ctx.Err()); err != nil {
		return nil, err
	}
	return raw, nil
}

func checkTransitionContexts(message transitionMessage, records []initialRecord) error {
	if len(message.OriginalContext) == 0 || len(message.OriginalContext) > 64<<10 || len(message.NativeContext) == 0 || len(message.NativeContext) > 64<<10 || len(records) == 0 {
		return ErrBinding
	}
	// Opaque unchanged observations convey no recovery authority. Fresh recovery
	// must decode and independently match both snapshots, including native context
	// even at an early prefix before that original context record was written.
	if string(message.OriginalContext) == string(message.NativeContext) {
		return nil
	}
	last := records[len(records)-1].Phase
	if message.Operation != "initial" || last != "stopped-verified" && last != "completed" {
		if message.Operation != "replacement" || last != "replacement-verified" && !transitionAfterContextBoundary(message, records) {
			return ErrBinding
		}
	}
	contextSeen, reloadSeen := false, false
	for _, record := range records {
		if record.Phase == "native-context" && record.SHA256 == digest(message.OriginalContext) {
			contextSeen = true
		}
		if record.Phase == "reload-intent" {
			reloadSeen = true
		}
	}
	if !contextSeen || !reloadSeen {
		return ErrBinding
	}
	return compareReloadContexts(message.OriginalContext, message.NativeContext)
}

func transitionAfterContextBoundary(message transitionMessage, records []initialRecord) bool {
	if len(records) == 0 || records[0].Phase != "forwarding-replacement-v2" {
		return false
	}
	last := records[len(records)-1]
	wanted := message.Journal + ".native-after"
	if last.Path != wanted || last.Phase != "file-intent" && last.Phase != "file-born" && last.Phase != "file-ready" {
		return false
	}
	for _, row := range records {
		if row.Phase == "file-intent" && row.Path == wanted && row.SHA256 == digest(message.NativeContext) {
			return true
		}
	}
	return false
}
