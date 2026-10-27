package forwarding

import (
	"errors"
	"os"
	"path/filepath"
)

// Unit-search custody belongs to this independently acquired Startup snapshot.
// It reuses only its own physical parent descriptors, never an installer's root.
func pinStartupNamespace(selection *runtimeSelection, paths []string) ([]string, error) {
	if selection == nil || len(paths) == 0 || len(paths) > 32 {
		return nil, ErrInput
	}
	f := selection.files
	var absent []string
	for _, directory := range paths {
		if !validPath(directory) {
			return nil, ErrBinding
		}
		prefix := directory
		for {
			_, err := os.Lstat(prefix)
			if err == nil {
				if _, err := f.parents(filepath.Join(prefix, "observation")); err != nil {
					return nil, err
				}
				break
			}
			if !errors.Is(err, os.ErrNotExist) || prefix == "/" {
				return nil, errors.Join(ErrBinding, err)
			}
			absent = append(absent, prefix)
			prefix = filepath.Dir(prefix)
		}
		for _, role := range []string{"forwarding", "issuer", "descriptor", "introduction", "join"} {
			for _, suffix := range []string{".service", ".service.d", ".socket", ".socket.d"} {
				path := filepath.Join(directory, "ardents-"+role+suffix)
				if path == "/etc/systemd/system/ardents-forwarding.service" {
					continue
				}
				absent = append(absent, path)
			}
		}
		for _, name := range []string{"ardents-.service.d", "service.d", "socket.d"} {
			absent = append(absent, filepath.Join(directory, name))
		}
	}
	if err := checkStartupNamespace(selection, absent); err != nil {
		return nil, err
	}
	return absent, nil
}

func checkStartupNamespace(selection *runtimeSelection, absent []string) error {
	if err := selection.check(); err != nil {
		return err
	}
	for _, path := range absent {
		if _, err := os.Lstat(path); !errors.Is(err, os.ErrNotExist) {
			return errors.Join(ErrBinding, err)
		}
	}
	return selection.check()
}

func (s *runtimeSelection) pinStartupDirectory(path string) error {
	f := s.files
	if f.dirs[path] != nil {
		return ErrBinding
	}
	parent, err := f.parents(path)
	if err != nil {
		return err
	}
	file, err := openInputAt(parent, filepath.Base(path), true)
	if err != nil {
		return err
	}
	d := &inputDirectory{path: path, name: filepath.Base(path), file: file, parent: parent}
	f.dirs[path], f.dirOrder = d, append(f.dirOrder, d)
	d.info, err = file.Stat()
	if err != nil || !nodeAccess(d.info, 0750, 0, s.binding.GID, true) {
		return errors.Join(ErrBinding, err)
	}
	if err := runtimeDirectoryMount(d, true); err != nil {
		return err
	}
	s.immutable = append(s.immutable, path)
	return s.check()
}
