//go:build linux

package endpoint

import (
	"context"
	"errors"
	"os"
	"path/filepath"

	"github.com/dianabuilds/ardents-network/internal/domain/network/state"
)

// PreparePlans authenticates and renders the operator declarations, then
// writes both canonical files into the selected output directory.
func PreparePlans(ctx context.Context, network state.Config, declared SourcePlan, declaration PlanDeclaration, directory string) error {
	if err := validateSourcePlanDeclaration(declared); err != nil {
		return err
	}
	_, headless, source, err := ConstructPlans(ctx, network, declaration, declared)
	if err != nil {
		return err
	}
	return persistConstructedPlans(ctx, directory, headless, source)
}

// validateSourcePlanDeclaration checks the distributed Source plan produced
// by source prepare-trust before opening the retained Network State.
func validateSourcePlanDeclaration(declared SourcePlan) error {
	if declared.Schema != "ardents-source-plan-v1" ||
		len(declared.Sources) != 2 || declared.OrderSeed == "" || declared.ClockObservedAt == "" ||
		declared.ClientCertificate == "" || declared.ClientKey == "" ||
		declared.RefreshIntervalMS < 100 || declared.RefreshIntervalMS > 60000 {
		return errors.New("source plan declaration is unavailable")
	}
	for _, member := range declared.Sources {
		if member.Address == "" || member.ServerName == "" || member.Identity == "" || member.Family == "" ||
			member.EndpointHandle == "" || member.RootCA == "" || member.LeafKeyDigest == "" {
			return errors.New("source plan declaration is unavailable")
		}
	}
	return nil
}

// persistConstructedPlans durably renders both plan files. Existing bytes are
// neither adopted nor overwritten: an interrupted construction is retried
// into a fresh directory, never repaired in place.
func persistConstructedPlans(ctx context.Context, directory string, headless, source []byte) (result error) {
	if err := ctx.Err(); err != nil {
		return err
	}
	if info, err := os.Lstat(directory); err != nil || !info.IsDir() {
		return errors.Join(errors.New("construction output directory is unavailable"), err)
	}
	files := []struct {
		name string
		body []byte
	}{{"headless.json", headless}, {"source.json", source}}
	for _, file := range files {
		if _, err := os.Lstat(filepath.Join(directory, file.name)); err == nil {
			return errors.New("constructed plan output already exists")
		}
	}
	var opened []*os.File
	defer func() {
		for i := len(opened) - 1; i >= 0; i-- {
			result = errors.Join(result, opened[i].Close())
		}
		result = errors.Join(result, ctx.Err())
	}()
	for _, file := range files {
		if err := ctx.Err(); err != nil {
			return err
		}
		handle, err := os.OpenFile(filepath.Join(directory, file.name), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
		if err != nil {
			return err
		}
		opened = append(opened, handle)
		if _, err := handle.Write(file.body); err != nil {
			return err
		}
		if err := handle.Sync(); err != nil {
			return err
		}
	}
	directoryFile, err := os.Open(directory)
	if err != nil {
		return err
	}
	defer func() { result = errors.Join(result, directoryFile.Close()) }()
	return directoryFile.Sync()
}
