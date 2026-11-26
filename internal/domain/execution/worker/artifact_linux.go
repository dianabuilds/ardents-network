//go:build linux

package worker

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"syscall"

	"github.com/dianabuilds/ardents-network/internal/canonicaldigest"
	"github.com/dianabuilds/ardents-network/internal/pathgrammar"
)

type Artifact struct {
	inv        inventory
	files      map[string][32]byte
	rootDevice uint64
	rootInode  uint64
}

// loadArtifact reads only the root-installed local artifact
// reference. Application input can neither select an inventory nor supply a digest.
func loadArtifact(inv inventory) (*Artifact, error) {
	body, err := readInstalledFile(inv.manifest, 16<<10)
	if err != nil {
		return nil, err
	}
	var manifest struct {
		Schema string            `json:"schema"`
		Files  map[string]string `json:"files"`
	}
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&manifest) != nil || manifest.Schema != inv.schema || len(manifest.Files) != 6 {
		return nil, errors.New("document worker artifact reference is invalid")
	}
	canonical, err := json.Marshal(manifest)
	if err != nil || !bytes.Equal(bytes.TrimSpace(body), canonical) {
		return nil, errors.New("document worker artifact reference is not canonical")
	}
	var extra any
	if decoder.Decode(&extra) != io.EOF {
		return nil, errors.New("document worker artifact reference has trailing data")
	}
	artifact := &Artifact{inv: inv, files: make(map[string][32]byte, 6)}
	paths := []string{inv.root + "/" + inv.prefix, inv.rule}
	for _, role := range []string{"reader", "publisher"} {
		paths = append(paths, "/etc/systemd/system/"+inv.prefix+"-"+role+"@.service", "/etc/systemd/system/"+inv.prefix+"-"+role+".socket")
	}
	for _, path := range paths {
		encoded, present := manifest.Files[path]
		digest, ok := canonicaldigest.Decode(encoded)
		if !present || !ok {
			return nil, errors.New("document worker artifact digest is invalid")
		}
		if digest == [32]byte{} {
			return nil, errors.New("document worker artifact digest is absent")
		}
		artifact.files[path] = digest
	}
	if err := artifact.Verify(); err != nil {
		return nil, err
	}
	return artifact, nil
}

// Verify rechecks the installed artifact bytes and the immutable private root
// against the pinned digests. The first successful call pins the root device
// and inode; a later substituted root is refused.
func (artifact *Artifact) Verify() error {
	if artifact == nil || len(artifact.files) != 6 {
		return errors.New("document worker artifact is unavailable")
	}
	root, err := installedPath(artifact.inv.root, true)
	if err != nil || root.Mode().Perm() != 0555 {
		return errors.New("document worker immutable root is unavailable")
	}
	identity, ok := root.Sys().(*syscall.Stat_t)
	if !ok {
		return errors.New("document worker root identity is unavailable")
	}
	if artifact.rootInode != 0 && (artifact.rootDevice != uint64(identity.Dev) || artifact.rootInode != identity.Ino) {
		return errors.New("document worker private root was substituted")
	}
	if err := verifyRootInventory(artifact.inv); err != nil {
		return err
	}
	for path, digest := range artifact.files {
		limit := int64(64 << 10)
		if path == artifact.inv.root+"/"+artifact.inv.prefix {
			limit = 64 << 20
		}
		body, err := readInstalledFile(path, limit)
		if err != nil || sha256.Sum256(body) != digest {
			return errors.New("document worker installed artifact was substituted")
		}
	}
	if artifact.rootInode == 0 {
		artifact.rootDevice, artifact.rootInode = uint64(identity.Dev), identity.Ino
	}
	return nil
}

// Every ancestor is root-controlled and has no symlink or writable untrusted
// component. The final open additionally refuses symlinks and checks identity.
func installedPath(path string, directory bool) (os.FileInfo, error) {
	if !pathgrammar.Strict(path) {
		return nil, errors.New("document worker installed path is invalid")
	}
	for current := path; ; current = filepath.Dir(current) {
		info, err := os.Lstat(current)
		if err != nil {
			return nil, errors.New("document worker installed path is unavailable")
		}
		stat, ok := info.Sys().(*syscall.Stat_t)
		if !ok || stat.Uid != 0 || info.Mode()&os.ModeSymlink != 0 || info.Mode().Perm()&0022 != 0 ||
			(current != path && !info.IsDir()) || (current == path && directory && !info.IsDir()) {
			return nil, errors.New("document worker installed path is not root-controlled")
		}
		if current == "/" {
			break
		}
	}
	return os.Lstat(path)
}

func readInstalledFile(path string, maximum int64) ([]byte, error) {
	before, err := installedPath(path, false)
	if err != nil || !before.Mode().IsRegular() || before.Size() < 0 || before.Size() > maximum {
		return nil, errors.New("document worker installed file is invalid")
	}
	fd, err := syscall.Open(path, syscall.O_RDONLY|syscall.O_CLOEXEC|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0)
	if err != nil {
		return nil, errors.New("document worker installed file is unavailable")
	}
	file := os.NewFile(uintptr(fd), path)
	defer file.Close()
	opened, err := file.Stat()
	if err != nil || !os.SameFile(before, opened) || !opened.Mode().IsRegular() {
		return nil, errors.New("document worker installed file changed")
	}
	body, readErr := io.ReadAll(io.LimitReader(file, maximum+1))
	after, statErr := file.Stat()
	closeErr := file.Close()
	if readErr != nil || statErr != nil || closeErr != nil || int64(len(body)) != before.Size() ||
		!os.SameFile(before, after) || !before.ModTime().Equal(after.ModTime()) || before.Size() != after.Size() {
		return nil, errors.New("document worker installed file changed or could not be read")
	}
	return body, nil
}

// These are empty base mount points required by the selected systemd
// confinement. No host data or arbitrary subtree is admitted into the image.
func verifyRootInventory(inv inventory) error {
	directories := map[string]bool{"dev": true, "proc": true, "sys": true, "run": true, "tmp": true,
		"etc": true, "root": true, "usr": true, "var": true, "var/tmp": true}
	seen := 0
	err := filepath.WalkDir(inv.root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return errors.New("document worker root inventory is unavailable")
		}
		if path == inv.root {
			return nil
		}
		relative, err := filepath.Rel(inv.root, path)
		if err != nil {
			return errors.New("document worker root path is invalid")
		}
		directory := directories[relative]
		if !directory && relative != inv.prefix {
			return errors.New("document worker root contains an extra resource")
		}
		info, err := installedPath(path, directory)
		if err != nil || info.Mode().Perm() != 0555 || (!directory && !info.Mode().IsRegular()) {
			return errors.New("document worker root resource is mutable or invalid")
		}
		seen++
		return nil
	})
	if err != nil || seen != len(directories)+1 {
		return errors.New("document worker root inventory is invalid")
	}
	return nil
}
