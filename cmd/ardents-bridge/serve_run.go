package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"github.com/dianabuilds/ardents-network/internal/bridge"
	"github.com/dianabuilds/ardents-network/internal/camouflage"
	"github.com/dianabuilds/ardents-network/internal/network/state"
	"github.com/dianabuilds/ardents-network/internal/node"
)

func runServe(ctx context.Context, path string, output io.Writer) (runErr error) {
	runtime, err := loadServePlan(path)
	if err != nil {
		return fmt.Errorf("load serve plan: %w", err)
	}
	defer func() { runErr = errors.Join(runErr, runtime.bridge.close()) }()
	runtime.bridge.config.ValidateCandidate = candidateCommitment
	owner, err := bridge.Open(runtime.bridge.config)
	if err != nil {
		return err
	}
	defer func() { runErr = errors.Join(runErr, owner.Close()) }()
	identity, envelope, err := owner.Contact()
	if err != nil {
		return err
	}
	candidate, err := camouflage.Validate(envelope, identity)
	if err != nil {
		return err
	}
	snapshot, err := runtime.bridge.config.CurrentNetwork()
	if err != nil {
		return err
	}
	runtime.node.NetworkID, runtime.node.NodeID = snapshot.NetworkID, identity
	runtime.node.Current = func() (node.Facts, error) {
		current, currentErr := runtime.bridge.config.CurrentNetwork()
		return nodeFacts(current), currentErr
	}
	events := make(chan node.Event, 16)
	runtime.node.Emit = func(eventCtx context.Context, event node.Event) error {
		select {
		case events <- event:
			return nil
		case <-eventCtx.Done():
			return eventCtx.Err()
		}
	}
	nodeCtx, cancelNode := context.WithCancel(ctx)
	defer cancelNode()
	nodeDone := make(chan error, 1)
	go func() { _, runErr := node.Run(nodeCtx, runtime.node); nodeDone <- runErr }()
	encoder := json.NewEncoder(output)
	var serving adapterServer
	for {
		select {
		case event := <-events:
			if err := encoder.Encode(event); err != nil {
				cancelNode()
				return errors.Join(err, stopAdapter(serving), <-nodeDone)
			}
			if event.Kind == "lifecycle" && event.State == "READY" && serving == nil {
				serving, err = camouflage.Serve(nodeCtx, candidate, runtime.server)
				if err != nil {
					cancelNode()
					return errors.Join(err, <-nodeDone)
				}
				if err := encoder.Encode(map[string]string{"kind": "adapter", "state": "READY"}); err != nil {
					cancelNode()
					return errors.Join(err, stopAdapter(serving), <-nodeDone)
				}
			}
			if serving != nil && event.Kind == "resource" && event.State == "PROTECT" {
				serving.Protect(true)
			}
			if serving != nil && event.Kind == "resource" && event.State == "NORMAL" {
				serving.Protect(false)
			}
			if serving != nil && (event.State == "DRAIN" || event.State == "DRAINING" ||
				event.State == "WITHDRAWN" || event.State == "FAILED") {
				err = errors.Join(err, serving.Close())
			}
		case err := <-nodeDone:
			return errors.Join(err, stopAdapter(serving))
		case <-ctx.Done():
			cancelNode()
			return errors.Join(ctx.Err(), stopAdapter(serving), <-nodeDone)
		}
	}
}

func candidateCommitment(raw []byte, identity [32]byte) ([32]byte, string, error) {
	config, err := camouflage.Validate(raw, identity)
	return config.Commitment(), "webtunnel-v0.0.6", err
}

func stopAdapter(serving adapterServer) error {
	if serving == nil {
		return nil
	}
	return serving.Close()
}

func nodeFacts(value state.Snapshot) node.Facts {
	return node.Facts{Generation: value.Generation, NetworkID: value.NetworkID, Epoch: value.Epoch, Digest: value.Digest,
		EpochValidFrom: value.EpochValidFrom, ValidUntil: value.ValidUntil, Profile: value.Profile,
		Conflicting: value.Conflicting, RecordPresent: value.RecordPresent, NodeID: value.NodeID,
		NodePublicKey: value.NodePublicKey, RecordValidFrom: value.RecordValidFrom, RecordValidUntil: value.RecordValidUntil,
		DeclaredFamily: value.DeclaredFamily, ProbeEndpoint: value.ProbeEndpoint, ProbeCapacity: value.ProbeCapacity,
		Assignment: value.Assignment, AssignmentDigest: value.AssignmentDigest, Fresh: value.Freshness == "fresh"}
}
