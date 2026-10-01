//go:build linux

package installation

import (
	"context"
	"errors"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"syscall"
)

type preparedInstallation struct {
	uid, gid uint32
	roots    []rootBinding
}

type preparationRecord struct {
	Schema           string `json:"schema"`
	GenerationDigest string `json:"generation_digest"`
	RequestDigest    string `json:"request_digest"`
	Phase            string `json:"phase"`
	UID              uint32 `json:"uid,omitempty"`
	GID              uint32 `json:"gid,omitempty"`
	OriginalError    string `json:"original_error,omitempty"`
}

func prepareInitialInstallation(ctx context.Context, request Request, authorization Authorization) (prepared preparedInstallation, returnedErr error) {
	if ctx == nil || os.Geteuid() != 0 {
		return preparedInstallation{}, errors.New("installation preparation requires root and context")
	}
	if _, ok := authorization.program.AcceptedDecision(); !ok {
		return preparedInstallation{}, errors.New("installation preparation lacks executable authorization")
	}
	if _, ok := authorization.generation.AcceptedDecision(); !ok {
		return preparedInstallation{}, errors.New("installation preparation lacks generation authorization")
	}
	if err := preflightInitialInstallation(ctx, request); err != nil {
		return preparedInstallation{}, err
	}
	if err := ctx.Err(); err != nil {
		return preparedInstallation{}, err
	}
	root := request.InstallationRoot
	if err := os.Mkdir(root, 0700); err != nil {
		return preparedInstallation{}, err
	}
	if err := syncDirectory(filepath.Dir(root)); err != nil {
		return preparedInstallation{}, err
	}
	journal := filepath.Join(root, "preparation")
	if err := os.Mkdir(journal, 0700); err != nil {
		return preparedInstallation{}, err
	}
	if err := syncDirectory(root); err != nil {
		return preparedInstallation{}, err
	}
	raw, err := canonicalJSON(request)
	if err != nil {
		return preparedInstallation{}, err
	}
	record := preparationRecord{Schema: "ardents-endpoint-installation-preparation-v1", GenerationDigest: digestHex(authorization.descriptor), RequestDigest: digestHex(raw), Phase: "creating-account"}
	writeRecord := func(name string) error {
		body, err := canonicalJSON(record)
		if err != nil {
			return err
		}
		if err := writeExclusiveGenerationFile(filepath.Join(journal, name), body, 0600, 0); err != nil {
			return err
		}
		return syncDirectory(journal)
	}
	if err := writeRecord("0001.json"); err != nil {
		return preparedInstallation{}, err
	}
	defer func() {
		if returnedErr != nil {
			record.Phase, record.OriginalError = "preparation-failed", returnedErr.Error()
			returnedErr = errors.Join(returnedErr, writeRecord("failure.json"))
		}
	}()
	if _, err := runInstallationCommand(ctx, "/usr/sbin/useradd", "--system", "--user-group", "--no-create-home", "--home-dir", "/nonexistent", "--shell", "/usr/sbin/nologin", "ardents-endpoint"); err != nil {
		return preparedInstallation{}, err
	}
	account, err := user.Lookup("ardents-endpoint")
	if err != nil {
		return preparedInstallation{}, err
	}
	uid, uidErr := strconv.ParseUint(account.Uid, 10, 32)
	gid, gidErr := strconv.ParseUint(account.Gid, 10, 32)
	group, groupErr := user.LookupGroup("ardents-endpoint")
	groups, groupsErr := account.GroupIds()
	if uidErr != nil || gidErr != nil || uid == 0 || gid == 0 || groupErr != nil || group.Gid != account.Gid || groupsErr != nil || len(groups) != 1 || groups[0] != account.Gid {
		return preparedInstallation{}, errors.New("created Endpoint account identity or supplementary groups differ")
	}
	prepared.uid, prepared.gid = uint32(uid), uint32(gid)
	record.UID, record.GID, record.Phase = prepared.uid, prepared.gid, "creating-mutable-roots"
	if err := writeRecord("0002.json"); err != nil {
		return preparedInstallation{}, err
	}
	created := map[string]bool{}
	for _, path := range writableDirectories(request) {
		if err := ctx.Err(); err != nil {
			return preparedInstallation{}, err
		}
		if err := createPrivateManagedDirectory(path, prepared.uid, prepared.gid, created); err != nil {
			return preparedInstallation{}, err
		}
	}
	for _, path := range mutableRoots(request.Headless) {
		info, err := os.Lstat(path)
		if err != nil {
			return preparedInstallation{}, err
		}
		identity, ok := info.Sys().(*syscall.Stat_t)
		if !ok || identity.Uid != prepared.uid || identity.Gid != prepared.gid || info.Mode().Perm() != 0700 {
			return preparedInstallation{}, errors.New("prepared mutable root access differs")
		}
		prepared.roots = append(prepared.roots, rootBinding{Path: path, Device: uint64(identity.Dev), Inode: identity.Ino})
	}
	if err := os.Mkdir(filepath.Join(root, "generations"), 0750); err != nil {
		return preparedInstallation{}, err
	}
	if err := os.Chown(filepath.Join(root, "generations"), 0, int(prepared.gid)); err != nil {
		return preparedInstallation{}, err
	}
	if err := os.Chmod(filepath.Join(root, "generations"), 0750); err != nil {
		return preparedInstallation{}, err
	}
	if err := syncDirectory(filepath.Join(root, "generations")); err != nil {
		return preparedInstallation{}, err
	}
	if err := os.Chown(root, 0, int(prepared.gid)); err != nil {
		return preparedInstallation{}, err
	}
	if err := os.Chmod(root, 0750); err != nil {
		return preparedInstallation{}, err
	}
	if err := syncDirectory(root); err != nil {
		return preparedInstallation{}, err
	}
	record.Phase = "mutable-roots-prepared"
	if err := writeRecord("0003.json"); err != nil {
		return preparedInstallation{}, err
	}
	return prepared, nil
}

func createPrivateManagedDirectory(path string, uid, gid uint32, created map[string]bool) error {
	if !canonicalPath(path) || uid == 0 || gid == 0 {
		return errors.New("private managed directory declaration is invalid")
	}
	if created[path] {
		info, err := os.Lstat(path)
		if err != nil {
			return err
		}
		identity, ok := info.Sys().(*syscall.Stat_t)
		if !info.IsDir() || !ok || identity.Uid != uid || identity.Gid != gid || info.Mode().Perm() != 0700 {
			return errors.New("previously created private directory differs")
		}
		return nil
	}
	if _, err := os.Lstat(path); !os.IsNotExist(err) {
		return errors.New("private managed directory already exists or is unavailable")
	}
	var missing []string
	for parent := filepath.Dir(path); ; parent = filepath.Dir(parent) {
		info, err := os.Lstat(parent)
		if os.IsNotExist(err) {
			missing = append(missing, parent)
			continue
		}
		if err != nil {
			return err
		}
		identity, ok := info.Sys().(*syscall.Stat_t)
		if !info.IsDir() || !ok || info.Mode().Perm()&0022 != 0 || identity.Uid != 0 && !(created[parent] && identity.Uid == uid && identity.Gid == gid) {
			return errors.New("private managed directory parent is untrusted")
		}
		if filepath.Dir(parent) == parent {
			break
		}
	}
	for i := len(missing) - 1; i >= 0; i-- {
		if err := os.Mkdir(missing[i], 0755); err != nil {
			return err
		}
		if err := os.Chmod(missing[i], 0755); err != nil {
			return err
		}
		if err := syncDirectory(filepath.Dir(missing[i])); err != nil {
			return err
		}
	}
	if err := os.Mkdir(path, 0700); err != nil {
		return err
	}
	if err := os.Chown(path, int(uid), int(gid)); err != nil {
		return err
	}
	if err := syncDirectory(path); err != nil {
		return err
	}
	if err := syncDirectory(filepath.Dir(path)); err != nil {
		return err
	}
	created[path] = true
	return nil
}
