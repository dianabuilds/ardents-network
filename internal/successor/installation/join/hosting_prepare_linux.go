package join

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"syscall"

	"github.com/dianabuilds/ardents-network/internal/successor/hosting"
	"github.com/dianabuilds/ardents-network/internal/successor/installation/cgroup"
	"github.com/dianabuilds/ardents-network/internal/successor/installation/systemd"
)

type hostingBirth struct {
	Schema  string `json:"schema"`
	Binding string `json:"binding_sha256"`
	Device  uint64 `json:"device"`
	Inode   uint64 `json:"inode"`
	Policy  string `json:"policy_sha256"`
}

type hostingFileIdentity struct {
	Device uint64 `json:"device"`
	Inode  uint64 `json:"inode"`
}

type hostingCompletion struct {
	Schema string              `json:"schema"`
	Intent string              `json:"intent_sha256"`
	Pin    hostingFileIdentity `json:"pin"`
	Lock   hostingFileIdentity `json:"lock"`
}

func hostingPreparationPaths(root string) (directory, journal, pending string) {
	return root + ".join-hosting", root + ".join-hosting-journal", root + ".join-hosting-pending"
}

// PrepareHosting initializes only the original empty allowance directory born
// by this completed stopped installation. Its own root-only one-use transcript
// survives outside the Node-writable budget. It grants no Startup, runtime,
// reservation, retry, replacement or recovery permission. Every original user
// closes before the final blocker unlink; partial results remain blocked.
func PrepareHosting(ctx context.Context, bindingPath string, policy hosting.Policy) (returned error) {
	return prepareHosting(ctx, bindingPath, policy, nil)
}

// The observer is a failure-only native test boundary after real file creation
// or before original physical close. It supplies no accepting fact or policy.
func prepareHosting(ctx context.Context, bindingPath string, policy hosting.Policy, observe func(string, *os.File)) (returned error) {
	if ctx == nil || os.Geteuid() != 0 || !validPath(bindingPath) {
		return ErrInput
	}
	if _, bounded := ctx.Deadline(); !bounded {
		return ErrInput
	}
	if err := errors.Join(policy.Validate(), ctx.Err()); err != nil {
		return err
	}
	completed, err := ReadCompleted(ctx, bindingPath)
	if err != nil {
		return err
	}
	var births *initialBirths
	var root *os.Root
	var manager *systemd.JoinStopped
	var namespace *startNamespace
	var identity *runtimeIdentity
	var pending *initialFile
	var pendingParent os.FileInfo
	complete := false
	defer func() {
		if observe != nil {
			observe("before-close", completed.original.directoryAt(completed.original.binding.Request.HostingRoot).file)
		}
		if root != nil {
			returned = errors.Join(returned, root.Close())
		}
		returned = errors.Join(returned, manager.Close(), namespace.close(), identity.close(), births.close(), completed.Close(), ctx.Err())
		if returned == nil && complete {
			returned = publishInitialPending(births, pending, pendingParent)
		}
		if returned != nil && births != nil && births.journal != nil {
			returned = errors.Join(ErrRepairRequired, returned)
		}
	}()
	c := completed.original
	b := c.binding
	directory, journalPath, pendingPath := hostingPreparationPaths(b.Request.InstallationRoot)
	root, err = os.OpenRoot(b.Request.HostingRoot)
	if err != nil {
		return err
	}
	rootInfo, err := root.Stat(".")
	original := c.directoryAt(b.Request.HostingRoot).info
	if err != nil || !os.SameFile(original, rootInfo) || !nodeAccess(rootInfo, 0700, b.UID, b.GID, true) {
		return errors.Join(ErrBinding, err)
	}
	probe, err := root.Open(".")
	if err != nil {
		return err
	}
	entries, readErr := probe.ReadDir(1)
	closeErr := probe.Close()
	if readErr != nil && readErr != io.EOF || closeErr != nil || len(entries) != 0 {
		return errors.Join(ErrBinding, readErr, closeErr)
	}
	identity, err = openRootIdentity(ctx)
	if err != nil {
		return err
	}
	manager, err = systemd.OpenJoinInstalledStopped(ctx)
	if err != nil {
		return err
	}
	paths, err := manager.UnitPaths()
	if err != nil {
		return err
	}
	namespace, err = openStartNamespace(ctx, paths)
	if err != nil {
		return err
	}
	managerID, err := manager.ManagerIdentity()
	if err != nil {
		return err
	}
	check := func() error {
		if err := errors.Join(completed.Check(), identity.check(), namespace.check(), cgroup.CheckJoinQuiescent(ctx)); err != nil {
			return err
		}
		currentManager, err := manager.ManagerIdentity()
		if err != nil || currentManager != managerID {
			return errors.Join(ErrBinding, err)
		}
		for n := 0; n < attemptLimit; n++ {
			for _, path := range []string{filepath.Join(b.Request.InstallationRoot, fmt.Sprintf("join-invocation-%08d", n)), filepath.Join(b.Request.InstallationRoot, fmt.Sprintf("join-retirement-%08d", n)), filepath.Join(b.Request.InstallationRoot, "join-startup")} {
				if _, err := os.Lstat(path); !errors.Is(err, os.ErrNotExist) {
					return errors.Join(ErrBinding, err)
				}
			}
		}
		current, err := root.Stat(".")
		if err != nil || !os.SameFile(original, current) || !nodeAccess(current, 0700, b.UID, b.GID, true) {
			return errors.Join(ErrBinding, err)
		}
		unit, service, err := manager.ObserveLoaded()
		if err != nil {
			return err
		}
		return errors.Join(verifyJoinStopped(unit, service, b.Request, b.Generation), completed.Check(), ctx.Err())
	}
	if err := check(); err != nil {
		return err
	}
	for _, path := range []string{directory, journalPath, pendingPath} {
		if _, err := os.Lstat(path); !errors.Is(err, os.ErrNotExist) {
			return errors.Join(ErrRepairRequired, err)
		}
	}
	pendingParent, err = os.Lstat(filepath.Dir(pendingPath))
	if err != nil || !trustedInputDirectory(pendingParent) {
		return errors.Join(ErrBinding, err)
	}
	allowed := map[string]bool{directory: true, journalPath: true, pendingPath: true, b.Request.HostingRoot: true, filepath.Join(directory, "intent.json"): true, filepath.Join(directory, "completed.json"): true}
	births, err = openJoinBirths(ctx, journalPath, allowed, "join-hosting-v1")
	c.hostingBirths = births
	if err != nil {
		return err
	}
	native := original.Sys().(*syscall.Stat_t)
	policyRaw, err := json.Marshal(policy)
	if err != nil {
		return err
	}
	intent := hostingBirth{"ardents-join-hosting-intent-v1", digest(c.files[bindingPath].body), uint64(native.Dev), native.Ino, digest(policyRaw)}
	raw, err := json.Marshal(intent)
	if err != nil {
		return err
	}
	raw = append(raw, '\n')
	if err := births.write(pendingPath, []byte("ardents-join-hosting-pending-v1\n"+digest(raw)+"\n"), 0600, 0, 0, check); err != nil {
		return err
	}
	pending = births.files[len(births.files)-1]
	if err := births.directory(directory, 0700, 0, 0, check); err != nil {
		return err
	}
	if err := births.write(filepath.Join(directory, "intent.json"), raw, 0600, 0, 0, check); err != nil {
		return err
	}
	if err := births.record("admitted", b.Request.HostingRoot, original, intent.Policy, 0700, b.UID, b.GID); err != nil {
		return err
	}
	if err := check(); err != nil {
		return err
	}
	files, err := hosting.InitializeFiles(ctx, root, policy)
	if err != nil {
		return err
	}
	if observe != nil {
		observe("files-created", c.directoryAt(b.Request.HostingRoot).file)
	}
	result := hostingCompletion{Schema: "ardents-join-hosting-completed-v1", Intent: digest(raw)}
	for _, name := range []string{"budget.pin", "budget.lock", "budget.json"} {
		if err := check(); err != nil {
			return err
		}
		info, err := promoteHostingFile(root, name, files.Original(name), b.UID, b.GID)
		if err != nil {
			return err
		}
		n := info.Sys().(*syscall.Stat_t)
		id := hostingFileIdentity{uint64(n.Dev), n.Ino}
		if name == "budget.pin" {
			result.Pin = id
		}
		if name == "budget.lock" {
			result.Lock = id
		}
	}
	if err := check(); err != nil {
		return err
	}
	resultRaw, err := json.Marshal(result)
	if err != nil {
		return err
	}
	resultRaw = append(resultRaw, '\n')
	if err := births.record("selected", b.Request.HostingRoot, original, digest(resultRaw), 0700, b.UID, b.GID); err != nil {
		return err
	}
	if err := births.write(filepath.Join(directory, "completed.json"), resultRaw, 0600, 0, 0, check); err != nil {
		return err
	}
	if err := check(); err != nil {
		return err
	}
	if err := births.record("completed", directory, births.identities[directory], digest(resultRaw), 0700, 0, 0); err != nil {
		return err
	}
	complete = true
	return nil
}

func promoteHostingFile(root *os.Root, name string, original os.FileInfo, uid, gid uint32) (result os.FileInfo, returned error) {
	before, err := root.Lstat(name)
	if err != nil || original == nil || !sameInputFile(original, before) || !nodeAccess(before, 0600, 0, 0, false) {
		return nil, errors.Join(ErrBinding, err)
	}
	file, err := root.OpenFile(name, os.O_RDWR|syscall.O_NOFOLLOW, 0)
	if err != nil {
		return nil, err
	}
	defer func() { returned = errors.Join(returned, file.Close()) }()
	opened, err := file.Stat()
	if err != nil || !os.SameFile(before, opened) {
		return nil, errors.Join(ErrBinding, err)
	}
	if err := file.Chown(int(uid), int(gid)); err != nil {
		return nil, err
	}
	if err := file.Sync(); err != nil {
		return nil, err
	}
	result, err = file.Stat()
	named, nameErr := root.Lstat(name)
	if err != nil || nameErr != nil || !os.SameFile(before, result) || !os.SameFile(result, named) || !nodeAccess(result, 0600, uid, gid, false) || !nodeAccess(named, 0600, uid, gid, false) {
		return nil, errors.Join(ErrBinding, err, nameErr)
	}
	return result, nil
}
