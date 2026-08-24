package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/dianabuilds/ardents-network/internal/endpoint"
	"github.com/dianabuilds/ardents-network/internal/endpoint/enrollment"
	"github.com/dianabuilds/ardents-network/internal/endpoint/portable"
	"github.com/dianabuilds/ardents-network/internal/release"
)

// runEndpoint adapts one bounded Endpoint process to the retained command
// result projection. The Endpoint owns process and connection lifecycle; this
// command only selects its explicit operator route.
func runEndpoint(ctx context.Context, arguments []string, output io.Writer) error {
	if len(arguments) == 3 && arguments[1] == "enrollment-check" {
		return runEnrollmentCheck(arguments[2], output)
	}
	if len(arguments) == 3 && arguments[1] == "enroll" {
		return runEnrolledPortable(ctx, arguments[2], output)
	}
	if len(arguments) == 2 && arguments[1] == "portable" {
		return runPortableEndpoint(ctx, output)
	}
	if len(arguments) != 3 || arguments[1] != "run" || arguments[2] == "" {
		return errors.New("usage: ardents endpoint <portable|enrollment-check <alpha-enrollment.json>|enroll <alpha-enrollment.json>|run <endpoint-plan.json>>")
	}
	encoder := json.NewEncoder(output)
	encoder.SetEscapeHTML(false)
	result, err := endpoint.Run(ctx, arguments[2], func(role string) {
		_ = encoder.Encode(map[string]string{"kind": "ready", "role": role})
	})
	if encodeErr := encoder.Encode(result); encodeErr != nil {
		return errors.Join(err, encodeErr)
	}
	publishedAt := time.Now()
	publishErr := encoder.Encode(struct {
		Kind       string `json:"kind"`
		AtUnixNano int64  `json:"at_unix_nano"`
	}{Kind: "connection-result-published", AtUnixNano: publishedAt.UnixNano()})
	return errors.Join(err, publishErr)
}

type alphaEnrollmentInput struct {
	Schema         string `json:"schema"`
	BundleRoot     string `json:"bundle_root"`
	Cohort         string `json:"cohort"`
	Release        string `json:"release"`
	Platform       string `json:"platform"`
	ManifestSHA256 string `json:"manifest_sha256"`
	Environment    string `json:"environment"`
	Network        string `json:"network"`
	TargetPath     string `json:"target_path"`
}

// runEnrollmentCheck is the explicit, non-executing participant preflight for
// one closed-alpha bundle. It verifies the independently delivered pin before
// parsing the manifest and does not report Endpoint or network readiness.
func runEnrollmentCheck(path string, output io.Writer) error {
	input, verified, err := verifyEnrollment(path)
	if err != nil {
		return err
	}
	encoder := json.NewEncoder(output)
	encoder.SetEscapeHTML(false)
	return encoder.Encode(struct {
		Kind, Cohort, Release, Platform string
		ArtifactSHA256                  string
	}{Kind: "alpha-enrollment-verified", Cohort: input.Cohort, Release: input.Release, Platform: input.Platform,
		ArtifactSHA256: hexDigest(verified.Inputs.Artifact)})
}

// runEnrolledPortable is the selected first-run composition: it claims the
// local owner profile before release-floor mutation, verifies the bundle pin,
// commits an accepted Release Decision, and only then exposes Portable
// readiness. It has no network route, browser capability, or update action.
func runEnrolledPortable(ctx context.Context, path string, output io.Writer) error {
	encoder := json.NewEncoder(output)
	encoder.SetEscapeHTML(false)
	_ = encoder.Encode(portableEvent(portable.Event{State: portable.StateStarting}))
	config, err := portable.DefaultConfig()
	if err != nil {
		return err
	}
	running, err := portable.Open(config)
	if err != nil {
		return err
	}
	defer running.Close()
	input, verified, err := verifyEnrollment(path)
	if err != nil {
		return err
	}
	verifier, err := release.Open(filepath.Join(config.StateHome, "floors", "release-decision"))
	if err != nil {
		return err
	}
	decision := verifier.Evaluate(ctx, verified.Inputs)
	closeErr := verifier.Close()
	if closeErr != nil {
		return closeErr
	}
	if decision.Outcome != release.OutcomeReleaseAccepted && decision.Outcome != release.OutcomeNoUpdate {
		return errors.New("alpha release decision rejected the enrolled bundle: " + string(decision.Outcome))
	}
	if err := encoder.Encode(struct {
		Kind, Outcome, Cohort, Release string
	}{Kind: "release-decision", Outcome: string(decision.Outcome), Cohort: input.Cohort, Release: input.Release}); err != nil {
		return err
	}
	_ = encoder.Encode(portableEvent(portable.Event{State: portable.StateReady, Attachment: running.Attachment()}))
	if err := running.Wait(ctx); err != nil {
		return err
	}
	return encoder.Encode(portableEvent(portable.Event{State: portable.StateStopped}))
}

func verifyEnrollment(path string) (alphaEnrollmentInput, enrollment.Verified, error) {
	var input alphaEnrollmentInput
	if err := decodeOperatorInput(path, 16<<10, &input); err != nil {
		return alphaEnrollmentInput{}, enrollment.Verified{}, err
	}
	if input.Schema != "ardents-alpha-enrollment-input-v1" {
		return alphaEnrollmentInput{}, enrollment.Verified{}, errors.New("alpha enrollment input schema is not canonical")
	}
	if input.Platform != runtime.GOOS+"-"+runtime.GOARCH {
		return alphaEnrollmentInput{}, enrollment.Verified{}, errors.New("alpha enrollment input does not match this platform")
	}
	executable, err := os.Executable()
	if err != nil {
		return alphaEnrollmentInput{}, enrollment.Verified{}, err
	}
	verified, err := enrollment.Verify(enrollment.Request{BundleRoot: input.BundleRoot, ExecutablePath: executable,
		Pin: enrollment.Pin{Cohort: input.Cohort, Release: input.Release, Platform: input.Platform,
			ManifestSHA256: input.ManifestSHA256}, Environment: input.Environment, Network: input.Network,
		TargetPath: input.TargetPath, Architecture: runtime.GOARCH, ReferenceTime: time.Now().UTC()})
	return input, verified, err
}

func hexDigest(value []byte) string {
	digest := sha256.Sum256(value)
	return hex.EncodeToString(digest[:])
}

// runPortableEndpoint adapts the selected H4-1A local lifecycle to the
// command's bounded event projection. It intentionally creates no network
// route, browser integration, or local application capability.
func runPortableEndpoint(ctx context.Context, output io.Writer) error {
	encoder := json.NewEncoder(output)
	encoder.SetEscapeHTML(false)
	config, err := portable.DefaultConfig()
	if err != nil {
		_ = encoder.Encode(portableEvent(portable.Event{State: portable.StateStarting}))
		_ = encoder.Encode(portableEvent(portable.Event{State: portable.StateIncompatible, Reason: portable.ReasonLocalProfileInvalid}))
		return err
	}
	return portable.Run(ctx, config, func(event portable.Event) {
		_ = encoder.Encode(portableEvent(event))
	})
}

func portableEvent(event portable.Event) struct {
	Kind       string `json:"kind"`
	State      string `json:"state"`
	Reason     string `json:"reason,omitempty"`
	Attachment string `json:"attachment,omitempty"`
} {
	return struct {
		Kind       string `json:"kind"`
		State      string `json:"state"`
		Reason     string `json:"reason,omitempty"`
		Attachment string `json:"attachment,omitempty"`
	}{Kind: "endpoint-lifecycle", State: string(event.State), Reason: string(event.Reason), Attachment: event.Attachment}
}
