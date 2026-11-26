package launch

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"path/filepath"
	"time"

	"github.com/dianabuilds/ardents-network/internal/domain/enrollment"
	"github.com/dianabuilds/ardents-network/internal/domain/release"
	"github.com/dianabuilds/ardents-network/internal/domain/release/infrastructure"
	"github.com/dianabuilds/ardents-network/internal/network/source/serving"
	"github.com/dianabuilds/ardents-network/internal/network/source/servingdeclaration"
	"github.com/dianabuilds/ardents-network/internal/network/state"
	"github.com/dianabuilds/ardents-network/internal/platform/artifacts"
	"github.com/dianabuilds/ardents-network/internal/platform/process"
	"github.com/dianabuilds/ardents-network/internal/platform/systemd"
)

type Operation uint8

const (
	RenderUnit Operation = iota + 1
	InitializeTrust
	CheckStart
)

// RetainedDeclaration is the exact declaration borrowed from PlatformInputs.
type RetainedDeclaration struct {
	files       *artifacts.PlatformInputs
	path        string
	raw         []byte
	declaration Declaration
}

// ReadDeclaration retains the original root-controlled declaration.
func ReadDeclaration(files *artifacts.PlatformInputs, path string) (RetainedDeclaration, error) {
	if files == nil || !below("/etc/ardents", path) {
		return RetainedDeclaration{}, ErrInput
	}
	raw, err := files.Read(path, 16<<10, false)
	if err != nil {
		return RetainedDeclaration{}, err
	}
	var declaration Declaration
	if artifacts.DecodeObject(raw, &declaration) != nil {
		return RetainedDeclaration{}, ErrInput
	}
	return RetainedDeclaration{files: files, path: path, raw: raw, declaration: declaration}, nil
}

// Role selects the fixed Endpoint or infrastructure consumer.
func (input RetainedDeclaration) Role() string { return input.declaration.Role }

// Bytes borrows the exact declaration for the selected Endpoint consumer.
func (input RetainedDeclaration) Bytes() []byte { return input.raw }

// Verify performs the selected native operation on the original retained inputs.
// Rendering emits the admitted unit while the input owner can still reobserve it.
func (input RetainedDeclaration) Verify(ctx context.Context, operation Operation, expectedPin string, emit func([]byte) error) (result error) {
	if input.files == nil || operation < RenderUnit || operation > CheckStart || (operation == RenderUnit && emit == nil) {
		return ErrInput
	}
	files, path, raw, decl := input.files, input.path, input.raw, input.declaration
	if !valid(decl) {
		return ErrInput
	}
	pin := digest(raw)
	if operation == CheckStart && expectedPin != pin {
		return ErrInput
	}
	plan, err := files.Read(decl.Configuration, 64<<10, false)
	if err != nil || digest(plan) != decl.ConfigSHA256 {
		return ErrInput
	}
	roots, err := writableRoots(decl.Role, plan)
	if err != nil || !equalRoots(roots, decl.WritableRoots) {
		return ErrInput
	}
	names, err := credentialPaths(decl.Role, plan)
	if err != nil || len(names) != len(decl.Inputs) {
		return ErrInput
	}
	for index, name := range names {
		raw, err := files.ReadRuntime(name, 64<<10, index == 1)
		if err != nil || digest(raw) != decl.Inputs[name] {
			return ErrInput
		}
	}
	unit := decl.unit(path, pin).Render()
	if operation == RenderUnit {
		if err := files.Check(ctx); err != nil {
			return err
		}
		return emit(unit)
	}
	// Runtime users cannot rename a retained trust root or private distribution
	// through a writable ancestor. The verifier still owns the root and floors;
	// these are independent read-only ancestor observations, not its lease.
	if err := files.RetainDirectory(filepath.Dir(decl.ReleaseRoot)); err != nil {
		return err
	}
	if err := files.RetainDirectory(decl.Distribution); err != nil {
		return err
	}

	var inventory interface {
		File(string) ([]byte, bool)
		Facts() (enrollment.InfrastructureFacts, bool)
	}
	var b enrollment.InfrastructureBundle
	var c enrollment.InfrastructureCandidate
	if operation == InitializeTrust {
		b, err = enrollment.VerifyInfrastructure(ctx, decl.Distribution, expectedPin)
		inventory = b
	} else {
		c, err = enrollment.ReadInfrastructureCandidate(ctx, decl.Distribution)
		inventory = c
	}
	if err != nil {
		return err
	}
	facts, ok := inventory.Facts()
	if !ok || facts.Invocation != decl.Role {
		return ErrInput
	}
	program, _ := inventory.File(infrastructure.ProgramName)
	template, _ := inventory.File("ardents-" + decl.Role + ".service")
	if !bytes.Equal(template, []byte(systemd.PlatformUnitTemplate)) {
		return ErrInput
	}
	actual, err := files.Read(decl.Program, 64<<20, true)
	if err != nil || !bytes.Equal(actual, program) {
		return ErrInput
	}
	// The privileged precheck itself must be the original private enrolled
	// executable. The public runtime copy is independently compared above.
	if err := artifacts.CheckRunningExecutable(filepath.Join(decl.Distribution, infrastructure.ProgramName), program); err != nil {
		return err
	}
	var sourceInspection state.SourceInspection
	var sourceUID uint32
	if decl.Role == "source" && operation == CheckStart {
		// The exact root-controlled plan is reused below and remains under the
		// existing file owner. This inspection creates no State or producer.
		defer func() { result = errors.Join(result, sourceInspection.Close()) }()
	}
	if operation == CheckStart {
		uid, err := readPlatformAccount(files, systemd.InfrastructureAccount(decl.Role))
		if err != nil {
			return err
		}
		installed, err := files.Read("/etc/systemd/system/ardents-"+decl.Role+".service", 64<<10, false)
		if err != nil || !bytes.Equal(installed, unit) {
			return ErrInput
		}
		if err := decl.unit(path, pin).ObservePrestart(ctx); err != nil {
			return err
		}
		if decl.Role == "source" {
			sourceUID = uid
			config, err := sourceInspectionConfig(plan)
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
	in := release.Inputs{RootBytes: root, Files: metadata, Artifact: program, TargetPath: infrastructure.ProgramTarget, Local: release.LocalEnvironment{Environment: decl.Environment, Network: decl.Network, Platform: "linux-amd64", Architecture: "amd64", RefTime: time.Now().UTC()}}
	var verifier *release.Verifier
	if operation == InitializeTrust {
		verifier, err = release.Open(decl.ReleaseRoot)
	} else {
		verifier, err = release.OpenRetained(decl.ReleaseRoot)
	}
	if err != nil {
		return err
	}
	var authorization infrastructure.RuntimeAuthorization
	if operation == InitializeTrust {
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
	if err == nil && operation == CheckStart {
		err = decl.unit(path, pin).ObservePrestart(ctx)
	}
	if err == nil && decl.Role == "source" && operation == CheckStart {
		err = sourceInspection.Check(ctx, plan, sourceUID, sourceUID)
	}
	// Returning success from ExecStartPre happens only after the genuine
	// verifier and every original file observer physically close successfully.
	return errors.Join(err, verifier.Close())
}

func sourceInspectionConfig(raw []byte) (state.Config, error) {
	plan, err := servingdeclaration.DecodeServingDeclaration(raw)
	if err != nil || !serving.ValidDeclaration(plan) {
		return state.Config{}, ErrInput
	}
	config, err := (*state.AuthorityDeclaration)(plan.Network).Config()
	if err != nil {
		return state.Config{}, err
	}
	config.LocalRoleStateRoot = plan.ParticipationRoot
	return config, nil
}

func readPlatformAccount(files *artifacts.PlatformInputs, name string) (uint32, error) {
	passwd, group, err := files.ReadAccountFiles()
	if err != nil {
		return 0, err
	}
	uid, ok := process.DedicatedAccountID(passwd, group, name)
	if !ok {
		return 0, ErrInput
	}
	return uid, nil
}

func digest(raw []byte) string {
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}
