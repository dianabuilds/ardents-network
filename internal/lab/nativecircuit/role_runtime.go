package nativecircuit

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"path/filepath"
	"sync"
	"time"
)

func runConfiguredRole(ctx context.Context, config roleConfig, evidenceDir string) (runErr error) {
	started := time.Now()
	result := newRoleResult(config)
	defer func() {
		result.finish(started, runErr)
		runErr = errors.Join(runErr, writeRoleResult(filepath.Join(evidenceDir, "result.json"), result))
	}()
	if config.Profile == directProfile && config.Role == "user" {
		runErr = runDirectUserRole(ctx, config, evidenceDir, &result)
	} else if config.Profile == directProfile {
		runErr = runDirectServiceRole(ctx, config, evidenceDir, &result)
	} else if isRelayRole(config.Role) || config.Role == "rendezvous" || config.Role == "introduction-node" {
		runErr = runNodeRole(ctx, config, evidenceDir, &result)
	} else if config.Role == "user" {
		runErr = runUserRole(ctx, config, evidenceDir, &result)
	} else {
		runErr = runServiceRole(ctx, config, evidenceDir, &result)
	}
	if runErr != nil {
		return runErr
	}
	return waitForRoleStart(ctx, "/control/capture-cleanup")
}

func runNodeRole(ctx context.Context, config roleConfig, evidenceDir string, result *roleResult) error {
	certificate, err := tls.LoadX509KeyPair(config.CertificatePath, config.PrivateKeyPath)
	if err != nil {
		return fmt.Errorf("load %s Node identity: %w", config.Role, err)
	}
	listener, err := (&net.ListenConfig{}).Listen(ctx, "tcp", config.ListenAddress)
	if err != nil {
		return fmt.Errorf("listen as %s: %w", config.Role, err)
	}
	if err := writeRoleReady(evidenceDir, config); err != nil {
		_ = listener.Close()
		return err
	}
	var mu sync.Mutex
	observe := func(field string) {
		mu.Lock()
		defer mu.Unlock()
		result.ObservedFields = append(result.ObservedFields, field)
	}
	nodeContext, cancel := context.WithCancel(ctx)
	defer cancel()
	completed := make(chan error, 1)
	go func() {
		if isRelayRole(config.Role) {
			completed <- serveRelayObserved(nodeContext, listener, certificate, config.AllowedNext, config.ExpectedConnections, observe)
			return
		}
		if config.Role == "rendezvous" {
			completed <- serveRendezvousObserved(nodeContext, listener, certificate, config.ExpectedConnections, observe)
			return
		}
		completed <- serveIntroductionObserved(nodeContext, listener, certificate, config.ExpectedConnections, observe)
	}()
	stopArrived := make(chan error, 1)
	go func() { stopArrived <- waitForRoleStart(ctx, "/control/stop") }()
	select {
	case err := <-completed:
		return err
	case err := <-stopArrived:
		if err != nil {
			cancel()
			<-completed
			return err
		}
		cancel()
		return ignoreControlledNodeStop(<-completed)
	}
}

func ignoreControlledNodeStop(err error) error {
	if err == nil || errors.Is(err, context.Canceled) || errors.Is(err, net.ErrClosed) {
		return nil
	}
	return err
}

func runUserRole(ctx context.Context, config roleConfig, evidenceDir string, result *roleResult) error {
	plan, err := loadUserPlan(config)
	if err != nil {
		return err
	}
	if plan.Attached != nil {
		defer plan.Attached.Close()
	}
	if err := writeRoleReady(evidenceDir, config); err != nil {
		return err
	}
	if err := waitForRoleStart(ctx, config.StartPath); err != nil {
		return err
	}
	plan.SetupVerified = func() error { return writeRoleMarker(evidenceDir, config, "setup-ready.json", "authenticated") }
	if config.Fault == "rendezvous-process" {
		plan.FirstChunkVerified = func() error {
			if err := writeRoleMarker(evidenceDir, config, "stream-ready.json", "first_chunk_verified"); err != nil {
				return err
			}
			return waitForRoleStart(ctx, "/control/stream-continue")
		}
	}
	observation, err := runCandidateUser(ctx, plan)
	result.applyEndpoint(observation)
	result.ObservedFields = append(result.ObservedFields,
		"introduction.acknowledgement", "rendezvous.joined", "target.instance_certificate", "application.protected_stream",
	)
	if err != nil {
		result.FailureKind = candidateFailureKind(err)
		return err
	}
	return writeRoleMarker(evidenceDir, config, "attempt-ready.json", "completed")
}

func runServiceRole(ctx context.Context, config roleConfig, evidenceDir string, result *roleResult) error {
	plan, err := loadServicePlan(config)
	if err != nil {
		return err
	}
	if plan.Attached != nil {
		defer plan.Attached.Close()
	}
	if err := waitForRoleStart(ctx, config.StartPath); err != nil {
		return err
	}
	plan.Registered = func() error { return writeRoleReady(evidenceDir, config) }
	observation, err := runCandidateService(ctx, plan)
	result.applyEndpoint(observation)
	result.ObservedFields = append(result.ObservedFields,
		"introduction.invitation_plaintext", "rendezvous.joined", "application.protected_stream",
	)
	if err != nil {
		result.FailureKind = candidateFailureKind(err)
		return err
	}
	return writeRoleMarker(evidenceDir, config, "attempt-ready.json", "completed")
}

func waitForRoleStart(ctx context.Context, path string) error {
	ticker := time.NewTicker(20 * time.Millisecond)
	defer ticker.Stop()
	deadline := time.NewTimer(5 * time.Minute)
	defer deadline.Stop()
	for {
		if info, err := netFileStat(path); err == nil && info {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-deadline.C:
			return errors.New("native role start marker did not arrive before the setup deadline")
		case <-ticker.C:
		}
	}
}
