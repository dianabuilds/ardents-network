//go:build linux

package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/dianabuilds/ardents-network/internal/enrollment"
	"github.com/dianabuilds/ardents-network/internal/network/source/servingdeclaration"
	"github.com/dianabuilds/ardents-network/internal/network/state"
	"github.com/dianabuilds/ardents-network/internal/release"
	"github.com/dianabuilds/ardents-network/internal/release/infrastructure"
)

// A launch declaration is operator configuration, not a domain identity or a
// serialized authorization. Its digest is part of the loaded pre-start argv.
type platformLaunch struct {
	Role          string            `json:"role"`
	Distribution  string            `json:"distribution"`
	ReleaseRoot   string            `json:"release_root"`
	Environment   string            `json:"environment"`
	Network       string            `json:"network"`
	Program       string            `json:"program"`
	Configuration string            `json:"configuration"`
	ConfigSHA256  string            `json:"config_sha256"`
	WritableRoots []string          `json:"writable_roots"`
	Inputs        map[string]string `json:"inputs"`
}

// unit emits bytes only. initialize-trust is a separate explicit first-pin
// action. check is the mandatory privileged ExecStartPre: retained Release only,
// no installation/start/stop/recovery effect and no accepting receipt to replay.
func runPlatform(ctx context.Context, args []string, out io.Writer) int {
	if ctx != nil && ctx.Err() == nil && len(args) == 1 && args[0] == "template" {
		_, err := io.WriteString(out, platformUnitTemplate)
		if err == nil {
			return 0
		}
		return 1
	}
	if ctx == nil || ctx.Err() != nil || os.Geteuid() != 0 || len(args) < 2 {
		return 2
	}
	if args[0] == "stage-endpoint" && len(args) == 2 {
		bounded, cancel := context.WithTimeout(ctx, 2*time.Minute)
		defer cancel()
		return runPlatformStageEndpoint(bounded, args[1], out)
	}
	if args[0] == "stage-endpoint" && len(args) == 3 {
		bounded, cancel := context.WithTimeout(ctx, 2*time.Minute)
		defer cancel()
		return runPlatformStageEndpointSuccessor(bounded, args[1], args[2], out)
	}
	initial := args[0] == "initialize-trust"
	if (args[0] != "unit" && args[0] != "check" && !initial) || (args[0] == "unit" && len(args) != 2) || (args[0] != "unit" && len(args) != 3) {
		return 2
	}
	bounded, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	files := &platformFiles{}
	err := executePlatformCheck(bounded, args, out, files)
	err = errors.Join(err, files.Close(), bounded.Err())
	if err != nil {
		return 1
	}
	return 0
}

func executePlatformCheck(ctx context.Context, args []string, out io.Writer, files *platformFiles) (result error) {
	path := args[1]
	if !platformBelow("/etc/ardents", path) {
		return errPlatformInput
	}
	raw, err := files.Read(path, 16<<10, false)
	if err != nil {
		return err
	}
	var launch platformLaunch
	if decodeAdmissionObject(raw, &launch) != nil || !validPlatformLaunch(launch) {
		return errPlatformInput
	}
	if launch.Role == "endpoint" {
		return executeEndpointPlatformCheck(ctx, args, out, files, launch, raw)
	}
	digest := sha256.Sum256(raw)
	pin := hex.EncodeToString(digest[:])
	if args[0] == "check" && args[2] != pin {
		return errPlatformInput
	}
	plan, err := files.Read(launch.Configuration, 64<<10, false)
	if err != nil || platformDigest(plan) != launch.ConfigSHA256 {
		return errPlatformInput
	}
	roots, err := platformPlanRoots(launch.Role, plan)
	if err != nil || !equalPlatformRoots(roots, launch.WritableRoots) {
		return errPlatformInput
	}
	names, err := platformPlanInputs(launch.Role, plan)
	if err != nil || len(names) != len(launch.Inputs) {
		return errPlatformInput
	}
	for index, name := range names {
		raw, err := files.ReadRuntime(name, 64<<10, index == 1)
		if err != nil || platformDigest(raw) != launch.Inputs[name] {
			return errPlatformInput
		}
	}
	unit := renderPlatformUnit(launch, path, pin)
	if args[0] == "unit" {
		if err := files.Check(ctx); err != nil {
			return err
		}
		_, err = out.Write(unit)
		return err
	}
	// Runtime users cannot rename a retained trust root or private distribution
	// through a writable ancestor. The verifier still owns the root and floors;
	// these are independent read-only ancestor observations, not its lease.
	if err := files.RetainDirectory(filepath.Dir(launch.ReleaseRoot)); err != nil {
		return err
	}
	if err := files.RetainDirectory(launch.Distribution); err != nil {
		return err
	}

	var inventory interface {
		File(string) ([]byte, bool)
		Facts() (enrollment.InfrastructureFacts, bool)
	}
	var b enrollment.InfrastructureBundle
	var c enrollment.InfrastructureCandidate
	if args[0] == "initialize-trust" {
		b, err = enrollment.VerifyInfrastructure(ctx, launch.Distribution, args[2])
		inventory = b
	} else {
		c, err = enrollment.ReadInfrastructureCandidate(ctx, launch.Distribution)
		inventory = c
	}
	if err != nil {
		return err
	}
	facts, ok := inventory.Facts()
	if !ok || facts.Invocation != launch.Role {
		return errPlatformInput
	}
	program, _ := inventory.File(infrastructure.ProgramName)
	template, _ := inventory.File("ardents-" + launch.Role + ".service")
	if !bytes.Equal(template, []byte(platformUnitTemplate)) {
		return errPlatformInput
	}
	actual, err := files.Read(launch.Program, 64<<20, true)
	if err != nil || !bytes.Equal(actual, program) {
		return errPlatformInput
	}
	// The privileged precheck itself must be the original private enrolled
	// executable. The public runtime copy is independently compared above.
	if err := platformOwnExecutable(filepath.Join(launch.Distribution, infrastructure.ProgramName), program); err != nil {
		return err
	}
	var sourceInspection state.SourceInspection
	var sourceUID uint32
	if launch.Role == "source" && args[0] == "check" {
		// The exact root-controlled plan is reused below and remains under the
		// existing file owner. This inspection creates no State or producer.
		defer func() { result = errors.Join(result, sourceInspection.Close()) }()
	}
	if args[0] == "check" {
		passwd, err := files.Read("/etc/passwd", 1<<20, false)
		if err != nil {
			return err
		}
		group, err := files.Read("/etc/group", 1<<20, false)
		if err != nil || !platformAccount(passwd, group, platformUser(launch)) {
			return errPlatformInput
		}
		installed, err := files.Read("/etc/systemd/system/ardents-"+launch.Role+".service", 64<<10, false)
		if err != nil || !bytes.Equal(installed, unit) {
			return errPlatformInput
		}
		if err := observePlatformPrestart(ctx, launch, path, pin); err != nil {
			return err
		}
		if launch.Role == "source" {
			for _, line := range strings.Split(string(passwd), "\n") {
				fields := strings.Split(line, ":")
				if len(fields) == 7 && fields[0] == platformUser(launch) {
					n, _ := strconv.ParseUint(fields[2], 10, 32)
					sourceUID = uint32(n)
				}
			}
			if sourceUID == 0 {
				return errPlatformInput
			}
			config, err := platformSourceInspectionConfig(plan)
			if err != nil {
				return err
			}
			sourceInspection, err = state.OpenSourceInspection(ctx, config)
			if err != nil {
				return err
			}
			if err := sourceInspection.Check(ctx, plan, sourceUID, sourceUID); err != nil {
				return err
			}
		}
	}
	root, _ := inventory.File("root.json")
	ts, _ := inventory.File("timestamp.json")
	snapshot, _ := inventory.File("snapshot.json")
	targets, _ := inventory.File("targets.json")
	metadata, err := release.StaticMetadataFiles(root, ts, snapshot, targets)
	if err != nil {
		return err
	}
	in := release.Inputs{RootBytes: root, Files: metadata, Artifact: program, TargetPath: infrastructure.ProgramTarget, Local: release.LocalEnvironment{Environment: launch.Environment, Network: launch.Network, Platform: "linux-amd64", Architecture: "amd64", RefTime: time.Now().UTC()}}
	var verifier *release.Verifier
	if args[0] == "initialize-trust" {
		verifier, err = release.Open(launch.ReleaseRoot)
	} else {
		verifier, err = release.OpenRetained(launch.ReleaseRoot)
	}
	if err != nil {
		return err
	}
	var authorization infrastructure.RuntimeAuthorization
	if args[0] == "initialize-trust" {
		authorization, err = infrastructure.AuthenticateRuntimeInitial(ctx, verifier, b, in)
	} else {
		authorization, err = infrastructure.AuthenticateRuntimeCandidate(ctx, verifier, c, in)
	}
	if err == nil {
		err = authorization.Check(ctx, verifier)
	}
	if err == nil {
		err = files.Check(ctx)
	}
	if err == nil && args[0] == "check" {
		err = observePlatformPrestart(ctx, launch, path, pin)
	}
	if err == nil && launch.Role == "source" && args[0] == "check" {
		err = sourceInspection.Check(ctx, plan, sourceUID, sourceUID)
	}
	// Returning success from ExecStartPre happens only after the genuine
	// verifier and every original file observer physically close successfully.
	return errors.Join(err, verifier.Close())
}

func platformSourceInspectionConfig(raw []byte) (state.Config, error) {
	plan, err := servingdeclaration.DecodeServingDeclaration(raw)
	if err != nil || !validSourceServePlan(plan) {
		return state.Config{}, errPlatformInput
	}
	config, err := networkStateConfig((*networkAuthorityPlan)(plan.Network))
	if err != nil {
		return state.Config{}, err
	}
	config.LocalRoleStateRoot = plan.ParticipationRoot
	return config, nil
}

// A named unit account must resolve to the fixed unprivileged local principal,
// never a root alias or a shared numeric identity. Provisioning is the operator's
// concern; this check creates no account or database record.
func platformAccount(passwd, group []byte, name string) bool {
	id := ""
	for _, line := range strings.Split(string(passwd), "\n") {
		fields := strings.Split(line, ":")
		if len(fields) == 7 && fields[0] == name {
			if id != "" || fields[1] != "x" || fields[2] != fields[3] || fields[5] != "/nonexistent" || fields[6] != "/usr/sbin/nologin" {
				return false
			}
			n, err := strconv.ParseUint(fields[2], 10, 32)
			if err != nil || n < 100 || n > 999 || strconv.FormatUint(n, 10) != fields[2] {
				return false
			}
			id = fields[2]
		}
	}
	if id == "" {
		return false
	}
	for _, line := range strings.Split(string(passwd), "\n") {
		fields := strings.Split(line, ":")
		if len(fields) == 7 && fields[0] != name && (fields[2] == id || fields[3] == id) {
			return false
		}
	}
	found := false
	for _, line := range strings.Split(string(group), "\n") {
		fields := strings.Split(line, ":")
		if len(fields) != 4 {
			continue
		}
		if fields[0] == name {
			if found || fields[1] != "x" || fields[2] != id || fields[3] != "" {
				return false
			}
			found = true
		} else if fields[2] == id {
			return false
		}
		for _, member := range strings.Split(fields[3], ",") {
			if member == name {
				return false
			}
		}
	}
	return found
}

var errPlatformInput = errors.New("platform launch refused")

func platformDigest(raw []byte) string {
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

// Keep unit arguments literal: no escaping, expansion, specifiers, aliases or
// service-writable ancestors may select another executable/configuration.
func platformPath(path string) bool {
	if !absoluteAdmissionPath(path) || path == "/" || len(path) > 4096 {
		return false
	}
	for _, c := range path {
		if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || strings.ContainsRune("/._-", c)) {
			return false
		}
	}
	return true
}

func platformBelow(parent, path string) bool {
	return platformPath(path) && strings.HasPrefix(path, parent+"/")
}

func validPlatformLaunch(p platformLaunch) bool {
	switch p.Role {
	case "source", "forwarding", "issuer", "descriptor", "introduction", "join":
		return validInfraPlatformLaunch(p)
	case "endpoint":
		return validEndpointPlatformLaunch(p)
	default:
		return false
	}
}

func validInfraPlatformLaunch(p platformLaunch) bool {
	if !platformBelow("/usr/lib/ardents", p.Program) || !platformBelow("/etc/ardents", p.Configuration) || !platformPath(p.Distribution) || !platformPath(p.ReleaseRoot) || !independentRouteRoots(p.Distribution, p.ReleaseRoot) || p.Environment == "" || p.Network == "" || len(p.ConfigSHA256) != 64 || strings.ToLower(p.ConfigSHA256) != p.ConfigSHA256 {
		return false
	}
	if _, err := hex.DecodeString(p.ConfigSHA256); err != nil || len(p.WritableRoots) < 2 || len(p.WritableRoots) > 8 {
		return false
	}
	wanted := 2
	if p.Role == "source" {
		wanted = 3
	}
	if len(p.Inputs) != wanted {
		return false
	}
	for path, digest := range p.Inputs {
		if !platformBelow("/etc/ardents", path) || len(digest) != 64 || strings.ToLower(digest) != digest {
			return false
		}
		if _, err := hex.DecodeString(digest); err != nil {
			return false
		}
	}
	for _, root := range p.WritableRoots {
		if !platformBelow("/var/lib/ardents/roles/"+p.Role, root) || !independentRouteRoots(root, p.Distribution, p.ReleaseRoot) {
			return false
		}
	}
	return independentRouteRoots(p.WritableRoots...)
}

func platformPlanInputs(role string, raw []byte) ([]string, error) {
	if role == "endpoint" {
		decl, err := platformEndpointDeclaration(raw)
		if err != nil {
			return nil, err
		}
		return platformEndpointInputs(decl)
	}
	var fields map[string]json.RawMessage
	if json.Unmarshal(raw, &fields) != nil {
		return nil, errPlatformInput
	}
	names := []string{"certificate", "private_key"}
	if role == "source" {
		names = append(names, "client_root")
	}
	paths := make([]string, 0, len(names))
	seen := map[string]bool{}
	for _, name := range names {
		var path string
		if json.Unmarshal(fields[name], &path) != nil || !platformBelow("/etc/ardents", path) || seen[path] {
			return nil, errPlatformInput
		}
		seen[path] = true
		paths = append(paths, path)
	}
	return paths, nil
}

func equalPlatformRoots(a, b []string) bool {
	return len(a) == len(b) && strings.Join(a, "\x00") == strings.Join(b, "\x00")
}

// The runtime remains the sole full plan decoder and domain admission owner.
// This projection limits platform write access to exactly the declared domain
// roots. No root is opened, initialized, inspected as clean or repaired here.
func platformPlanRoots(role string, raw []byte) ([]string, error) {
	if role == "endpoint" {
		decl, err := platformEndpointDeclaration(raw)
		if err != nil {
			return nil, err
		}
		return platformEndpointRoots(decl)
	}
	var fields map[string]json.RawMessage
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	if len(raw) == 0 || len(raw) > 64<<10 || uniqueJSON(decoder) != nil || json.Unmarshal(raw, &fields) != nil || fields == nil {
		return nil, errPlatformInput
	}
	var network networkAuthorityPlan
	if decodeAdmissionObject(fields["network"], &network) != nil {
		return nil, errPlatformInput
	}
	roots := []string{network.Root}
	names := []string{"participation_root", "spend_root", "hosting_root"}
	switch role {
	case "source":
		names = names[:1]
	case "introduction":
		names = append(names, "introduction_root")
	case "descriptor":
		names = append(names, "descriptor_root")
	case "issuer":
		var issuance json.RawMessage = fields["issuance"]
		prepared, err := decodeIssuanceResultPlan(issuance, "initialize-results")
		if err != nil {
			return nil, err
		}
		// Key bytes remain read-only; only their existing lock file needs
		// a writable open. Quota/debit and results retain their own histories.
		for _, name := range names {
			var root string
			if json.Unmarshal(fields[name], &root) != nil {
				return nil, errPlatformInput
			}
			roots = append(roots, root)
		}
		return append(roots, prepared.Admission.Root, prepared.Root, filepath.Join(prepared.Keys.Root, "issuer.lock")), nil
	case "forwarding", "join":
	default:
		return nil, errPlatformInput
	}
	for _, name := range names {
		var root string
		if json.Unmarshal(fields[name], &root) != nil {
			return nil, errPlatformInput
		}
		roots = append(roots, root)
	}
	return roots, nil
}
