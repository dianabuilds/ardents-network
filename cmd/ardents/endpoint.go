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

	"github.com/dianabuilds/ardents-network/internal/endpoint/portable"
	"github.com/dianabuilds/ardents-network/internal/endpoint/replacement"
	"github.com/dianabuilds/ardents-network/internal/enrollment"
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
	if len(arguments) == 3 && arguments[1] == "enroll-installed" {
		return runEnrolledInstalled(ctx, arguments[2], output)
	}
	if len(arguments) == 3 && arguments[1] == "headless" {
		return runHeadlessRuntime(ctx, arguments[2], output)
	}
	if len(arguments) == 6 && arguments[1] == "open" {
		return runHeadlessOpen(ctx, arguments[2], arguments[3], arguments[4], arguments[5], output)
	}
	if len(arguments) == 3 && (arguments[1] == "publish" || arguments[1] == "withdraw") {
		return runHeadlessAdministration(ctx, arguments[1], arguments[2], output)
	}
	if len(arguments) == 2 && arguments[1] == "portable" {
		return runPortableEndpoint(ctx, output)
	}
	if len(arguments) == 3 && arguments[1] == "user-unit" {
		return runEndpointUserUnit(arguments[2], output)
	}
	if len(arguments) == 3 && arguments[1] == "installed-user-unit" {
		return runEndpointInstalledUserUnit(arguments[2], output)
	}
	if len(arguments) == 3 && arguments[1] == "replacement-self-test" {
		return runReplacementSelfTest(arguments[2], output)
	}
	if len(arguments) == 2 && arguments[1] == "replacement-recovery" {
		return runEndpointReplacementRecovery(output)
	}
	if len(arguments) == 3 && arguments[1] == "replace" {
		return runEndpointReplace(ctx, arguments[2], output)
	}
	if len(arguments) == 3 && arguments[1] == "rollback" {
		return runEndpointRollback(ctx, arguments[2], output)
	}
	return errors.New("usage: ardents endpoint <portable|enrollment-check <alpha-enrollment.json>|enroll <alpha-enrollment.json>|enroll-installed <package-enrollment.json>|headless <headless-runtime.json>|open <application-socket> <service-link> <input-file> <output-file>|publish <administration-socket>|withdraw <administration-socket>|user-unit <alpha-enrollment.json>|installed-user-unit <package-enrollment.json>|replacement-self-test <replacement-state-root>|replacement-recovery|replace <replacement-bundle>|rollback <replacement-bundle>>")
}

// runReplacementSelfTest is the candidate-side, no-network Endpoint
// replacement check. It proves that the binary actually executing matches a
// precommitted successor record; it does not activate a program, contact the
// network, or start an Endpoint runtime.
func runReplacementSelfTest(stateRoot string, output io.Writer) error {
	executable, err := os.Executable()
	if err != nil {
		return err
	}
	running, err := replacement.VerifyPreparedRunning(stateRoot, executable)
	if err != nil {
		return err
	}
	if running.State != replacement.StatePrepared {
		return errors.New("endpoint replacement self-test does not match a prepared successor")
	}
	encoder := json.NewEncoder(output)
	encoder.SetEscapeHTML(false)
	return encoder.Encode(struct {
		Kind, State, Release string
		ReleaseVersion       int64 `json:"release_version"`
	}{Kind: "endpoint-replacement-self-test", State: string(running.State), Release: running.Record.ReleaseID,
		ReleaseVersion: running.Record.ReleaseVersion})
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

type installedEnrollmentInput struct {
	Schema         string `json:"schema"`
	StaticRoot     string `json:"static_root"`
	ArtifactPath   string `json:"artifact_path"`
	Cohort         string `json:"cohort"`
	Release        string `json:"release"`
	Platform       string `json:"platform"`
	ManifestSHA256 string `json:"manifest_sha256"`
	Environment    string `json:"environment"`
	Network        string `json:"network"`
	TargetPath     string `json:"target_path"`
}

type enrollmentFact struct {
	Cohort, Release string
	Verified        enrollment.Verified
}

const (
	alphaEnrollmentInvalid     portable.Reason = "alpha-enrollment-invalid"
	releaseDecisionUnavailable portable.Reason = "release-decision-unavailable"
	releaseDecisionRejected    portable.Reason = "release-decision-rejected"
)

// runEnrollmentCheck is an already-running command's bounded local diagnosis
// for one closed-alpha bundle. It verifies the independently delivered pin
// before parsing the manifest and does not report Endpoint or network
// readiness. It cannot authenticate its own first execution.
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
	return runEnrolledEndpoint(ctx, output, false, func() (enrollmentFact, error) {
		input, verified, err := verifyEnrollment(path)
		return enrollmentFact{Cohort: input.Cohort, Release: input.Release, Verified: verified}, err
	})
}

func runEnrolledInstalled(ctx context.Context, path string, output io.Writer) error {
	return runEnrolledEndpoint(ctx, output, true, func() (enrollmentFact, error) {
		input, verified, err := verifyInstalledEnrollment(path)
		return enrollmentFact{Cohort: input.Cohort, Release: input.Release, Verified: verified}, err
	})
}

func runEnrolledEndpoint(ctx context.Context, output io.Writer, allowInstalledRebind bool, verify func() (enrollmentFact, error)) error {
	encoder := json.NewEncoder(output)
	encoder.SetEscapeHTML(false)
	if err := encodePortableEvent(encoder, portable.Event{State: portable.StateStarting}); err != nil {
		return err
	}
	config, err := portable.DefaultConfig()
	if err != nil {
		return encodeEnrolledFailure(encoder, err, portable.Event{State: portable.StateIncompatible, Reason: portable.ReasonLocalProfileInvalid})
	}
	running, err := portable.Open(config)
	if err != nil {
		return encodeEnrolledFailure(encoder, err, portable.FailureEvent(err))
	}
	defer running.Close()
	executable, err := os.Executable()
	if err != nil {
		return encodeEnrolledFailure(encoder, err, portable.Event{State: portable.StateBlocked, Reason: releaseDecisionUnavailable})
	}
	replacementRoot := filepath.Join(config.StateHome, "replacement")
	current, err := replacement.VerifyRunning(replacementRoot, executable)
	if err == nil && current.State == replacement.StateCurrent {
		if err := encoder.Encode(struct {
			Kind, State, Release string
			ReleaseVersion       int64 `json:"release_version"`
		}{Kind: "endpoint-replacement", State: string(current.State), Release: current.Record.ReleaseID,
			ReleaseVersion: current.Record.ReleaseVersion}); err != nil {
			return err
		}
		if err := encodePortableEvent(encoder, portable.Event{State: portable.StateReady, Attachment: running.Attachment()}); err != nil {
			return err
		}
		if err := running.Wait(ctx); err != nil {
			return encodeEnrolledFailure(encoder, err, portable.Event{State: portable.StateBlocked, Reason: portable.ReasonLockError})
		}
		return encodePortableEvent(encoder, portable.Event{State: portable.StateStopped})
	}
	if err != nil || (current.State != replacement.StateUnbound && !(allowInstalledRebind && current.State == replacement.StateMismatch)) {
		if err == nil {
			err = errors.New("endpoint replacement state does not match the executing program")
		}
		return encodeEnrolledFailure(encoder, err, portable.Event{State: portable.StateBlocked, Reason: releaseDecisionRejected})
	}
	fact, err := verify()
	if err != nil {
		return encodeEnrolledFailure(encoder, err, portable.Event{State: portable.StateIncompatible, Reason: alphaEnrollmentInvalid})
	}
	verified := fact.Verified
	verifier, err := release.Open(filepath.Join(config.StateHome, "floors", "release-decision"))
	if err != nil {
		return encodeEnrolledFailure(encoder, err, portable.Event{State: portable.StateBlocked, Reason: releaseDecisionUnavailable})
	}
	decision := verifier.Evaluate(ctx, verified.Inputs)
	closeErr := verifier.Close()
	if closeErr != nil {
		return encodeEnrolledFailure(encoder, closeErr, portable.Event{State: portable.StateBlocked, Reason: releaseDecisionUnavailable})
	}
	if decision.Outcome != release.OutcomeReleaseAccepted && decision.Outcome != release.OutcomeNoUpdate {
		return encodeEnrolledFailure(encoder, errors.New("alpha release decision rejected the enrolled bundle: "+string(decision.Outcome)), portable.Event{State: portable.StateBlocked, Reason: releaseDecisionRejected})
	}
	authorization, authorized := decision.Authorization()
	if !authorized {
		return encodeEnrolledFailure(encoder, errors.New("accepted alpha release decision lacks replacement authorization"), portable.Event{State: portable.StateBlocked, Reason: releaseDecisionUnavailable})
	}
	if _, err := replacement.Prepare(ctx, replacement.Request{StateRoot: replacementRoot, Artifact: verified.Inputs.Artifact, Authorization: authorization}); err != nil {
		return encodeEnrolledFailure(encoder, err, portable.Event{State: portable.StateBlocked, Reason: releaseDecisionUnavailable})
	}
	prepared, err := replacement.VerifyPreparedRunning(replacementRoot, executable)
	if err != nil || prepared.State != replacement.StatePrepared {
		if err == nil {
			err = errors.New("initial Endpoint program does not match its prepared replacement record")
		}
		return encodeEnrolledFailure(encoder, err, portable.Event{State: portable.StateBlocked, Reason: releaseDecisionRejected})
	}
	if _, err := replacement.CommitPrepared(replacementRoot, executable); err != nil {
		return encodeEnrolledFailure(encoder, err, portable.Event{State: portable.StateBlocked, Reason: releaseDecisionUnavailable})
	}
	if err := encoder.Encode(struct {
		Kind    string `json:"kind"`
		Outcome string `json:"outcome"`
		Cohort  string `json:"cohort"`
		Release string `json:"release"`
	}{Kind: "release-decision", Outcome: string(decision.Outcome), Cohort: fact.Cohort, Release: fact.Release}); err != nil {
		return err
	}
	if err := encodePortableEvent(encoder, portable.Event{State: portable.StateReady, Attachment: running.Attachment()}); err != nil {
		return err
	}
	if err := running.Wait(ctx); err != nil {
		return encodeEnrolledFailure(encoder, err, portable.Event{State: portable.StateBlocked, Reason: portable.ReasonLockError})
	}
	return encodePortableEvent(encoder, portable.Event{State: portable.StateStopped})
}

// encodeEnrolledFailure keeps a failed enrollment visible to the participant
// and service manager without suppressing the command failure itself. A
// failed enrollment never reaches ready or emits a normal stopped event.
func encodeEnrolledFailure(encoder *json.Encoder, cause error, event portable.Event) error {
	return errors.Join(cause, encodePortableEvent(encoder, event))
}

func encodePortableEvent(encoder *json.Encoder, event portable.Event) error {
	return encoder.Encode(portableEvent(event))
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

func verifyInstalledEnrollment(path string) (installedEnrollmentInput, enrollment.Verified, error) {
	var input installedEnrollmentInput
	if err := decodeOperatorInput(path, 16<<10, &input); err != nil {
		return installedEnrollmentInput{}, enrollment.Verified{}, err
	}
	if input.Schema != "ardents-ubuntu-package-enrollment-input-v1" {
		return installedEnrollmentInput{}, enrollment.Verified{}, errors.New("package enrollment input schema is not canonical")
	}
	if input.Platform != runtime.GOOS+"-"+runtime.GOARCH {
		return installedEnrollmentInput{}, enrollment.Verified{}, errors.New("package enrollment input does not match this platform")
	}
	executable, err := os.Executable()
	if err != nil {
		return installedEnrollmentInput{}, enrollment.Verified{}, err
	}
	if input.ArtifactPath == "" || input.StaticRoot == "" {
		return installedEnrollmentInput{}, enrollment.Verified{}, errors.New("package enrollment input is incomplete")
	}
	verified, err := enrollment.Verify(enrollment.Request{BundleRoot: input.StaticRoot, ExecutablePath: executable, ArtifactPath: input.ArtifactPath,
		Pin: enrollment.Pin{Cohort: input.Cohort, Release: input.Release, Platform: input.Platform,
			ManifestSHA256: input.ManifestSHA256}, Environment: input.Environment, Network: input.Network,
		TargetPath: input.TargetPath, Architecture: runtime.GOARCH, ReferenceTime: time.Now().UTC()})
	return input, verified, err
}

func hexDigest(value []byte) string {
	digest := sha256.Sum256(value)
	return hex.EncodeToString(digest[:])
}

// runPortableEndpoint adapts the selected Portable Endpoint local lifecycle to
// the command's bounded event projection. It intentionally creates no network
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
