package command

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"

	"github.com/dianabuilds/ardents-network/internal/domain/endpoint"
	"github.com/dianabuilds/ardents-network/internal/domain/network/source"
	"github.com/dianabuilds/ardents-network/internal/domain/network/state"
	"github.com/dianabuilds/ardents-network/internal/pathcheck"
	"github.com/dianabuilds/ardents-network/internal/platform/artifacts"
)

var (
	errPreparationPath  = errors.New("invalid Endpoint preparation path")
	errPreparationInput = errors.New("invalid Endpoint preparation input")
)

// preparePlansFromFiles selects original local declarations and writes the
// fresh Endpoint plan pair. State independently authenticates retained history.
func preparePlansFromFiles(ctx context.Context, networkFile, sourceFile, declarationFile, outputDirectory string) error {
	for _, path := range []string{networkFile, sourceFile, declarationFile, outputDirectory} {
		if !pathcheck.IsCleanAbsolute(path) {
			return errPreparationPath
		}
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	ownerRaw, err := artifacts.ReadBoundedFile(networkFile, 8<<10)
	if err != nil {
		return errPreparationInput
	}
	var owner state.AuthorityDeclaration
	if artifacts.DecodeObject(ownerRaw, &owner) != nil {
		return errPreparationInput
	}
	// The established local Network owner projection builds the reference
	// configuration; the constructor itself refuses pins that do not open the
	// accepted retained history.
	config, err := owner.Config()
	if err != nil {
		return errPreparationInput
	}
	sourceRaw, err := artifacts.ReadBoundedFile(sourceFile, 32<<10)
	if err != nil {
		return errPreparationInput
	}
	var declared endpoint.SourcePlan
	if artifacts.DecodeObject(sourceRaw, &declared) != nil {
		return errPreparationInput
	}
	declarationRaw, err := artifacts.ReadBoundedFile(declarationFile, 8<<10)
	if err != nil {
		return errPreparationInput
	}
	var declaration endpoint.PlanDeclaration
	if artifacts.DecodeObject(declarationRaw, &declaration) != nil {
		return errPreparationInput
	}
	return endpoint.PreparePlans(ctx, config, declared, declaration, outputDirectory)
}

// prepareHistoriesFromFiles reads the bounded inputs for first Endpoint
// preparation. The declaration supplies storage targets and pins only.
func prepareHistoriesFromFiles(ctx context.Context, planFile, bundleFile, profileFile string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	raw, err := artifacts.ReadBoundedFile(planFile, 16<<10)
	if err != nil {
		return errPreparationInput
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	if artifacts.UniqueJSON(decoder) != nil {
		return errPreparationInput
	}
	if _, err := decoder.Token(); err != io.EOF {
		return errPreparationInput
	}
	var plan endpoint.HeadlessPlan
	if json.Unmarshal(raw, &plan) != nil {
		return errPreparationInput
	}
	bundleRaw, err := artifacts.ReadBoundedFile(bundleFile, 1<<20)
	if err != nil {
		return errPreparationInput
	}
	bundle, err := source.DecodeBundle(bundleRaw)
	if err != nil {
		return errPreparationInput
	}
	profile, err := artifacts.ReadBoundedFile(profileFile, 64<<10)
	if err != nil {
		return errPreparationInput
	}
	return endpoint.PrepareHistories(ctx, plan, bundle, profile)
}
