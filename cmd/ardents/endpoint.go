package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"runtime"

	"github.com/dianabuilds/ardents-network/internal/enrollment"
)

var errEndpointOpenRetired = errors.New("endpoint open is retired")
var errEndpointPublishRetired = errors.New("endpoint publish is retired; use ardents-text publish <administration-socket> <document-file>")

// The per-user Portable Endpoint lifecycle is retired in favor of the
// platform-led protected system-unit delivery (ADR-0125, #617 S3.5). Each
// former route refuses before input or root effects; the canonical successor
// composition lives in ardents-next (platform template/unit/check, endpoint
// construct-plan/prepare-history/start-installed, installation
// upgrade-installed/recover-installed). The successor Endpoint lifecycle is
// forward-only: rollback has no successor translation.
var errEndpointEnrollRetired = errors.New("endpoint enroll is retired; use ardents-next platform delivery with endpoint construct-plan and prepare-history")
var errEndpointEnrollInstalledRetired = errors.New("endpoint enroll-installed is retired; use ardents-next platform delivery with endpoint construct-plan and prepare-history")
var errEndpointHeadlessRetired = errors.New("endpoint headless is retired; use ardents-next endpoint start-installed under the platform unit")
var errEndpointUserUnitRetired = errors.New("endpoint user-unit is retired; use ardents-next platform unit")
var errEndpointInstalledUserUnitRetired = errors.New("endpoint installed-user-unit is retired; use ardents-next platform unit")
var errEndpointReplaceRetired = errors.New("endpoint replace is retired; use ardents-next platform stage-endpoint")
var errEndpointRollbackRetired = errors.New("endpoint rollback is retired; the successor Endpoint lifecycle is forward-only")
var errEndpointReplacementSelfTestRetired = errors.New("endpoint replacement-self-test is retired; use ardents-next platform stage-endpoint")
var errEndpointReplacementRecoveryRetired = errors.New("endpoint replacement-recovery is retired; use platform delivery after verified clean shutdown")

// The persisted ardents-alpha-enrollment-input-v1 document form is retired
// (ADR-0127, #647): the bundle root and independently delivered manifest pin
// are the only enrollment-check inputs. The refusal precedes any file read.
var errEndpointEnrollmentCheckDocumentRetired = errors.New("endpoint enrollment-check <alpha-enrollment.json> is retired; use endpoint enrollment-check <bundle-root> <manifest-sha256>")

// runEndpoint adapts one bounded Endpoint process to the retained command
// result projection. The Endpoint owns process and connection lifecycle; this
// command only selects its explicit operator route.
func runEndpoint(ctx context.Context, arguments []string, output io.Writer) error {
	if len(arguments) >= 2 {
		switch arguments[1] {
		case "provision", "upgrade-installed", "recover-installed", "start-installed", "installation-check":
			return errors.New("legacy Endpoint installation is retired")
		case "enroll":
			return errEndpointEnrollRetired
		case "enroll-installed":
			return errEndpointEnrollInstalledRetired
		case "headless":
			return errEndpointHeadlessRetired
		case "user-unit":
			return errEndpointUserUnitRetired
		case "installed-user-unit":
			return errEndpointInstalledUserUnitRetired
		case "replace":
			return errEndpointReplaceRetired
		case "rollback":
			return errEndpointRollbackRetired
		case "replacement-self-test":
			return errEndpointReplacementSelfTestRetired
		case "replacement-recovery":
			return errEndpointReplacementRecoveryRetired
		}
	}
	if len(arguments) == 3 && arguments[1] == "enrollment-check" {
		return errEndpointEnrollmentCheckDocumentRetired
	}
	if len(arguments) == 4 && arguments[1] == "enrollment-check" {
		return runEnrollmentCheck(ctx, arguments[2], arguments[3], output)
	}
	if len(arguments) == 6 && arguments[1] == "open" {
		return errEndpointOpenRetired
	}
	if len(arguments) >= 2 && arguments[1] == "publish" {
		return errEndpointPublishRetired
	}
	if len(arguments) == 3 && arguments[1] == "withdraw" {
		return runHeadlessAdministration(ctx, arguments[1], arguments[2], output)
	}
	return errors.New("usage: ardents endpoint <enrollment-check <bundle-root> <manifest-sha256>|enroll (retired; use ardents-next platform delivery)|enroll-installed (retired; use ardents-next platform delivery)|headless (retired; use ardents-next endpoint start-installed)|open <application-socket> <target-link> <input-file> <output-file> (retired; refuses before effects)|publish <administration-socket> (retired; use ardents-text publish <administration-socket> <document-file>)|withdraw <administration-socket>|user-unit (retired; use ardents-next platform unit)|installed-user-unit (retired; use ardents-next platform unit)|replacement-self-test (retired)|replacement-recovery (retired)|replace (retired; use ardents-next platform stage-endpoint)|rollback (retired; forward-only)>")
}

// runEnrollmentCheck is an already-running command's bounded local diagnosis
// for one closed-alpha bundle. The successor enrollment owner verifies the
// independently delivered pin before parsing the manifest; this command only
// renders the authenticated initial facts. It does not report Endpoint or
// network readiness and cannot authenticate its own first execution.
func runEnrollmentCheck(ctx context.Context, bundleRoot, manifestSHA256 string, output io.Writer) error {
	executable, err := os.Executable()
	if err != nil {
		return err
	}
	bundle, err := enrollment.Verify(ctx, enrollment.Request{BundleRoot: bundleRoot, ExecutablePath: executable, ManifestSHA256: manifestSHA256, Scope: enrollment.General})
	if err != nil {
		return err
	}
	facts, ok := bundle.Facts()
	if !ok {
		return errors.New("manifest-pinned enrollment returned no verified facts")
	}
	if facts.Platform != runtime.GOOS+"-"+runtime.GOARCH {
		return errors.New("manifest-pinned enrollment does not match this platform")
	}
	artifact, ok := bundle.File(facts.Artifact)
	if !ok {
		return errors.New("manifest-pinned enrollment lacks its verified artifact")
	}
	defer clear(artifact)
	encoder := json.NewEncoder(output)
	encoder.SetEscapeHTML(false)
	return encoder.Encode(struct {
		Kind, Cohort, Release, Platform string
		ArtifactSHA256                  string
	}{Kind: "manifest-pinned-enrollment-verified", Cohort: facts.Cohort, Release: facts.Release, Platform: facts.Platform,
		ArtifactSHA256: hexDigest(artifact)})
}

func hexDigest(value []byte) string {
	digest := sha256.Sum256(value)
	return hex.EncodeToString(digest[:])
}
