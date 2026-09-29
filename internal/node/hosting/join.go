package hosting

import (
	"context"
	"time"

	"github.com/dianabuilds/ardents-network/internal/node/authority"
	"github.com/dianabuilds/ardents-network/internal/resource"
	"github.com/dianabuilds/ardents-network/internal/route"
	"github.com/dianabuilds/ardents-network/internal/route/replay"
)

// joinHandle supplies the class-2 Hosting policy while JOIN owns the opened
// handle and closes it after its admitted children join.
type joinHandle struct {
	host   Handle
	source authority.Source
	now    func() time.Time
}

// NewJoinHandle adapts an opened provider-period handle to JOIN's Host surface.
func NewJoinHandle(host Handle, source authority.Source, now func() time.Time) *joinHandle {
	return &joinHandle{host: host, source: source, now: now}
}

func (hosting *joinHandle) Sample(ctx context.Context, age time.Duration) (resource.HostingSample, error) {
	return hosting.host.Sample(ctx, age)
}

func (hosting *joinHandle) AdmissionVerifier(receiver route.ClosedRoleReceiver) route.ClosedAdmissionVerifier {
	work, termination := joinEnvelope()
	return AdmissionVerifier(hosting.source, hosting.now, receiver, hosting.host, work, termination)
}

func (hosting *joinHandle) Replenisher(receiver route.ClosedRoleReceiver, spends *replay.Ledger) route.ClosedForwardingReplenisher {
	work, termination := joinEnvelope()
	return Replenisher(hosting.source, hosting.now, receiver, hosting.host, spends, work, termination)
}

func (hosting *joinHandle) Close() error { return hosting.host.Close() }

// The envelope includes both directions and transport/control overhead; the
// installed policy chooses which directions the actual provider charges.
func joinEnvelope() (resource.HostingTraffic, resource.HostingTraffic) {
	return resource.HostingTraffic{Tx: 64 << 20, Rx: 64 << 20}, resource.HostingTraffic{Tx: 1 << 20, Rx: 1 << 20}
}
