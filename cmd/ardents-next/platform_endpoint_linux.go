//go:build linux

package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/dianabuilds/ardents-network/internal/successor/enrollment"
	requestinput "github.com/dianabuilds/ardents-network/internal/successor/installation/request"
	endpointunit "github.com/dianabuilds/ardents-network/internal/successor/installation/unit"
)

// errEndpointPlatformCheckPending refuses the privileged Endpoint pre-start check
// until the F-25 trust transfer and worker confinement verification are owned by the
// next bounded step. It is a fail-closed refusal, never an accepting receipt.
var errEndpointPlatformCheckPending = errors.New("endpoint platform check is not yet available")

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
	candidate, err := enrollment.ReadCandidate(ctx, launch.Distribution, enrollment.Headless)
	if err != nil {
		return err
	}
	template, ok := candidate.File("endpoint-unit.service")
	if !ok {
		return errPlatformInput
	}
	prepared, err := endpointunit.PrepareEndpointTemplate(template, decl.InstallationRoot, filepath.Dir(launch.Program))
	if err != nil {
		return errPlatformInput
	}
	unit, err := prepared.Render(roots)
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
	return errEndpointPlatformCheckPending
}
