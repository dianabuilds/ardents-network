//go:build linux

package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"

	"github.com/dianabuilds/ardents-network/internal/endpoint"
)

func preflight(ctx context.Context, arguments []string, output io.Writer) error {
	if len(arguments) != 1 {
		return errors.New("usage: ardents-qualification preflight <local-plan.json>")
	}
	artifact, err := verifyQualificationEndpointArtifact(arguments[0])
	if err != nil {
		return err
	}
	file, err := os.Open(arguments[0])
	if err != nil {
		return err
	}
	digest := sha256.New()
	plan, decodeErr := decodePlan(io.TeeReader(file, digest))
	if err := errors.Join(decodeErr, file.Close()); err != nil {
		return err
	}
	selections := make([]endpoint.StreamQualificationPreflight, 0, len(plan.Configs))
	for _, config := range plan.Configs {
		selection, err := endpoint.PreflightStreamQualification(ctx, config)
		if err != nil {
			return err
		}
		selections = append(selections, selection)
	}
	return json.NewEncoder(output).Encode(struct {
		Kind         string
		Mode         string
		PlanSHA256   string
		Participants int
		Selections   []endpoint.StreamQualificationPreflight
		Artifact     qualificationEndpointArtifact
	}{"qualification-preflight", plan.Mode, hex.EncodeToString(digest.Sum(nil)), len(plan.Configs), selections, artifact})
}
