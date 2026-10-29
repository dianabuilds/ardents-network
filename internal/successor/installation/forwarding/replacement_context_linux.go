package forwarding

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"slices"
)

type nativeObject struct {
	Path   string `json:"path"`
	Device uint64 `json:"device"`
	Inode  uint64 `json:"inode"`
	Mode   uint32 `json:"mode"`
	UID    uint32 `json:"uid"`
	GID    uint32 `json:"gid"`
}

type nativeContextSnapshot struct {
	Schema     string         `json:"schema"`
	Manager    string         `json:"manager"`
	Process    nativeObject   `json:"process"`
	Started    uint64         `json:"started"`
	BootSHA256 string         `json:"boot_sha256"`
	Namespaces []nativeObject `json:"namespaces"`
	UnitPaths  []string       `json:"unit_paths"`
	Ancestors  []nativeObject `json:"ancestors"`
	Absent     []string       `json:"absent"`
}

func generatorDirectory(path string) bool {
	return path == "/run/systemd/generator" || path == "/run/systemd/generator.early" || path == "/run/systemd/generator.late"
}

func decodeNativeContext(raw []byte) (nativeContextSnapshot, error) {
	var snapshot nativeContextSnapshot
	if len(raw) == 0 || len(raw) > 64<<10 || json.Unmarshal(raw, &snapshot) != nil {
		return snapshot, ErrBinding
	}
	canonical, err := json.Marshal(snapshot)
	if err != nil || !bytes.Equal(raw, append(canonical, '\n')) || snapshot.Schema != "ardents-forwarding-native-context-v1" || snapshot.Manager == "" || len(snapshot.Manager) > 288 || snapshot.Process.Path != "/proc/1" || snapshot.Process.Device == 0 || snapshot.Process.Inode == 0 || snapshot.Started == 0 || !validStartDigest(snapshot.BootSHA256) || len(snapshot.Namespaces) != 5 || len(snapshot.UnitPaths) == 0 || len(snapshot.UnitPaths) > 32 {
		return snapshot, errors.Join(ErrBinding, err)
	}
	for n, name := range []string{"mnt", "pid", "cgroup", "uts", "user"} {
		object := snapshot.Namespaces[n]
		if object.Path != "/proc/self/ns/"+name || object.Device == 0 || object.Inode == 0 {
			return snapshot, ErrBinding
		}
	}
	paths := map[string]bool{}
	for _, path := range snapshot.UnitPaths {
		if !validPath(path) || paths[path] {
			return snapshot, ErrBinding
		}
		paths[path] = true
	}
	paths = map[string]bool{}
	for n, object := range snapshot.Ancestors {
		if object.Path != "/" && !validPath(object.Path) || object.Device == 0 || object.Inode == 0 || n != 0 && snapshot.Ancestors[n-1].Path >= object.Path {
			return snapshot, ErrBinding
		}
		if generatorDirectory(object.Path) {
			mode := os.FileMode(object.Mode)
			if !mode.IsDir() || mode.Perm()&0022 != 0 || mode&(os.ModeSymlink|os.ModeSetuid|os.ModeSetgid|os.ModeSticky) != 0 || object.UID != 0 || object.GID != 0 {
				return snapshot, ErrBinding
			}
		}
		paths[object.Path] = true
	}
	if !paths["/"] {
		return snapshot, ErrBinding
	}
	for n, path := range snapshot.Absent {
		if !validPath(path) || paths[path] || n != 0 && snapshot.Absent[n-1] >= path {
			return snapshot, ErrBinding
		}
	}
	return snapshot, nil
}

// Only these owned reload outputs may differ. This comparison is byte/native
// provenance, not proof that any reload, original cut or recovery succeeded.
func compareReloadContexts(original, current []byte) error {
	before, err := decodeNativeContext(original)
	if err != nil {
		return err
	}
	after, err := decodeNativeContext(current)
	if err != nil {
		return err
	}
	strip := func(snapshot *nativeContextSnapshot) {
		snapshot.Ancestors = slices.DeleteFunc(snapshot.Ancestors, func(object nativeObject) bool { return generatorDirectory(object.Path) })
		snapshot.Absent = slices.DeleteFunc(snapshot.Absent, generatorDirectory)
	}
	strip(&before)
	strip(&after)
	a, _ := json.Marshal(before)
	b, _ := json.Marshal(after)
	if !bytes.Equal(a, b) {
		return ErrBinding
	}
	return nil
}

// The hash is reconstructed from independently retained original objects. No
// detached context object can install a generation or authorize continuation.
func replacementContext(identity *runtimeIdentity, manager string, namespace *startNamespace, input *inputs, files *completedFiles) ([]byte, error) {
	if input == nil || files == nil {
		return nil, ErrInput
	}
	return captureNativeContext(identity, manager, namespace, input, files)
}

// Only concrete original observers enter this physical snapshot. Initial birth
// retains Inputs; start/stop retain Completed; replacement retains both. This
// private function supplies no caller-selected policy or accepting authority.
func captureNativeContext(identity *runtimeIdentity, manager string, namespace *startNamespace, input *inputs, files *completedFiles) ([]byte, error) {
	if identity == nil || !identity.requireManagerNamespaces || manager == "" || len(manager) > 288 || namespace == nil || input == nil && files == nil {
		return nil, ErrInput
	}
	check := func() error {
		if err := errors.Join(identity.check(), namespace.check()); err != nil {
			return err
		}
		if input != nil {
			if err := input.links(); err != nil {
				return err
			}
		}
		if files != nil {
			if err := files.check(); err != nil {
				return err
			}
		}
		return nil
	}
	if err := check(); err != nil {
		return nil, err
	}
	object := func(path string, info os.FileInfo) (nativeObject, error) {
		native, ok := infoSys(info)
		if !ok || native.Dev == 0 || native.Ino == 0 {
			return nativeObject{}, ErrBinding
		}
		return nativeObject{path, uint64(native.Dev), native.Ino, uint32(info.Mode()), native.Uid, native.Gid}, nil
	}
	var namespaces []nativeObject
	for _, name := range []string{"mnt", "pid", "cgroup", "uts", "user"} {
		info, err := identity.namespaces[name].Stat()
		if err != nil {
			return nil, err
		}
		entry, err := object("/proc/self/ns/"+name, info)
		if err != nil {
			return nil, err
		}
		namespaces = append(namespaces, entry)
	}
	ancestors := map[string]nativeObject{}
	retain := func(path string, file *os.File) error {
		info, err := file.Stat()
		if err != nil {
			return err
		}
		entry, err := object(path, info)
		if err != nil {
			return err
		}
		if old, exists := ancestors[path]; exists && old != entry {
			return ErrBinding
		}
		ancestors[path] = entry
		return nil
	}
	groups := [][]*inputDirectory{namespace.parents.dirs}
	if input != nil {
		groups = append(groups, input.dirs)
	}
	if files != nil {
		groups = append(groups, files.dirOrder)
		if files.recoveryContext != nil {
			if err := files.recoveryContext.check(); err != nil {
				return nil, err
			}
			for path, file := range files.recoveryContext.dirs {
				if err := retain(path, file); err != nil {
					return nil, err
				}
			}
		}
	}
	for _, directories := range groups {
		for _, directory := range directories {
			if err := retain(directory.path, directory.file); err != nil {
				return nil, err
			}
		}
	}
	var paths []string
	for path := range ancestors {
		paths = append(paths, path)
	}
	slices.Sort(paths)
	var ordered []nativeObject
	for _, path := range paths {
		ordered = append(ordered, ancestors[path])
	}
	managerProcess, err := object("/proc/1", identity.managerInfo)
	if err != nil {
		return nil, err
	}
	absent := slices.Clone(namespace.absent)
	slices.Sort(absent)
	absent = slices.Compact(absent)
	context := nativeContextSnapshot{"ardents-forwarding-native-context-v1", manager, managerProcess, identity.started, digest(identity.bootRaw), namespaces, append([]string(nil), namespace.paths...), ordered, absent}
	raw, err := json.Marshal(context)
	if err != nil || len(raw)+1 > 64<<10 {
		return nil, errors.Join(ErrBinding, err)
	}
	if err := check(); err != nil {
		return nil, err
	}
	return append(raw, '\n'), nil
}
