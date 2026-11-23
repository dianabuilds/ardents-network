//go:build linux

package main

import (
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"errors"
	"os"
	"time"

	artifactio "github.com/dianabuilds/ardents-network/internal/platform/artifacts"
)

func signingAncestors(p *preparationPath) error {
	if err := p.CheckRootAccess(); err != nil {
		return errors.Join(errNetworkSigning, err)
	}
	return nil
}

func parseNetworkSigningKey(raw []byte) (ed25519.PrivateKey, error) {
	key, err := artifactio.ParseEd25519Key(raw)
	if err != nil {
		return nil, errNetworkSigning
	}
	return key, nil
}

func executeNetworkSigning(ctx context.Context, args []string) (*networkSigningResult, error) {
	return executeObservedNetworkSigning(ctx, args, nil)
}

func executeObservedNetworkSigning(ctx context.Context, args []string, observe func(string, *os.File)) (result *networkSigningResult, err error) {
	if os.Geteuid() != 0 {
		return nil, errNetworkSigning
	}
	var admitted *networkSigning
	defer func() {
		err = errors.Join(err, ctx.Err())
		if admitted != nil {
			err = errors.Join(err, admitted.checkTime(time.Now()))
		}
	}()
	plan, err := artifactio.OpenProtectedInput(ctx, args[1], maximumNetworkPreparationPlan, false)
	if err != nil {
		return nil, err
	}
	defer func() { err = errors.Join(err, plan.Close()) }()
	prepared, err := artifactio.OpenProtectedInput(ctx, args[2], maximumNetworkPreparationPlan, false)
	if err != nil {
		return nil, err
	}
	defer func() { err = errors.Join(err, prepared.Close()) }()
	operation, err := admitNetworkSigning(args[0], plan.Bytes(), prepared.Bytes(), args[3])
	if err != nil {
		return nil, err
	}
	admitted = operation
	if err = operation.checkTime(time.Now()); err != nil {
		return nil, err
	}
	secret, err := artifactio.OpenProtectedInput(ctx, args[4], 16<<10, true)
	if err != nil {
		return nil, err
	}
	defer func() { err = errors.Join(err, secret.Close()) }()
	key, err := parseNetworkSigningKey(secret.Bytes())
	if err != nil {
		return nil, err
	}
	defer clear(key)
	if observe != nil {
		observe("inputs-read", secret.Descriptor())
	}
	check := func() error {
		return errors.Join(plan.Check(ctx), prepared.Check(ctx), secret.Check(ctx), operation.checkTime(time.Now()), ctx.Err())
	}
	if err = check(); err != nil {
		return nil, err
	}
	artifacts, err := operation.sign(key, time.Now())
	if err != nil {
		return nil, err
	}
	if err = check(); err != nil {
		return nil, err
	}
	if observe != nil {
		observe("signed", secret.Descriptor())
	}
	err = persistNetworkSigning(ctx, args[5], artifacts, check, observe)
	return &networkSigningResult{digest: sha256.Sum256(artifacts[0].raw), interval: operation.networkSigningInterval}, err
}

// Public signed artifacts borrow the native finite-file mechanism without
// granting it signing or semantic authority.
func persistNetworkSigning(ctx context.Context, path string, artifacts []networkSignedArtifact, check func() error, observe func(string, *os.File)) error {
	files := make([]networkOutputFile, 0, len(artifacts))
	for _, a := range artifacts {
		files = append(files, networkOutputFile(a))
	}
	return persistNetworkFiles(ctx, path, files, false, check, observe)
}
