package launch

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/dianabuilds/ardents-network/internal/domain/enrollment"
	requestinput "github.com/dianabuilds/ardents-network/internal/endpoint/plan"
	endpointunit "github.com/dianabuilds/ardents-network/internal/endpoint/unit"
	"github.com/dianabuilds/ardents-network/internal/platform/artifacts"
	"github.com/dianabuilds/ardents-network/internal/platform/process"
	"github.com/dianabuilds/ardents-network/internal/platform/systemd"
	"github.com/dianabuilds/ardents-network/internal/release"
	generationauthorization "github.com/dianabuilds/ardents-network/internal/release/endpoint"
)

var errPlatformInput = errors.New("invalid Endpoint platform input")

func platformPathsOverlap(a, b string) bool {
	return a == b || strings.HasPrefix(a, b+"/") || strings.HasPrefix(b, a+"/")
}

// PlatformOperation selects which generation observation the caller needs.
type PlatformOperation uint8

const (
	PlatformRenderUnit PlatformOperation = iota + 1
	PlatformCheckStart
	PlatformInitializeTrust
)

// VerifyPlatform checks or renders the original enrolled platform launch.
// The caller retains the input observations until it reports the operation.
func VerifyPlatform(ctx context.Context, files *artifacts.PlatformInputs, operation PlatformOperation, manifest, suppliedPin string, raw []byte) ([]byte, error) {
	if ctx == nil || files == nil || !requestinput.PlatformBelow("/etc/ardents", manifest) ||
		(operation != PlatformRenderUnit && operation != PlatformCheckStart && operation != PlatformInitializeTrust) {
		return nil, errPlatformInput
	}
	var launch requestinput.PlatformLaunch
	if artifacts.DecodeObject(raw, &launch) != nil || !launch.Valid() {
		return nil, errPlatformInput
	}
	pin := digestHex(raw)
	if operation == PlatformCheckStart && suppliedPin != pin {
		return nil, errPlatformInput
	}
	plan, err := files.Read(launch.Configuration, 64<<10, false)
	if err != nil || digestHex(plan) != launch.ConfigSHA256 {
		return nil, errPlatformInput
	}
	decl, err := requestinput.DecodeDeclaration(plan)
	if err != nil {
		return nil, err
	}
	roots, err := requestinput.PlatformWritableRoots(decl)
	if err != nil || !slices.Equal(roots, launch.WritableRoots) {
		return nil, errPlatformInput
	}
	names, err := requestinput.PlatformSourceCredentials(decl)
	if err != nil || len(names) != len(launch.Inputs) {
		return nil, errPlatformInput
	}
	for index, name := range names {
		credential, err := files.ReadRuntime(name, 64<<10, index == 1)
		if err != nil || digestHex(credential) != launch.Inputs[name] {
			return nil, errPlatformInput
		}
	}
	if operation == PlatformInitializeTrust {
		return nil, initializeEndpointPlatformTrust(ctx, files, launch, decl, suppliedPin)
	}
	candidate, err := enrollment.ReadCandidate(ctx, launch.Distribution)
	if err != nil {
		return nil, err
	}
	descriptor, ok := candidate.File("protected-endpoint.json")
	if !ok || launch.Program != filepath.Join(decl.InstallationRoot, "generations", digestHex(descriptor), "ardents-linux-amd64") {
		return nil, errPlatformInput
	}
	template, ok := candidate.File("ardents-endpoint.service")
	if !ok {
		return nil, errPlatformInput
	}
	prepared, err := endpointunit.PrepareEndpointTemplate(template, decl.InstallationRoot, filepath.Dir(launch.Program))
	if err != nil {
		return nil, errPlatformInput
	}
	base, err := prepared.Render(roots)
	if err != nil {
		return nil, errPlatformInput
	}
	checkProgram := filepath.Join(launch.Distribution, "ardents-linux-amd64")
	unit, err := endpointunit.RenderPlatform(base, checkProgram, manifest, pin)
	if err != nil {
		return nil, errPlatformInput
	}
	if operation == PlatformRenderUnit {
		if err := files.Check(ctx); err != nil {
			return nil, err
		}
		return unit, nil
	}
	return nil, verifyEndpointPlatformPrestart(ctx, files, launch, decl, roots, unit, candidate, checkProgram, manifest, pin)
}

// initializeEndpointPlatformTrust admits the independently pinned initial bundle
// through Enrollment and Release. It does not stage an installed generation.
func initializeEndpointPlatformTrust(ctx context.Context, files *artifacts.PlatformInputs, launch requestinput.PlatformLaunch, decl requestinput.Declaration, pin string) error {
	if launch.Distribution != decl.BundleRoot || launch.ReleaseRoot != decl.ReleaseFloorRoot ||
		(decl.ManifestSHA256 != "" && decl.ManifestSHA256 != pin) ||
		platformPathsOverlap(launch.Distribution, launch.ReleaseRoot) {
		return errPlatformInput
	}
	if err := files.RetainDirectory(launch.Distribution); err != nil {
		return err
	}
	if err := files.RetainDirectory(filepath.Dir(launch.ReleaseRoot)); err != nil {
		return err
	}
	executable, err := os.Executable()
	if err != nil {
		return err
	}
	bundle, err := enrollment.Verify(ctx, enrollment.Request{
		BundleRoot: launch.Distribution, ExecutablePath: executable, ManifestSHA256: pin,
	})
	if err != nil {
		return err
	}
	facts, ok := bundle.Facts()
	if !ok || facts.Environment != launch.Environment || facts.Network != launch.Network {
		return errPlatformInput
	}
	program, ok := bundle.File(facts.Artifact)
	if !ok {
		return errPlatformInput
	}
	if err := artifacts.CheckRunningExecutable(filepath.Join(launch.Distribution, facts.Artifact), program); err != nil {
		return err
	}
	refTime, err := time.Parse(time.RFC3339Nano, decl.ReferenceTime)
	if err != nil {
		return err
	}
	input, ok := generationauthorization.InitialInputs(bundle, refTime)
	if !ok {
		return errPlatformInput
	}
	if err := files.Check(ctx); err != nil {
		return err
	}
	verifier, err := release.Open(launch.ReleaseRoot)
	if err != nil {
		return err
	}
	authorization, err := generationauthorization.AuthenticateInitial(ctx, verifier, bundle, input)
	if err == nil {
		programProof, generationProof := authorization.Targets()
		err = verifier.CheckAuthorizations(ctx, programProof, generationProof)
	}
	return errors.Join(err, verifier.Close())
}

// verifyEndpointPlatformPrestart is the privileged F-25 pre-start trust transfer. It
// verifies the fixed account, the installed main unit against the rendered enrolled
// template, the enrolled program against the actual generation executable, the loaded
// systemd confinement, and a fresh retained Release authorization. It performs no
// installation/start/stop/recovery effect and yields no accepting receipt to replay;
// success returns only after the verifier physically closes.
func verifyEndpointPlatformPrestart(ctx context.Context, files *artifacts.PlatformInputs, launch requestinput.PlatformLaunch, decl requestinput.Declaration, roots []string, unit []byte, candidate enrollment.Candidate, checkProgram, manifest, pin string) error {
	passwd, group, err := files.ReadAccountFiles()
	if err != nil {
		return err
	}
	if _, ok := process.DedicatedAccountID(passwd, group, "ardents-endpoint"); !ok {
		return errPlatformInput
	}
	installed, err := files.Read("/etc/systemd/system/ardents-endpoint.service", 64<<10, false)
	if err != nil || !bytes.Equal(installed, unit) {
		return errPlatformInput
	}
	facts, ok := candidate.Facts()
	if !ok {
		return errPlatformInput
	}
	program, _ := candidate.File(facts.Artifact)
	actual, err := files.Read(launch.Program, 64<<20, true)
	if err != nil || len(program) == 0 || !bytes.Equal(actual, program) {
		return errPlatformInput
	}
	version, err := systemd.Version(ctx)
	if err != nil {
		return err
	}
	unitProps, serviceProps, err := systemd.Endpoint(ctx)
	if err != nil {
		return err
	}
	generationDigest := path.Base(filepath.Dir(launch.Program))
	if err := endpointunit.VerifyPlatformConfiguration(version, unitProps, serviceProps, endpointunit.Configuration{InstallationRoot: decl.InstallationRoot, WritePaths: roots}, generationDigest, checkProgram, manifest, pin); err != nil {
		return errPlatformInput
	}
	// The platform manager cannot complete an interrupted guarded start.
	// Refuse before opening Release state, even if only part of the exchange
	// remains and the original guard is already absent.
	for _, name := range []string{"start-guard.json", "start-socket.json", "start-completion.socket"} {
		if _, err := os.Lstat(filepath.Join(decl.InstallationRoot, name)); !errors.Is(err, os.ErrNotExist) {
			return errors.Join(errPlatformInput, err)
		}
	}
	refTime, err := time.Parse(time.RFC3339Nano, decl.ReferenceTime)
	if err != nil {
		return errPlatformInput
	}
	input, ok := generationauthorization.CandidateInputs(candidate, refTime)
	if !ok {
		return errPlatformInput
	}
	verifier, err := release.OpenRetained(decl.ReleaseFloorRoot)
	if err != nil {
		return err
	}
	auth, err := generationauthorization.AuthenticateCandidate(ctx, verifier, candidate, input)
	if err == nil {
		programAuth, _ := auth.Targets()
		decision, accepted := programAuth.AcceptedDecision()
		// A fresh re-evaluation against the CURRENT committed floor admits either a
		// new release acceptance (first launch: candidate newer than the floor) or
		// no-update (restart: the candidate IS the already-accepted committed floor).
		// AuthenticateCandidate has already bound the accepted decision to the exact
		// candidate program/generation bytes (CoherentTargets + digest equality), so
		// no-update is the current accepted program, not a stored replay of a stale
		// proof: a candidate that does not match the current committed floor is
		// refused upstream. Both outcomes require an accepted decision; anything else
		// (no accepted decision, foreign floor, authority loss) refuses.
		if !accepted || (decision.Outcome != release.OutcomeReleaseAccepted && decision.Outcome != release.OutcomeNoUpdate) {
			err = release.ErrTrustUnavailable
		}
	}
	return errors.Join(err, verifier.Close())
}
