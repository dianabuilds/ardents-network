//go:build !linux && !darwin

package worker

import (
	"context"
	"errors"
	"net"
	"os"
)

// There is no selected native worker on these platforms. The common real
// launch composition still consumes local admission and joins its refused
// Job, but this adapter cannot create an artifact, activation or cleanup pin.
// The worker launch is a confinement and lifetime contract (G1–G6) proven on
// Linux through the installed systemd/cgroup mechanism; Windows named-pipe
// impersonation is a different authentication model, not the per-frame
// SCM_CREDENTIALS binding and escape-proof cgroup cleanup this contract
// requires. Per ADR-0129 the missing required protection is an explicit
// unavailable outcome, never a weaker accepting path. The macOS refusal adapter
// (manager_darwin.go) documents the same guarantees for the Unix-family staging.
var errNativeUnavailable = errors.New("ordinary Execution native platform is unavailable")

type Artifact struct{}
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

// inventory is the refusal adapter's empty selector: no inventory is admitted.
type inventory struct{}

func (inventory) socket(string) string { return "" }

var applicationInventory = inventory{}
var streamInventory = inventory{}

// listing is the refusal adapter's empty baseline observation.
type listing struct{}

// newAttachment admits no bytes: the Windows refusal Attachment carries no
// socket or bound identity. The orchestration refuses before this is reached.
func newAttachment(*net.UnixConn, uint32, uint32) *Attachment { return &Attachment{} }

// prepareSocket refuses: Windows has no AF_UNIX per-frame peer-credential
// capability to enable on an accepted socket.
func prepareSocket(*net.UnixConn) error { return errNativeUnavailable }

// The capability steps below refuse with the typed unavailable outcome. The
// portable orchestration calls verifyEndpointService first, so this platform
// returns before any artifact, socket, baseline or cleanup pin is created.
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
