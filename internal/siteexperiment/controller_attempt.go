package siteexperiment

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"

	"github.com/dianabuilds/ardents-network/internal/experimentrun"
	"github.com/dianabuilds/ardents-network/internal/nativecircuit"
)

func runRouteAttempt(ctx context.Context, identity experimentrun.Layout, fixture *authorityFixture, superseded *instanceCredential, sequence int, images experimentImages, retained string) (runErr error) {
	_, repositoryRoot, runDirectory, _, err := identity.OwnedPaths(true, true)
	if err != nil {
		return err
	}
	nonce := make([]byte, 32)
	if _, err := rand.Read(nonce); err != nil {
		return err
	}
	reference, err := startReferenceApplication(ctx, repositoryRoot, runDirectory, images.reference, hex.EncodeToString(nonce), sequence, fixture, superseded)
	if err != nil {
		if reference != nil {
			err = errors.Join(err, reference.close())
		}
		return err
	}
	referenceClosed := false
	defer func() {
		if !referenceClosed {
			closeErr := reference.close()
			if closeErr == nil {
				closeErr = writeAttemptCleanup(retained, sequence)
			}
			runErr = errors.Join(runErr, closeErr)
		}
	}()
	root, chain, key := fixture.routeIdentity()
	if _, err := nativecircuit.RunAttached(ctx, identity, images.application, images.tooling, reference.routeSocket, reference.serviceSocket, root, chain, key); err != nil {
		return err
	}
	if err := reference.wait(ctx); err != nil {
		return err
	}
	if err := retainReferenceEvidence(retained, sequence, reference.evidence); err != nil {
		return err
	}
	if err := retainAttemptEvidence(retained, sequence); err != nil {
		return err
	}
	if err := clearCurrentNativeEvidence(retained); err != nil {
		return err
	}
	closeErr := reference.close()
	referenceClosed = true
	if closeErr != nil {
		return closeErr
	}
	return writeAttemptCleanup(retained, sequence)
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

func writeAttemptCleanup(retained string, sequence int) error {
	return writeBoundedJSON(filepath.Join(retained, "attempts", formatAttempt(sequence), "cleanup.json"), map[string]any{
		"schema_version": "gatec-attempt-cleanup/v1", "reference_resources_removed": true,
	})
}

func attemptCleanupProven(retained string, sequence int) bool {
	directory := filepath.Join(retained, "attempts", formatAttempt(sequence))
	var cleanup struct {
		Removed bool `json:"reference_resources_removed"`
	}
	var native struct {
		Checks map[string]bool `json:"checks"`
	}
	return readStrictEvidence(filepath.Join(directory, "cleanup.json"), &cleanup) == nil && cleanup.Removed &&
		readStrictEvidence(filepath.Join(directory, "native-run.json"), &native) == nil && native.Checks["cleanup_complete"]
}

func supersededPublicationRejected(retained string, sequence int) bool {
	var publication struct {
		Attempted bool `json:"superseded_publication_attempted"`
		Rejected  bool `json:"superseded_publication_rejected"`
	}
	path := filepath.Join(retained, "attempts", formatAttempt(sequence), "reference", "administration", "publication.json")
	return readStrictEvidence(path, &publication) == nil && publication.Attempted && publication.Rejected
}

var referenceEvidenceFiles = map[string]string{
	"authority/authority.json":               "authority/authority.json",
	"administration/publication.json":        "administration/publication.json",
	"gateway/gateway.json":                   "gateway/gateway.json",
	"relay/relay.json":                       "relay/relay.json",
	"client-endpoint/client-endpoint.json":   "client-endpoint/client-endpoint.json",
	"http-client/http-client.json":           "http-client/http-client.json",
	"http-application/http-application.json": "http-application/http-application.json",
	"isolation.json":                         "isolation.json",
}

func retainReferenceEvidence(retained string, sequence int, source string) error {
	destination := filepath.Join(retained, "attempts", formatAttempt(sequence), "reference")
	for relativeSource, relativeDestination := range referenceEvidenceFiles {
		data, err := os.ReadFile(filepath.Join(source, filepath.FromSlash(relativeSource)))
		if err != nil || len(data) == 0 || len(data) > 1024*1024 {
			return errors.New("reference role evidence is missing or unbounded")
		}
		path := filepath.Join(destination, filepath.FromSlash(relativeDestination))
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			return err
		}
		if err := os.WriteFile(path, data, 0o600); err != nil {
			return err
		}
	}
	return nil
}

func retainAttemptEvidence(retained string, sequence int) error {
	directory := filepath.Join(retained, "attempts", formatAttempt(sequence))
	if err := os.MkdirAll(directory, 0o700); err != nil {
		return err
	}
	for _, name := range []string{"native-run.json", "resource-samples.json"} {
		data, err := os.ReadFile(filepath.Join(retained, name))
		if err != nil || len(data) == 0 || len(data) > 4*1024*1024 {
			return errors.New("native attached evidence is missing or unbounded")
		}
		if err := os.WriteFile(filepath.Join(directory, name), data, 0o600); err != nil {
			return err
		}
	}
	roleDirectory := filepath.Join(directory, "native-roles")
	if err := os.Mkdir(roleDirectory, 0o700); err != nil {
		return err
	}
	for _, role := range []string{"user", "service"} {
		data, err := os.ReadFile(filepath.Join(retained, "native-roles", role+".json"))
		if err != nil || len(data) == 0 || len(data) > 1024*1024 {
			return errors.New("attached endpoint evidence is missing or unbounded")
		}
		if err := os.WriteFile(filepath.Join(roleDirectory, role+".json"), data, 0o600); err != nil {
			return err
		}
	}
	return retainNativeToolEvidence(retained, directory)
}

func retainNativeToolEvidence(retained, attemptDirectory string) error {
	source := filepath.Join(retained, "native-tools")
	entries, err := os.ReadDir(source)
	if err != nil || len(entries) == 0 || len(entries) > 32 {
		return errors.New("native tool evidence set is missing or unbounded")
	}
	destination := filepath.Join(attemptDirectory, "native-tools")
	if err := os.Mkdir(destination, 0o700); err != nil {
		return err
	}
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" || filepath.Base(entry.Name()) != entry.Name() {
			return errors.New("native tool evidence contains an unexpected entry")
		}
		data, err := os.ReadFile(filepath.Join(source, entry.Name()))
		if err != nil || len(data) == 0 || len(data) > 1024*1024 {
			return errors.New("native tool evidence is missing or unbounded")
		}
		if err := os.WriteFile(filepath.Join(destination, entry.Name()), data, 0o600); err != nil {
			return err
		}
	}
	return nil
}

func formatAttempt(sequence int) string {
	const digits = "000"
	value := []byte(digits)
	for index := len(value) - 1; index >= 0; index-- {
		value[index] = byte('0' + sequence%10)
		sequence /= 10
	}
	return string(value)
}
