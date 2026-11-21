package runtime

import (
	"context"

	"github.com/dianabuilds/ardents-network/internal/execution"
	"github.com/dianabuilds/ardents-network/internal/execution/worker"
)

// attachmentConsumer owns the original attachment observation for one qualified
// invocation. It is private to this package; the fixed public launch entries
// select it, and no caller supplies it. A nil consumer keeps the ordinary Text
// setup/Publisher/Reader observation dispatch unchanged.
type attachmentConsumer interface {
	observe(invocation *Invocation)
}

// workerProtocol selects one fixed root-installed worker inventory and its INIT
// and reobservation protocol. It is private: ordinary Text composition uses the
// text protocol through Launch/LaunchReader/LaunchPublisher, and the trusted
// qualification composition uses the stream protocol through
// LaunchStreamQualification. No caller-visible inventory selector exists.
type workerProtocol interface {
	activate(ctx context.Context, role string) (*worker.Activation, error)
	initialize(ctx context.Context, attachment *worker.Attachment, nonce [32]byte, role string) error
	observe(ctx context.Context, name, role string) (worker.Instance, error)
	role(surface execution.Surface) string
	consumer() attachmentConsumer
}

// textProtocol is the ordinary closed Text inventory. Its behavior is exactly
// the original launch path: worker.Activate, the platform Text INIT seam, and
// worker.ObserveInstance, with the existing attachment observation dispatch.
type textProtocol struct {
	snapshot []byte
}

func (textProtocol) activate(ctx context.Context, role string) (*worker.Activation, error) {
	return worker.Activate(ctx, role)
}

func (protocol textProtocol) initialize(ctx context.Context, attachment *worker.Attachment, nonce [32]byte, role string) error {
	return initializeWorker(ctx, attachment, role, nonce, protocol.snapshot)
}

func (textProtocol) observe(ctx context.Context, name, role string) (worker.Instance, error) {
	return worker.ObserveInstance(ctx, name, role)
}

func (textProtocol) role(surface execution.Surface) string {
	if surface == execution.Administration {
		return "publisher"
	}
	return "reader"
}

func (textProtocol) consumer() attachmentConsumer { return nil }
