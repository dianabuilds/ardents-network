package siteexperiment

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
)

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
	return copyBoundedEvidenceFiles(source, destination, referenceEvidenceFiles, "reference role evidence is missing or unbounded")
}

func copyBoundedEvidenceFiles(source, destination string, files map[string]string, missing string) error {
	for relativeSource, relativeDestination := range files {
		data, err := os.ReadFile(filepath.Join(source, filepath.FromSlash(relativeSource)))
		if err != nil || len(data) == 0 || len(data) > 1024*1024 {
			return errors.New(missing)
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

func retainReferencePartialRoleViews(retained string, sequence int, source string) error {
	destination := filepath.Join(retained, "attempts", formatAttempt(sequence), "reference")
	for role, relative := range map[string]string{"isolation": "isolation.json", "relay": filepath.Join("relay", "relay.json"), "gateway": filepath.Join("gateway", "gateway.json")} {
		data, err := os.ReadFile(filepath.Join(source, relative))
		path := filepath.Join(destination, relative)
		if err == nil {
			if len(data) == 0 || len(data) > 1024*1024 || !json.Valid(data) {
				return errors.New("partial reference role-view evidence is empty, unbounded, or invalid")
			}
			if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
				return err
			}
			if err := os.WriteFile(path, data, 0o600); err != nil {
				return err
			}
			continue
		}
		if !os.IsNotExist(err) {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			return err
		}
		if err := writeBoundedJSON(path, map[string]any{"schema_version": "gatec-role-view-unavailable/v1", "role": role, "status": "not_observed_before_terminal_failure"}); err != nil {
			return err
		}
	}
	return nil
}

func retainNativeRunEvidence(retained string, sequence int) error {
	source := filepath.Join(retained, "native-run.json")
	data, err := os.ReadFile(source)
	if err != nil || len(data) == 0 || len(data) > 1024*1024 || !json.Valid(data) {
		return errors.New("native failure evidence is missing, unbounded, or invalid")
	}
	destination := filepath.Join(retained, "attempts", formatAttempt(sequence), "native-run.json")
	if err := os.MkdirAll(filepath.Dir(destination), 0o700); err != nil {
		return err
	}
	return os.WriteFile(destination, data, 0o600)
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
