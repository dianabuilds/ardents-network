package endpoint

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"

	"github.com/dianabuilds/ardents-network/internal/domain/network/source"
	"github.com/dianabuilds/ardents-network/internal/domain/network/state"
	"github.com/dianabuilds/ardents-network/internal/pathcheck"
	"github.com/dianabuilds/ardents-network/internal/platform/artifacts"
)

var (
	ErrPreparationPath  = errors.New("invalid Endpoint preparation path")
	ErrPreparationInput = errors.New("invalid Endpoint preparation input")
)

// PreparePlansFromFiles selects original local declarations and writes the
// fresh Endpoint plan pair. State independently authenticates retained history.
func PreparePlansFromFiles(ctx context.Context, networkFile, sourceFile, declarationFile, outputDirectory string) error {
	for _, path := range []string{networkFile, sourceFile, declarationFile, outputDirectory} {
		if !pathcheck.IsCleanAbsolute(path) {
			return ErrPreparationPath
		}
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	ownerRaw, err := artifacts.ReadBoundedFile(networkFile, 8<<10)
	if err != nil {
		return ErrPreparationInput
	}
	var owner state.AuthorityDeclaration
	if artifacts.DecodeObject(ownerRaw, &owner) != nil {
		return ErrPreparationInput
	}
	// The established local Network owner projection builds the reference
	// configuration; the constructor itself refuses pins that do not open the
	// accepted retained history.
	config, err := owner.Config()
	if err != nil {
		return ErrPreparationInput
	}
	sourceRaw, err := artifacts.ReadBoundedFile(sourceFile, 32<<10)
	if err != nil {
		return ErrPreparationInput
	}
	var declared SourcePlan
	if artifacts.DecodeObject(sourceRaw, &declared) != nil {
		return ErrPreparationInput
	}
	declarationRaw, err := artifacts.ReadBoundedFile(declarationFile, 8<<10)
	if err != nil {
		return ErrPreparationInput
	}
	var declaration PlanDeclaration
	if artifacts.DecodeObject(declarationRaw, &declaration) != nil {
		return ErrPreparationInput
	}
	return PreparePlans(ctx, config, declared, declaration, outputDirectory)
}

// PrepareHistoriesFromFiles reads the bounded inputs for first Endpoint
// preparation. The declaration supplies storage targets and pins only.
func PrepareHistoriesFromFiles(ctx context.Context, planFile, bundleFile, profileFile string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	raw, err := artifacts.ReadBoundedFile(planFile, 16<<10)
	if err != nil {
		return ErrPreparationInput
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	if artifacts.UniqueJSON(decoder) != nil {
		return ErrPreparationInput
	}
	if _, err := decoder.Token(); err != io.EOF {
		return ErrPreparationInput
	}
	var plan HeadlessPlan
	if json.Unmarshal(raw, &plan) != nil {
		return ErrPreparationInput
	}
	bundleRaw, err := artifacts.ReadBoundedFile(bundleFile, 1<<20)
	if err != nil {
		return ErrPreparationInput
	}
	bundle, err := source.DecodeBundle(bundleRaw)
	if err != nil {
		return ErrPreparationInput
	}
	profile, err := artifacts.ReadBoundedFile(profileFile, 64<<10)
	if err != nil {
		return ErrPreparationInput
	}
	return PrepareHistories(ctx, plan, bundle, profile)
}
