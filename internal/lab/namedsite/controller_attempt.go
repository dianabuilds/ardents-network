package namedsite

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"

	"github.com/dianabuilds/ardents-network/internal/lab/nativecircuit"
	"github.com/dianabuilds/ardents-network/internal/lab/runlayout"
)

func runRouteAttempt(ctx context.Context, identity runlayout.Layout, fixture *authorityFixture, superseded *instanceCredential, sequence int, images experimentImages, retained string, progress func(string) error) (runErr error) {
	if progress == nil {
		progress = func(string) error { return nil }
	}
	if err := progress("reference-preparing"); err != nil {
		return matrixOperational(err)
	}
	_, repositoryRoot, runDirectory, _, err := identity.OwnedPaths(true, true)
	if err != nil {
		return matrixOperational(err)
	}
	nonce := make([]byte, 32)
	if _, err := rand.Read(nonce); err != nil {
		return matrixOperational(err)
	}
	reference, err := startReferenceApplication(ctx, repositoryRoot, runDirectory, images.reference, hex.EncodeToString(nonce), sequence, fixture, superseded, progress)
	if err != nil {
		var evidenceErr error
		if reference != nil {
			if isHardGateFailure(err) || errors.Is(err, errScenarioFailure) {
				evidenceErr = retainReferencePartialRoleViews(retained, sequence, reference.evidence)
			}
			closeErr := reference.close()
			if closeErr == nil {
				closeErr = writeReferenceOnlyAttemptCleanup(retained, sequence)
			}
			evidenceErr = errors.Join(evidenceErr, closeErr)
		}
		if isHardGateFailure(err) {
			if evidenceErr != nil {
				return errors.Join(err, matrixOperational(evidenceErr))
			}
			return err
		}
		if errors.Is(err, errScenarioFailure) {
			if evidenceErr != nil {
				return errors.Join(err, matrixOperational(evidenceErr))
			}
			return err
		}
		return matrixOperational(errors.Join(err, evidenceErr))
	}
	referenceClosed := false
	defer func() {
		if !referenceClosed {
			closeErr := reference.close()
			if closeErr == nil {
				closeErr = writeAttemptCleanup(retained, sequence)
			}
			if closeErr != nil {
				runErr = errors.Join(runErr, matrixOperational(closeErr))
			}
		}
	}()
	if err := progress("native-route-running"); err != nil {
		return matrixOperational(err)
	}
	root, chain, key := fixture.routeIdentity()
	if _, err := nativecircuit.RunAttached(ctx, identity, images.application, images.tooling, reference.routeSocket, reference.serviceSocket, root, chain, key); err != nil {
		scenario, outcomeErr := nativeAttachedScenarioFailed(retained)
		if outcomeErr != nil {
			return matrixOperational(errors.Join(err, outcomeErr))
		}
		if scenario {
			evidenceErr := errors.Join(
				retainNativeRunEvidence(retained, sequence),
				retainReferencePartialRoleViews(retained, sequence, reference.evidence),
			)
			if evidenceErr != nil {
				return errors.Join(scenarioFailure(err), matrixOperational(evidenceErr))
			}
			return scenarioFailure(err)
		}
		return matrixOperational(err)
	}
	if err := reference.wait(ctx); err != nil {
		if errors.Is(err, errScenarioFailure) {
			evidenceErr := errors.Join(
				retainNativeRunEvidence(retained, sequence),
				retainReferencePartialRoleViews(retained, sequence, reference.evidence),
			)
			if evidenceErr != nil {
				return errors.Join(err, matrixOperational(evidenceErr))
			}
		}
		return err
	}
	if err := progress("retaining-evidence"); err != nil {
		return matrixOperational(err)
	}
	if err := retainReferenceEvidence(retained, sequence, reference.evidence); err != nil {
		return matrixOperational(err)
	}
	if err := retainAttemptEvidence(retained, sequence); err != nil {
		return matrixOperational(err)
	}
	if err := clearCurrentNativeEvidence(retained); err != nil {
		return matrixOperational(err)
	}
	closeErr := reference.close()
	referenceClosed = true
	if closeErr != nil {
		return matrixOperational(closeErr)
	}
	if err := writeAttemptCleanup(retained, sequence); err != nil {
		return matrixOperational(err)
	}
	return nil
}

func clearCurrentNativeEvidence(retained string) error {
	for _, name := range []string{"native-run.json", "resource-samples.json"} {
		if err := os.Remove(filepath.Join(retained, name)); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	for _, name := range []string{"native-roles", "native-tools"} {
		directory := filepath.Join(retained, name)
		info, err := os.Lstat(directory)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 || filepath.Base(directory) != name {
			return errors.New("current native evidence is not an owned directory")
		}
		if err := os.RemoveAll(directory); err != nil {
			return err
		}
	}
	return nil
}

func supersededPublicationRejected(retained string, sequence int) bool {
	var publication struct {
		Attempted bool `json:"superseded_publication_attempted"`
		Rejected  bool `json:"superseded_publication_rejected"`
	}
	path := filepath.Join(retained, "attempts", formatAttempt(sequence), "reference", "administration", "publication.json")
	return readStrictEvidence(path, &publication) == nil && publication.Attempted && publication.Rejected
}

func nativeAttachedScenarioFailed(retained string) (bool, error) {
	var summary struct {
		SchemaVersion string          `json:"schema_version"`
		Status        string          `json:"status"`
		FailureKind   string          `json:"failure_kind"`
		Checks        map[string]bool `json:"checks"`
	}
	if err := readStrictEvidence(filepath.Join(retained, "native-run.json"), &summary); err != nil {
		return false, err
	}
	if summary.SchemaVersion != "carrier-lab-native-run/v1" || summary.Status != "failed" || !summary.Checks["cleanup_complete"] {
		return false, errors.New("native attached failure evidence is incomplete")
	}
	if summary.FailureKind != "scenario" && summary.FailureKind != "operational" {
		return false, errors.New("native attached failure kind is invalid")
	}
	return summary.FailureKind == "scenario", nil
}
