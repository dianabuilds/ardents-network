package runtime

import (
	"context"

	"github.com/dianabuilds/ardents-network/internal/domain/execution"
	"github.com/dianabuilds/ardents-network/internal/domain/execution/worker"
)

// documentProtocol binds the fixed Text inventory and document INIT. A zero
// profile uses the Control observer for ordinary launch and preparation. The
// Reader and Publisher entry points each supply their own observer.
type documentProtocol struct {
	snapshot []byte
	start    func(*Invocation)
}

func (documentProtocol) activate(ctx context.Context, role string) (*worker.Activation, error) {
	return worker.Activate(ctx, role)
}

func (documentProtocol) observe(ctx context.Context, name, role string) (worker.Instance, error) {
	return worker.ObserveInstance(ctx, name, role)
}

func (documentProtocol) role(surface execution.Surface) string {
	if surface == execution.Administration {
		return "publisher"
	}
	return "reader"
}

func (protocol documentProtocol) startObservation(invocation *Invocation) {
	if protocol.start == nil {
		invocation.observeControlAttachment()
		return
	}
	protocol.start(invocation)
}
