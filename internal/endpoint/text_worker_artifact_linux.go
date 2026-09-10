//go:build linux

package endpoint

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"syscall"
)

const textWorkerArtifactPath = "/etc/ardents/text-worker-artifact.json"
const textWorkerStopRulePath = "/usr/share/polkit-1/rules.d/50-ardents-text.rules"

type textWorkerArtifact struct {
	files      map[string][32]byte
	rootDevice uint64
	rootInode  uint64
}

// loadTextWorkerArtifact reads only the root-installed local artifact reference.
// It is never populated from AAI3, worker readiness, a remote response or an
// Application-supplied digest. Root installation is the selected trust boundary.
func loadTextWorkerArtifact() (*textWorkerArtifact, error) {
	body, err := readTextInstalledFile(textWorkerArtifactPath, 16<<10)
	if err != nil {
		return nil, err
	}
	var manifest struct {
		Schema string            `json:"schema"`
		Files  map[string]string `json:"files"`
	}
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&manifest) != nil || manifest.Schema != "ardents-text-worker-artifact-v1" || len(manifest.Files) != 6 {
		return nil, errors.New("text worker artifact reference is invalid")
	}
	canonical, err := json.Marshal(manifest)
	if err != nil || !bytes.Equal(bytes.TrimSpace(body), canonical) {
		return nil, errors.New("text worker artifact reference is not canonical")
	}
	var extra any
	if decoder.Decode(&extra) != io.EOF {
		return nil, errors.New("text worker artifact reference has trailing data")
	}
	artifact := &textWorkerArtifact{files: make(map[string][32]byte, 6)}
	paths := []string{textWorkerRoot + "/ardents-text", textWorkerStopRulePath}
	for _, role := range []string{"reader", "publisher"} {
		paths = append(paths, "/etc/systemd/system/ardents-text-"+role+"@.service", "/etc/systemd/system/ardents-text-"+role+".socket")
	}
	for _, path := range paths {
		encoded, ok := manifest.Files[path]
		decoded, err := hex.DecodeString(encoded)
		if !ok || err != nil || len(decoded) != 32 || hex.EncodeToString(decoded) != encoded {
			return nil, errors.New("text worker artifact digest is invalid")
		}
		var digest [32]byte
		copy(digest[:], decoded)
		if digest == [32]byte{} {
			return nil, errors.New("text worker artifact digest is absent")
		}
		artifact.files[path] = digest
	}
	if err := artifact.verify(); err != nil {
		return nil, err
	}
	return artifact, nil
}

func (artifact *textWorkerArtifact) verify() error {
	if artifact == nil || len(artifact.files) != 6 {
		return errors.New("text worker artifact is unavailable")
	}
	root, err := textInstalledPath(textWorkerRoot, true)
	if err != nil || root.Mode().Perm() != 0555 {
		return errors.New("text worker immutable root is unavailable")
	}
	identity, ok := root.Sys().(*syscall.Stat_t)
	if !ok {
		return errors.New("text worker root identity is unavailable")
	}
	if artifact.rootInode != 0 && (artifact.rootDevice != uint64(identity.Dev) || artifact.rootInode != identity.Ino) {
		return errors.New("text worker private root was substituted")
	}
	if err := verifyTextWorkerRootInventory(); err != nil {
		return err
	}
	for path, digest := range artifact.files {
		limit := int64(64 << 10)
		if path == textWorkerRoot+"/ardents-text" {
			limit = 64 << 20
		}
		body, err := readTextInstalledFile(path, limit)
		if err != nil || sha256.Sum256(body) != digest {
			return errors.New("text worker installed artifact was substituted")
		}
	}
	if artifact.rootInode == 0 {
		artifact.rootDevice, artifact.rootInode = uint64(identity.Dev), identity.Ino
	}
	return nil
}

// Every ancestor is root-controlled and has no symlink or writable untrusted
// component. The final open additionally refuses symlinks and checks identity.
func textInstalledPath(path string, directory bool) (os.FileInfo, error) {
	if !filepath.IsAbs(path) || filepath.Clean(path) != path {
		return nil, errors.New("text worker installed path is invalid")
	}
	for current := path; ; current = filepath.Dir(current) {
		info, err := os.Lstat(current)
		if err != nil {
			return nil, errors.New("text worker installed path is unavailable")
		}
		stat, ok := info.Sys().(*syscall.Stat_t)
		if !ok || stat.Uid != 0 || info.Mode()&os.ModeSymlink != 0 || info.Mode().Perm()&0022 != 0 ||
			(current != path && !info.IsDir()) || (current == path && directory && !info.IsDir()) {
			return nil, errors.New("text worker installed path is not root-controlled")
		}
		if current == "/" {
			break
		}
	}
	return os.Lstat(path)
}

func readTextInstalledFile(path string, maximum int64) ([]byte, error) {
	before, err := textInstalledPath(path, false)
	if err != nil || !before.Mode().IsRegular() || before.Size() < 0 || before.Size() > maximum {
		return nil, errors.New("text worker installed file is invalid")
	}
	fd, err := syscall.Open(path, syscall.O_RDONLY|syscall.O_CLOEXEC|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0)
	if err != nil {
		return nil, errors.New("text worker installed file is unavailable")
	}
	file := os.NewFile(uintptr(fd), path)
	defer file.Close()
	opened, err := file.Stat()
	if err != nil || !os.SameFile(before, opened) || !opened.Mode().IsRegular() {
		return nil, errors.New("text worker installed file changed")
	}
	body, readErr := io.ReadAll(io.LimitReader(file, maximum+1))
	after, statErr := file.Stat()
	closeErr := file.Close()
	if readErr != nil || statErr != nil || closeErr != nil || int64(len(body)) != before.Size() ||
		!os.SameFile(before, after) || !before.ModTime().Equal(after.ModTime()) || before.Size() != after.Size() {
		return nil, errors.New("text worker installed file changed or could not be read")
	}
	return body, nil
}

// These are empty base mount points required by systemd 255's selected
// ProtectSystem/ProtectHome/PrivateTmp/API-filesystem mounts. No host data or
// arbitrary subtree is admitted into the immutable base image.
func verifyTextWorkerRootInventory() error {
	directories := map[string]bool{"dev": true, "proc": true, "sys": true, "run": true, "tmp": true,
		"etc": true, "root": true, "usr": true, "var": true, "var/tmp": true}
	seen := 0
	err := filepath.WalkDir(textWorkerRoot, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return errors.New("text worker root inventory is unavailable")
		}
		if path == textWorkerRoot {
			return nil
		}
		relative, err := filepath.Rel(textWorkerRoot, path)
		if err != nil {
			return errors.New("text worker root path is invalid")
		}
		directory := directories[relative]
		if !directory && relative != "ardents-text" {
			return errors.New("text worker root contains an extra resource")
		}
		info, err := textInstalledPath(path, directory)
		if err != nil || info.Mode().Perm() != 0555 || (!directory && !info.Mode().IsRegular()) {
			return errors.New("text worker root resource is mutable or invalid")
		}
		seen++
		return nil
	})
	if err != nil || seen != len(directories)+1 {
		return errors.New("text worker root inventory is invalid")
	}
	return nil
}
