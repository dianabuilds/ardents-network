//go:build linux

package provisioning

import (
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"errors"
	"github.com/dianabuilds/ardents-network/internal/domain/node/provisioning/networkdraft"
	"os"
	"time"

	artifactio "github.com/dianabuilds/ardents-network/internal/platform/artifacts"
)

func parseNetworkSigningKey(raw []byte) (ed25519.PrivateKey, error) {
	key, err := artifactio.ParseEd25519Key(raw)
	if err != nil {
		return nil, errNetworkSigning
	}
	return key, nil
}

// SignNetwork publishes the exact signed inventory after joined input checks.
// Optional observation reports the original custody cuts to qualification.
func SignNetwork(ctx context.Context, args []string, observe func(string, *os.File)) (result *NetworkSigningResult, err error) {
	if ctx == nil || len(args) != 6 || !IsNetworkSigning(args[0]) {
		return nil, errNetworkSigning
	}
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
	plan, err := artifactio.OpenProtectedInput(ctx, args[1], networkdraft.MaximumPlanBytes, false)
	if err != nil {
		return nil, err
	}
	defer func() { err = errors.Join(err, plan.Close()) }()
	prepared, err := artifactio.OpenProtectedInput(ctx, args[2], networkdraft.MaximumPlanBytes, false)
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
	files := make([]artifactio.Content, len(artifacts))
	for i, file := range artifacts {
		files[i] = artifactio.Content{Name: file.name, Data: file.raw}
	}
	err = PublishNetworkInventory(ctx, args[5], files, false, check, observe)
	return &NetworkSigningResult{Digest: sha256.Sum256(artifacts[0].raw), interval: operation.networkSigningInterval}, err
}
