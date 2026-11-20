//go:build linux

package node

import (
	"context"
	"errors"
	"path/filepath"
	"time"

	"github.com/dianabuilds/ardents-network/internal/network/state"
	"github.com/dianabuilds/ardents-network/internal/node/hosting"
)

// Run owns one Node duty through its bounded terminal-cleanup attempt. A failed
// result reports cleanup that did not complete or could not be proven.
func Run(ctx context.Context, input Config) (result Result, runErr error) {
	config, err := resolveConfig(input)
	if err != nil {
		return Result{}, err
	}
	if err := config.openClosedHosting(); err != nil {
		return Result{}, err
	}
	config.cleanup = &dutyCleanup{host: hosting.NewLifetime(config.host), release: func() error { return releaseLocalDuty(config) }}
	defer func() {
		if !config.cleanup.attempted {
			runErr = errors.Join(runErr, config.cleanup.Close())
		}
	}()
	machine := stateMachine{current: stateAbsent}
	if err := emitState(config, machine, state.NodeDuty{}, "process started"); err != nil {
		return Result{State: stateNames[stateFailed], Reason: err.Error()}, err
	}
	ticker := time.NewTicker(config.PollInterval)
	defer ticker.Stop()
	for {
		snapshot, currentErr := currentFacts(config.Current)
		if currentErr != nil {
			return fail(config, &machine, nil, "persistent Network State is unavailable", currentErr)
		}
		admission := assessAdmission(config, snapshot)
		switch admission.kind {
		case admissionFailed:
			return fail(config, &machine, nil, admission.reason, errors.New(admission.reason))
		case admissionReady:
			if err := retainLocalDuty(config, snapshot, "prepared"); err != nil {
				return fail(config, &machine, nil, "local role state is unavailable", err)
			}
			config.cleanup.retained = true
			return runDuty(ctx, config, &machine, snapshot)
		case admissionPrepared:
			if machine.current == stateAbsent {
				if err := retainLocalDuty(config, snapshot, "prepared"); err != nil {
					return fail(config, &machine, nil, "local role state is unavailable", err)
				}
				config.cleanup.retained = true
				if err := moveAndEmit(config, &machine, statePrepared, snapshot, admission.reason); err != nil {
					return fail(config, &machine, nil, "external evidence channel failed", err)
				}
			}
		case admissionAbsent:
			if machine.current == statePrepared {
				return fail(config, &machine, nil, admission.reason, errors.New(admission.reason))
			}
		}
		select {
		case <-ctx.Done():
			return terminalWithoutDuty(config, &machine, snapshot, ctx.Err())
		case <-ticker.C:
		}
	}
}

// The shared period is opened once for the complete Node lifecycle, before
// listener activation, and retained until every admitted child has joined.
func (config *runtimeConfig) openClosedHosting() error {
	closed := config.ClosedForwarding.Certificate.PrivateKey != nil || config.ClosedIssuer.Certificate.PrivateKey != nil ||
		config.ClosedResolution.Certificate.PrivateKey != nil || config.ClosedIntroduction.Certificate.PrivateKey != nil ||
		config.ClosedDataJoin.Certificate.PrivateKey != nil
	if !closed {
		return nil
	}
	root := config.HostingRoot
	for _, selected := range []string{config.ClosedForwarding.HostingRoot, config.ClosedDataJoin.HostingRoot} {
		if selected == "" {
			continue
		}
		if root != "" && root != selected {
			return errors.New("node duties selected different host periods")
		}
		root = selected
	}
	if root == "" || !filepath.IsAbs(root) || filepath.Clean(root) != root {
		return errors.New("closed Node hosting period unavailable")
	}
	config.measurementOrigin = time.Now()
	var err error
	config.host, err = hosting.Open(root)
	return err
}
