//go:build darwin

package worker

import (
	"context"
	"errors"
	"net"
	"os"
)

// errNativeUnavailable is the explicit typed refusal of the macOS native worker
// launch capability. The worker launch is not a bare process spawn: it is a
// confinement and lifetime contract (G1–G6) that the selected Linux adapter
// proves through the installed systemd/cgroup mechanism. macOS has no admitted
// mechanism that satisfies the same guarantees today, so this adapter refuses
// before any artifact, activation or cleanup pin rather than admitting a weaker
// path (ADR-0129). This mirrors the per-frame peer-credential refusal in
// peercred_darwin.go: a missing required protection is an explicit unavailable
// outcome, never a weaker accepting path.
//
// The missing guarantees, relative to the Linux systemd/cgroup adapter:
//
//   - G1 dedicated identity: the worker must run as a dedicated dynamic UID
//     (61184–65519) distinct from the calling Endpoint. Linux proves this through
//     DynamicUser plus LookupDynamicUserByName; macOS launchd has no admitted
//     equivalent binding here.
//   - G2 confinement: RootDirectory into the immutable installed root,
//     ProtectSystem=strict, PrivateNetwork/IPC/Devices/Tmp, NoNewPrivileges,
//     CapabilityBoundingSet=0, the seccomp deny-list, MemoryMax/TasksMax and
//     AF_UNIX-only address families. macOS sandbox/chroot/setrlimit could express
//     much of this, but no profile is selected or proven for the closed C0 scope,
//     and there is no exact seccomp deny-list equivalent.
//   - G3 lifetime binding: BindsTo+After ties the worker to loss of the Endpoint
//     service even on an uncatchable exit; socket EOF alone cannot terminate a
//     hostile fork. macOS has no admitted kernel lifetime binding here.
//   - G4 original-invocation identity: InvocationID plus the pinned cgroup bind
//     cleanup to the original invocation, never a replacement.
//   - G5 no residual execution: cgroup.events ENODEV/populated=0 is kernel-verified
//     process absence (escape-proof). A macOS process-group kill is escapable and
//     is not the same proof.
//   - G6 per-frame peer-credential authentication: already refused separately by
//     peercred_darwin.go (macOS exposes only connect-time LOCAL_PEERCRED).
//
// A macOS native confinement profile that proves G1–G6 through macOS mechanisms
// (sandbox/chroot/setrlimit/process-group/launchd/audit-token/crypto channel
// binding) is a separate future slice requiring its own research, ADR and
// per-guarantee security-equivalence evidence; it is not admitted by this adapter.
// The portable orchestration in activate.go is shared so that such an adapter
// plugs into the same sequence rather than duplicating it.
var errNativeUnavailable = errors.New("ordinary Execution native platform is unavailable")

// Artifact is the refusal adapter's empty pin: macOS creates no artifact.
type Artifact struct{}

// Cleanup is the refusal adapter's empty owner: macOS joins no original cgroup.
type Cleanup struct{}

// Instance is one detached original invocation observation. It grants no
// local permission, cleanup completion or right to signal a replacement.
type Instance struct {
	Name       string
	Role       string
	Cgroup     string
	PID        uint32
	UID        uint32
	Invocation [16]byte
}

// inventory is the refusal adapter's empty selector: macOS admits no inventory.
type inventory struct{}

func (inventory) socket(string) string { return "" }

var applicationInventory = inventory{}
var streamInventory = inventory{}

// listing is the refusal adapter's empty baseline observation.
type listing struct{}

// newAttachment binds the accepted local socket on the Unix-family transport.
// macOS reaches this only through the shared transport; the launch orchestration
// refuses at verifyEndpointService before any attachment is bound.
func newAttachment(connection *net.UnixConn, pid, uid uint32) *Attachment {
	return &Attachment{connection: connection, pid: pid, uid: uid}
}

// The capability steps below refuse with the typed unavailable outcome. The
// portable orchestration calls verifyEndpointService first, so macOS returns
// before any artifact, socket, baseline or cleanup pin is created.
func verifyEndpointService(context.Context) error { return errNativeUnavailable }

func loadArtifact(inventory) (*Artifact, error) { return nil, errNativeUnavailable }

func verifyActivationSocket(context.Context, inventory, string) error {
	return errNativeUnavailable
}

func inspectActivationSocket(inventory, string) (os.FileInfo, error) {
	return nil, errNativeUnavailable
}

func listInstances(context.Context, inventory, string) (listing, error) {
	return listing{}, errNativeUnavailable
}

func selectNewInstance(_, _ listing, _ string) (string, error) {
	return "", errNativeUnavailable
}

func observeInstance(context.Context, inventory, string, string) (Instance, error) {
	return Instance{}, errNativeUnavailable
}

func ObserveInstance(context.Context, string, string) (Instance, error) {
	return Instance{}, errNativeUnavailable
}

func ObserveInstanceStream(context.Context, string, string) (Instance, error) {
	return Instance{}, errNativeUnavailable
}

func NewCleanup(Instance, func(error)) (*Cleanup, error) {
	return nil, errNativeUnavailable
}

func (*Artifact) Verify() error { return errNativeUnavailable }
func (*Cleanup) Close() error   { return errNativeUnavailable }
