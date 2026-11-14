package descriptor

import (
	"encoding/json"
	"errors"
	"os"
	"slices"
)

func captureStartNativeContext(identity *runtimeIdentity, manager string, namespace *startNamespace, completed *completedInitial) ([]byte, error) {
	if identity == nil || !identity.requireManagerNamespaces || manager == "" || len(manager) > 288 || namespace == nil || completed == nil {
		return nil, ErrInput
	}
	check := func() error {
		if err := errors.Join(identity.check(), namespace.check()); err != nil {
			return err
		}
		if completed != nil {
			if err := (Completed{completed}).Check(); err != nil {
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
	groups := [][]*inputDirectory{namespace.parents.dirs, completed.parents.dirs}
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
	context := nativeContextSnapshot{"ardents-descriptor-native-context-v1", manager, managerProcess, identity.started, digest(identity.bootRaw), namespaces, append([]string(nil), namespace.paths...), ordered, absent}
	raw, err := json.Marshal(context)
	if err != nil || len(raw)+1 > 64<<10 {
		return nil, errors.Join(ErrBinding, err)
	}
	if err := check(); err != nil {
		return nil, err
	}
	return append(raw, '\n'), nil
}
