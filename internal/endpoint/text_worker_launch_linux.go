//go:build linux

package endpoint

import (
	"context"
	"errors"
	"net"
	"os"
	"path/filepath"
	"syscall"
	"time"
	"unicode/utf8"

	"github.com/dianabuilds/ardents-network/internal/application/broker"
	"github.com/dianabuilds/ardents-network/internal/application/streamqualification"
	"github.com/dianabuilds/ardents-network/internal/application/textdocument"
)

// The installed sockets and their PID/UID instance namespace belong to the
// process, not to one Endpoint value. Keep the inventory baseline, activation
// and identity observation serial across every Endpoint in this process.
var textWorkerLaunchGate = make(chan struct{}, 1)

// launchTextWorker owns a local job from reservation through verified readiness.
// No caller supplies a worker identity, artifact digest, isolation flag, socket,
// executable, Principal or Grant. All of those observations are obtained here.
func (owner *textContext) launchTextWorker(ctx context.Context, snapshot []byte) (*qualifiedTextWorker, error) {
	return owner.launchInstalledWorker(ctx, snapshot, nil)
}

func (owner *textContext) launchStreamQualificationWorker(ctx context.Context, profile streamqualification.Profile, seed [32]byte) (*qualifiedTextWorker, error) {
	role := streamqualification.ReaderRole
	if owner != nil && owner.surface == broker.Administration {
		role = streamqualification.PublisherRole
	}
	if _, err := profile.Definition(role); err != nil {
		return nil, err
	}
	if seed == [32]byte{} {
		return nil, errors.New("qualification workload seed is absent")
	}
	return owner.launchInstalledWorker(ctx, nil, &streamqualification.Init{Role: role, Profile: profile, Seed: seed})
}

func (owner *textContext) launchInstalledWorker(ctx context.Context, snapshot []byte, qualification *streamqualification.Init) (*qualifiedTextWorker, error) {
	if owner == nil || ctx == nil || ctx.Err() != nil {
		return nil, errors.New("text worker launch is unavailable")
	}
	role := "reader"
	if owner.surface == broker.Administration {
		role = "publisher"
	}
	if len(snapshot) > textdocument.MaximumBytes || !utf8.Valid(snapshot) || role == "reader" && len(snapshot) != 0 {
		return nil, errors.New("text worker snapshot is invalid")
	}
	job, err := owner.beginJob(owner.endpoint, owner.surface)
	if err != nil {
		return nil, err
	}
	inventory := textInventory
	if qualification != nil {
		inventory = streamInventory
		copy := *qualification
		copy.Nonce = job.nonce
		job.qualification = &copy
	}
	// Before activation a refused launch has no process-cleanup obligation.
	activated, transferred := false, false
	var connection *net.UnixConn
	var release func()
	defer func() {
		if !transferred {
			owner.retireJob(job)
			var cleanupErr error
			if connection != nil {
				cleanupErr = connection.Close()
			}
			if activated {
				cleanupErr = errors.Join(cleanupErr, errors.New("text worker activation cleanup is unverified"))
			}
			_ = owner.finishJobCleanup(job, cleanupErr)
		}
		if release != nil {
			release()
		}
	}()
	bounded, cancel := context.WithTimeout(job.context, 15*time.Second)
	defer cancel()
	stopCaller := context.AfterFunc(ctx, cancel)
	defer stopCaller()
	release, err = owner.endpoint.acquireTextLaunch(bounded)
	if err != nil {
		return nil, err
	}
	if job.context.Err() != nil {
		return nil, errors.New("text worker launch was revoked")
	}
	if err := verifyTextEndpointService(bounded); err != nil {
		return nil, err
	}
	artifact, err := loadInstalledWorkerArtifact(inventory)
	if err != nil {
		return nil, err
	}
	path := inventory.socket(role)
	socket, err := inspectTextWorkerSocket(path)
	if err != nil {
		return nil, err
	}
	before, err := listInstalledWorkerInstances(bounded, role, inventory)
	if err != nil {
		return nil, err
	}
	// Even a failed dial can have queued an activation. Retain failure if its
	// exact invocation cannot be pinned; another job may not reuse that ambiguity.
	activated = true
	raw, err := (&net.Dialer{}).DialContext(bounded, "unix", path)
	if err != nil {
		return nil, errors.New("text worker activation failed")
	}
	var ok bool
	connection, ok = raw.(*net.UnixConn)
	if !ok {
		_ = raw.Close()
		return nil, errors.New("text worker attachment is not local")
	}
	if err := prepareTextWorkerSocket(connection); err != nil {
		return nil, err
	}
	afterSocket, err := inspectTextWorkerSocket(path)
	if err != nil || !os.SameFile(socket, afterSocket) {
		return nil, errors.New("text worker activation socket changed")
	}
	observed, err := awaitInstalledWorkerInstance(bounded, before, role, inventory)
	if err != nil {
		return nil, err
	}
	attachment := &textWorkerAttachment{connection: connection, pid: observed.pid, uid: observed.uid}
	// The lifetime takes over before INIT, including failure and cancellation.
	transferred = true
	lifetime, err := initializeOwnedTextWorker(job.context, bounded, attachment, observed, job, snapshot, artifact)
	if err != nil {
		return nil, err
	}
	refuse := func(cause error) (*qualifiedTextWorker, error) { return nil, errors.Join(cause, lifetime.Close()) }
	if err := artifact.verify(); err != nil {
		return refuse(err)
	}
	current, err := observeTextWorkerInstance(bounded, observed.name, role)
	if err != nil || current != observed {
		return refuse(errors.New("text worker invocation changed before Grant"))
	}
	if bounded.Err() != nil || ctx.Err() != nil {
		return refuse(errors.New("text worker launch was cancelled"))
	}
	// Grant creation is private to this fully observed launch, after readiness
	// and before any Connection can be given to the worker.
	worker, err := owner.bindTextWorker(job, lifetime)
	if err != nil {
		return refuse(err)
	}
	return worker, nil
}

func (endpoint *endpoint) acquireTextLaunch(ctx context.Context) (func(), error) {
	select {
	case textWorkerLaunchGate <- struct{}{}:
		if ctx.Err() != nil || !endpoint.textAvailable() {
			<-textWorkerLaunchGate
			return nil, errors.New("text worker activation is unavailable")
		}
		return func() { <-textWorkerLaunchGate }, nil
	case <-ctx.Done():
		return nil, errors.New("text worker activation was cancelled")
	}
}

func inspectTextWorkerSocket(path string) (os.FileInfo, error) {
	if path != textInventory.socket("reader") && path != textInventory.socket("publisher") && path != streamInventory.socket("reader") && path != streamInventory.socket("publisher") || os.Geteuid() == 0 {
		return nil, errors.New("text worker activation address is unavailable")
	}
	if _, err := textInstalledPath(filepath.Dir(path), true); err != nil {
		return nil, err
	}
	info, err := os.Lstat(path)
	if err != nil {
		return nil, errors.New("text worker activation socket is unavailable")
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok || info.Mode()&os.ModeSocket == 0 || info.Mode().Perm() != 0600 || stat.Uid != uint32(os.Geteuid()) || stat.Gid != uint32(os.Getegid()) {
		return nil, errors.New("text worker activation socket ownership is invalid")
	}
	return info, nil
}

func awaitInstalledWorkerInstance(ctx context.Context, before textWorkerListing, role string, inventory workerInventory) (textWorkerInstance, error) {
	ticker := time.NewTicker(25 * time.Millisecond)
	defer ticker.Stop()
	var candidate string
	for {
		after, err := listInstalledWorkerInstances(ctx, role, inventory)
		if err != nil {
			return textWorkerInstance{}, err
		}
		name, err := newTextWorkerInstance(before, after, candidate)
		if err != nil {
			return textWorkerInstance{}, err
		}
		if name != "" {
			candidate = name
			observed, err := observeTextWorkerInstance(ctx, name, role)
			if err == nil {
				return observed, nil
			}
		}
		select {
		case <-ctx.Done():
			return textWorkerInstance{}, errors.New("text worker activation did not become verifiable")
		case <-ticker.C:
		}
	}
}
