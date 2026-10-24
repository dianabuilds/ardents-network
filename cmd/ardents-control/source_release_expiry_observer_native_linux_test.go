//go:build linux && network_signing_native

package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Genuine canonical test signatures and an actual wall-clock crossing exercise
// the outside Release observer. This fixture grants no installed authority.
func exerciseSourceReleaseExpiryObserver(t *testing.T, input sourceTimestampNativeInput) {
	t.Helper()
	helper, _ := sourceNativePinnedProgram(t, "ARDENTS_SOURCE_EXPIRY_OBSERVER_PROGRAM")
	plan := input.plan
	plan.Schema = "ardents-source-timestamp-release-plan-v1"
	plan.Expires = time.Now().UTC().Add(15 * time.Second).Truncate(time.Second)
	raw, err := json.Marshal(plan)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(input.planPath, raw, 0600); err != nil {
		t.Fatal(err)
	}
	metadata, history := filepath.Join(input.parent, "timestamp-wall"), filepath.Join(input.parent, "timestamp-wall-history")
	if _, err := signSourceRelease(t.Context(), []string{input.planPath, input.keys, metadata, history, input.distribution}, sourceReleaseTimestamp, nil); err != nil {
		t.Fatal("actual short Timestamp signing", err)
	}
	distribution := filepath.Join(input.parent, "timestamp-wall-distribution")
	if err := os.Mkdir(distribution, 0700); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"ardents-node-linux-amd64", "infrastructure-source.json", "ardents-source.service", "root.json", "snapshot.json", "targets.json", "timestamp.json"} {
		directory := input.distribution
		if name == "timestamp.json" {
			directory = metadata
		}
		raw, err := os.ReadFile(filepath.Join(directory, name))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(distribution, name), raw, 0600); err != nil {
			t.Fatal(err)
		}
	}
	inventory := filepath.Join(input.parent, "timestamp-wall-inventory")
	if _, err := prepareSourceArtifact(t.Context(), []string{"prepare-inventory", distribution, inventory}, nil); err != nil {
		t.Fatal(err)
	}
	raw, err = os.ReadFile(filepath.Join(inventory, "infrastructure-enrollment.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(distribution, "infrastructure-enrollment.json"), raw, 0600); err != nil {
		t.Fatal(err)
	}
	profile := struct {
		Schema              string            `json:"schema"`
		PublicReceiptSHA256 string            `json:"public_receipt_sha256"`
		Distribution        string            `json:"distribution"`
		History             string            `json:"history"`
		Network             string            `json:"network"`
		Phase               string            `json:"phase"`
		Files               map[string]string `json:"files"`
	}{"source-release-expiry-observer-v1", strings.Repeat("0", 64), distribution, history, plan.Network, "before", map[string]string{}}
	for _, name := range []string{"ardents-node-linux-amd64", "infrastructure-source.json", "ardents-source.service", "root.json", "timestamp.json", "snapshot.json", "targets.json", "infrastructure-enrollment.json"} {
		raw, err := os.ReadFile(filepath.Join(distribution, name))
		if err != nil {
			t.Fatal(err)
		}
		digest := sha256.Sum256(raw)
		profile.Files[name] = hex.EncodeToString(digest[:])
	}
	observe := func(phase string, wantSuccess bool) {
		t.Helper()
		profile.Phase = phase
		raw, err := json.Marshal(profile)
		if err != nil {
			t.Fatal(err)
		}
		raw = append(raw, '\n')
		path := filepath.Join(input.parent, "expiry-"+phase+"-profile.json")
		if err := os.WriteFile(path, raw, 0600); err != nil {
			t.Fatal(err)
		}
		pin := sha256.Sum256(raw)
		child := exec.CommandContext(t.Context(), helper, "-test.run=^TestSourceReleaseExpiryObserverNative$", "-test.v", "-test.timeout=45s")
		child.Env = append(os.Environ(), "ARDENTS_SOURCE_RELEASE_EXPIRY_OBSERVER_MODE=native-release-expiry", "ARDENTS_SOURCE_RELEASE_EXPIRY_PROFILE="+path, "ARDENTS_SOURCE_RELEASE_EXPIRY_PROFILE_SHA256="+hex.EncodeToString(pin[:]))
		output, err := child.CombinedOutput()
		t.Logf("%s", output)
		if !wantSuccess {
			if err == nil || bytes.Contains(output, []byte("SOURCE_RELEASE_EXPIRY_OBSERVATION ")) || !bytes.Contains(output, []byte("actual wall time does not match selected expiry phase")) {
				t.Fatal("wrong actual clock phase supplied a passing expiry observation", err)
			}
			return
		}
		if err != nil {
			t.Fatal("actual expiry observer", err)
		}
		var reports [][]byte
		for _, line := range bytes.Split(output, []byte("\n")) {
			if rest, ok := bytes.CutPrefix(line, []byte("SOURCE_RELEASE_EXPIRY_OBSERVATION ")); ok {
				reports = append(reports, rest)
			}
		}
		if len(reports) != 1 {
			t.Fatal("one actual expiry observation required")
		}
		var result struct {
			Phase      string            `json:"phase"`
			Reference  time.Time         `json:"reference_time"`
			Expires    time.Time         `json:"timestamp_expires"`
			Files      map[string]string `json:"files"`
			Outcomes   []string          `json:"outcomes"`
			Authorized []bool            `json:"authorized"`
			Floors     bool              `json:"floors_unchanged"`
			Closed     bool              `json:"verifier_closed"`
			Installed  bool              `json:"installed_execution"`
		}
		if json.Unmarshal(reports[0], &result) != nil || result.Phase != phase || !result.Expires.Equal(plan.Expires) || result.Installed || !result.Floors || !result.Closed || len(result.Outcomes) != 2 || len(result.Authorized) != 2 {
			t.Fatal("actual observer result incomplete", string(reports[0]))
		}
		want, authorized := "no-update", true
		if phase == "expired" {
			want, authorized = "release-expired", false
		}
		for i := range result.Outcomes {
			if result.Outcomes[i] != want || result.Authorized[i] != authorized {
				t.Fatal("actual target decision differs", string(reports[0]))
			}
		}
		if (phase == "before") != result.Reference.Before(result.Expires) {
			t.Fatal("supplied time replaced wall observation")
		}
		for name, pin := range profile.Files {
			current, err := os.ReadFile(filepath.Join(distribution, name))
			digest := sha256.Sum256(current)
			if err != nil || result.Files[name] != pin || hex.EncodeToString(digest[:]) != pin {
				t.Fatal("wall crossing changed an artifact", name, err)
			}
		}
	}
	observe("before", true)
	observe("expired", false)
	timer := time.NewTimer(time.Until(plan.Expires.Add(time.Millisecond)))
	defer timer.Stop()
	select {
	case <-timer.C:
	case <-t.Context().Done():
		t.Fatal(t.Context().Err())
	}
	observe("expired", true)
	observe("before", false)
}
