package main

import (
	"context"
	"errors"
	"io"
	"os"

	"github.com/dianabuilds/ardents-network/internal/network/state"
	"github.com/dianabuilds/ardents-network/internal/node"
)

func runNode(ctx context.Context, path string, output io.Writer) error {
	boundedOutput, ok := output.(*os.File)
	if !ok {
		return errors.New("node lifecycle output does not support write deadlines")
	}
	runtime, err := readNodePlan(path)
	if err != nil {
		return err
	}
	store, err := state.Open(runtime.state)
	if err != nil {
		return err
	}
	runtime.node.Current = func() (node.Facts, error) {
		snapshot, currentErr := store.Current()
		if currentErr != nil {
			return node.Facts{}, currentErr
		}
		return node.Facts{
			Generation: snapshot.Generation, NetworkID: snapshot.NetworkID,
			Epoch: snapshot.Epoch, Digest: snapshot.Digest,
			EpochValidFrom: snapshot.EpochValidFrom, ValidUntil: snapshot.ValidUntil,
			Profile: snapshot.Profile, Freshness: snapshot.Freshness, Conflicting: snapshot.Conflicting,
			RecordPresent: snapshot.RecordPresent, NodeID: snapshot.NodeID,
			NodePublicKey: snapshot.NodePublicKey, RecordValidFrom: snapshot.RecordValidFrom,
			RecordValidUntil: snapshot.RecordValidUntil, ProbeEndpoint: snapshot.ProbeEndpoint,
			ProbeCapacity: snapshot.ProbeCapacity, Assignment: snapshot.Assignment,
			AssignmentDigest: snapshot.AssignmentDigest,
		}, nil
	}
	runtime.node.Emit = node.EventEmitter(boundedOutput)
	_, runErr := node.Run(ctx, runtime.node)
	if closeErr := store.Close(); runErr == nil {
		runErr = closeErr
	}
	return runErr
}
