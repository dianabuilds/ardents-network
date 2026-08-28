//go:build h4_8_a11

package service_test

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/endpoint/enrollment"
)

const h48A11CandidateEnvironmentPrefix = "ARDENTS_H4_8_A11_CANDIDATE_"

type h48A11CandidateInput struct {
	bundleRoot, manifestPin, cohort, release, platform string
	environment, network, targetPath, architecture     string
	referenceRaw, endpointDigestRaw, controlDigestRaw  string
	referenceAt                                        time.Time
	endpointDigest, controlDigest                      [32]byte
	request                                            enrollment.Request
}

func assertH48A11ExactCandidateExpiry(t *testing.T) {
	t.Helper()
	report := h48A11QualifyCandidateV2(t)
	raw, err := json.Marshal(report)
	if err != nil || strings.ContainsAny(string(raw), "\r\n") {
		t.Fatalf("encode compact exact candidate expiry report: %v", err)
	}
	t.Logf("A11_EXPIRY_CANDIDATE_REPORT %s", raw)
}

func h48A11LoadCandidateInput() (h48A11CandidateInput, error) {
	names := []string{"BUNDLE_ROOT", "MANIFEST_PIN", "COHORT", "RELEASE", "PLATFORM", "ENVIRONMENT", "NETWORK",
		"TARGET_PATH", "ARCHITECTURE", "REFERENCE_AT", "ENDPOINT_SHA256", "CONTROL_SHA256"}
	values := make(map[string]string, len(names))
	for _, name := range names {
		value, found := os.LookupEnv(h48A11CandidateEnvironmentPrefix + name)
		if !found || value == "" || value != strings.TrimSpace(value) || strings.ContainsAny(value, "\x00\r\n") {
			return h48A11CandidateInput{}, fmt.Errorf("%s%s is absent or non-canonical", h48A11CandidateEnvironmentPrefix, name)
		}
		values[name] = value
	}
	if !filepath.IsAbs(values["BUNDLE_ROOT"]) {
		return h48A11CandidateInput{}, errors.New("candidate bundle root is not absolute")
	}
	if filepath.Clean(values["BUNDLE_ROOT"]) != values["BUNDLE_ROOT"] {
		return h48A11CandidateInput{}, errors.New("candidate bundle root is not canonical")
	}
	referenceAt, err := time.Parse(time.RFC3339, values["REFERENCE_AT"])
	if err != nil || referenceAt.Nanosecond() != 0 || values["REFERENCE_AT"] != referenceAt.UTC().Format(time.RFC3339) {
		return h48A11CandidateInput{}, errors.New("candidate reference_at is not one exact UTC RFC3339 second")
	}
	endpointDigest, err := h48A11ParseDigest("candidate Endpoint", values["ENDPOINT_SHA256"])
	if err != nil {
		return h48A11CandidateInput{}, err
	}
	controlDigest, err := h48A11ParseDigest("candidate control", values["CONTROL_SHA256"])
	if err != nil {
		return h48A11CandidateInput{}, err
	}
	input := h48A11CandidateInput{bundleRoot: filepath.Clean(values["BUNDLE_ROOT"]), manifestPin: values["MANIFEST_PIN"],
		cohort: values["COHORT"], release: values["RELEASE"], platform: values["PLATFORM"], environment: values["ENVIRONMENT"],
		network: values["NETWORK"], targetPath: values["TARGET_PATH"], architecture: values["ARCHITECTURE"],
		referenceRaw: values["REFERENCE_AT"], referenceAt: referenceAt, endpointDigestRaw: values["ENDPOINT_SHA256"],
		controlDigestRaw: values["CONTROL_SHA256"], endpointDigest: endpointDigest, controlDigest: controlDigest}
	input.request = enrollment.Request{BundleRoot: input.bundleRoot, ExecutablePath: filepath.Join(input.bundleRoot, "ardents-"+input.platform),
		Pin:         enrollment.Pin{Cohort: input.cohort, Release: input.release, Platform: input.platform, ManifestSHA256: input.manifestPin},
		Environment: input.environment, Network: input.network, TargetPath: input.targetPath, Architecture: input.architecture, ReferenceTime: input.referenceAt}
	return input, nil
}

func h48A11ParseDigest(owner, raw string) ([32]byte, error) {
	if len(raw) != 64 || raw != strings.ToLower(raw) {
		return [32]byte{}, fmt.Errorf("%s SHA-256 is not 64 lowercase hexadecimal characters", owner)
	}
	decoded, err := hex.DecodeString(raw)
	if err != nil {
		return [32]byte{}, fmt.Errorf("decode %s SHA-256: %w", owner, err)
	}
	var digest [32]byte
	copy(digest[:], decoded)
	return digest, nil
}
