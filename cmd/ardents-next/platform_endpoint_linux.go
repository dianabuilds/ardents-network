//go:build linux

package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/enrollment"
	generationauthorization "github.com/dianabuilds/ardents-network/internal/successor/installation"
	requestinput "github.com/dianabuilds/ardents-network/internal/successor/installation/request"
	"github.com/dianabuilds/ardents-network/internal/successor/installation/systemd"
	endpointunit "github.com/dianabuilds/ardents-network/internal/successor/installation/unit"
	"github.com/dianabuilds/ardents-network/internal/successor/release"
)

// validEndpointPlatformLaunch admits the protected text Endpoint declaration shape.
// The Program is the enrolled generation executable (installationRoot/generations/
// <digest>/ardents-linux-amd64); its exact generation binding is verified against the
// declaration during rendering, not assumed here. Writable roots carry no fixed infra
// parent: their exact projection and immutable-overlap refusal are owned by
// platformEndpointRoots and cross-checked against the declaration before any effect.
func validEndpointPlatformLaunch(p platformLaunch) bool {
	if !platformBelow("/etc/ardents", p.Configuration) || !platformPath(p.Distribution) || !platformPath(p.ReleaseRoot) || !platformPath(p.Program) || !independentRouteRoots(p.Distribution, p.ReleaseRoot) || p.Environment == "" || p.Network == "" || len(p.ConfigSHA256) != 64 || strings.ToLower(p.ConfigSHA256) != p.ConfigSHA256 {
		return false
	}
	if _, err := hex.DecodeString(p.ConfigSHA256); err != nil || len(p.WritableRoots) < 2 || len(p.WritableRoots) > 16 {
		return false
	}
	if path.Base(p.Program) != "ardents-linux-amd64" || !strings.Contains(p.Program, "/generations/") {
		return false
	}
	if len(p.Inputs) != 2 {
		return false
	}
	for credential, digest := range p.Inputs {
		if !platformBelow("/etc/ardents", credential) || len(digest) != 64 || strings.ToLower(digest) != digest {
			return false
		}
		if _, err := hex.DecodeString(digest); err != nil {
			return false
		}
	}
	for _, root := range p.WritableRoots {
		if !platformPath(root) || !independentRouteRoots(root, p.Distribution, p.ReleaseRoot, p.Program) {
			return false
		}
	}
	return independentRouteRoots(p.WritableRoots...)
}

// The protected text Endpoint keeps its own canonical installation declaration
// (ardents-endpoint-installation-request-v1) rather than the infra role plan. These
// projections limit platform write access to exactly the declared domain roots and
// bind the Source credential pair; they open no root, initialize no history and
// grant no Release, manager or startup authority. The runtime remains the sole full
// plan decoder and domain admission owner.

func platformEndpointDeclaration(raw []byte) (requestinput.Declaration, error) {
	decl, err := requestinput.DecodeDeclaration(raw)
	if err != nil {
		return requestinput.Declaration{}, errPlatformInput
	}
	return decl, nil
}

// platformEndpointRoots mirrors the fixed Endpoint write allowance: the mutable
// semantic roots plus the socket/permission parent directories. A writable parent
// must not enclose immutable artifacts or retained trust. It selects no root.
func platformEndpointRoots(decl requestinput.Declaration) ([]string, error) {
	paths := requestinput.MutableRoots(decl.Headless)
	for _, file := range []string{
		decl.Headless.ApplicationSocket,
		decl.Headless.ReaderPermission.RequestPath,
		decl.Headless.ReaderPermission.ResponsePath,
	} {
		paths = append(paths, path.Dir(file))
	}
	if decl.Headless.Role == "" {
		for _, file := range []string{
			decl.Headless.AdministrationSocket,
			decl.Headless.PublisherPermission.RequestPath,
			decl.Headless.PublisherPermission.ResponsePath,
		} {
			paths = append(paths, path.Dir(file))
		}
	}
	for _, writable := range paths {
		if !platformPath(writable) || writable == "/" {
			return nil, errPlatformInput
		}
		for _, immutable := range []string{decl.BundleRoot, decl.InstallationRoot, decl.ReleaseFloorRoot} {
			if pathsOverlapPlatform(writable, immutable) {
				return nil, errPlatformInput
			}
		}
	}
	sort.Strings(paths)
	distinct := make([]string, 0, len(paths))
	for _, writable := range paths {
		if len(distinct) == 0 || writable != distinct[len(distinct)-1] {
			distinct = append(distinct, writable)
		}
	}
	return distinct, nil
}

// platformEndpointInputs binds the Source credential pair exactly; both remain
// read-only enrolled inputs below /etc/ardents, never a writable semantic root.
func platformEndpointInputs(decl requestinput.Declaration) ([]string, error) {
	paths := []string{decl.Source.ClientCertificate, decl.Source.ClientKey}
	seen := map[string]bool{}
	for _, credential := range paths {
		if !platformBelow("/etc/ardents", credential) || seen[credential] {
			return nil, errPlatformInput
		}
		seen[credential] = true
	}
	return paths, nil
}

func pathsOverlapPlatform(a, b string) bool {
	return a == b || platformBelow(a, b) || platformBelow(b, a)
}

// executeEndpointPlatformCheck owns the protected text Endpoint platform launch. It
// reuses the canonical installation declaration and the enrolled fixed Endpoint
// template; the runtime remains the sole full plan decoder and domain admission
// owner. `unit` renders the enrolled main unit bound to the admitted write roots and
// generation command path. The privileged pre-start trust transfer (F-25) and worker
// confinement verification are a separate bounded step and refuse until then.
func executeEndpointPlatformCheck(ctx context.Context, args []string, out io.Writer, files *platformFiles, launch platformLaunch, raw []byte) error {
	digest := sha256.Sum256(raw)
	pin := hex.EncodeToString(digest[:])
	if args[0] == "check" && args[2] != pin {
		return errPlatformInput
	}
	plan, err := files.Read(launch.Configuration, 64<<10, false)
	if err != nil || platformDigest(plan) != launch.ConfigSHA256 {
		return errPlatformInput
	}
	decl, err := platformEndpointDeclaration(plan)
	if err != nil {
		return err
	}
	roots, err := platformEndpointRoots(decl)
	if err != nil || !equalPlatformRoots(roots, launch.WritableRoots) {
		return errPlatformInput
	}
	names, err := platformEndpointInputs(decl)
	if err != nil || len(names) != len(launch.Inputs) {
		return errPlatformInput
	}
	for index, name := range names {
		credential, err := files.ReadRuntime(name, 64<<10, index == 1)
		if err != nil || platformDigest(credential) != launch.Inputs[name] {
			return errPlatformInput
		}
	}
	if args[0] == "initialize-trust" {
		return initializeEndpointPlatformTrust(ctx, files, launch, decl, args[2])
	}
	candidate, err := enrollment.ReadCandidate(ctx, launch.Distribution, enrollment.Headless)
	if err != nil {
		return err
	}
	template, ok := candidate.File("ardents-endpoint.service")
	if !ok {
		return errPlatformInput
	}
	prepared, err := endpointunit.PrepareEndpointTemplate(template, decl.InstallationRoot, filepath.Dir(launch.Program))
	if err != nil {
		return errPlatformInput
	}
	base, err := prepared.Render(roots)
	if err != nil {
		return errPlatformInput
	}
	checkProgram := filepath.Join(launch.Distribution, "ardents-linux-amd64")
	manifest := args[1]
	unit, err := renderEndpointPlatformUnit(base, checkProgram, manifest, pin)
	if err != nil {
		return errPlatformInput
	}
	if args[0] == "unit" {
		if err := files.Check(ctx); err != nil {
			return err
		}
		_, err = out.Write(unit)
		return err
	}
	return verifyEndpointPlatformPrestart(ctx, files, launch, decl, roots, unit, candidate, checkProgram, manifest, pin)
}

// renderEndpointPlatformUnit derives the platform-managed Endpoint unit from the
// enrolled fixed template by injecting the sole privileged F-25 pre-start check. The
// transitional native rendering (renderEndpointUnit) stays unchanged and keeps no
// ExecStartPre; the enrolled inventory and protected descriptor are not widened. The
// check program is the original enrolled executable, bound to this exact launch
// declaration digest; literal safe paths only, no expansion or extra hook.
func renderEndpointPlatformUnit(base []byte, checkProgram, manifest, pin string) ([]byte, error) {
	if !platformPath(checkProgram) || !platformPath(manifest) || len(pin) != 64 || strings.ToLower(pin) != pin {
		return nil, errPlatformInput
	}
	if _, err := hex.DecodeString(pin); err != nil || bytes.Contains(base, []byte("ExecStartPre=")) {
		return nil, errPlatformInput
	}
	pre := "ExecStartPre=+:" + checkProgram + " platform check " + manifest + " " + pin + "\n"
	unit := bytes.Replace(base, []byte("ExecStart="), []byte(pre+"ExecStart="), 1)
	if bytes.Equal(unit, base) {
		return nil, errPlatformInput
	}
	return unit, nil
}

// initializeEndpointPlatformTrust admits the independently pinned initial bundle
// through Enrollment and Release. It does not stage an installed generation.
func initializeEndpointPlatformTrust(ctx context.Context, files *platformFiles, launch platformLaunch, decl requestinput.Declaration, pin string) error {
	if launch.Distribution != decl.BundleRoot || launch.ReleaseRoot != decl.ReleaseFloorRoot ||
		(decl.ManifestSHA256 != "" && decl.ManifestSHA256 != pin) ||
		pathsOverlapPlatform(launch.Distribution, launch.ReleaseRoot) {
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
		BundleRoot: launch.Distribution, ExecutablePath: executable, ManifestSHA256: pin, Scope: enrollment.Headless,
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
	if err := platformOwnExecutable(filepath.Join(launch.Distribution, facts.Artifact), program); err != nil {
		return err
	}
	refTime, err := time.Parse(time.RFC3339Nano, decl.ReferenceTime)
	if err != nil {
		return err
	}
	input, ok := initialReleaseInputs(bundle, refTime)
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
func verifyEndpointPlatformPrestart(ctx context.Context, files *platformFiles, launch platformLaunch, decl requestinput.Declaration, roots []string, unit []byte, candidate enrollment.Candidate, checkProgram, manifest, pin string) error {
	passwd, err := files.Read("/etc/passwd", 1<<20, false)
	if err != nil {
		return err
	}
	group, err := files.Read("/etc/group", 1<<20, false)
	if err != nil || !platformAccount(passwd, group, platformUser(launch)) {
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
	input, ok := candidateReleaseInputs(candidate, refTime)
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
