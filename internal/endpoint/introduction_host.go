//go:build linux

package endpoint

import (
	"context"
	"sync"
	"time"

	"github.com/dianabuilds/ardents-network/internal/endpoint/introduction"
)

// dutyIntroductionHost adapts the duty context to the introduction package's
// Host seam. It is constructed per call for the only two recovery transitions
// that need context authority; every Locked call still arrives with owner.mu
// held, exactly as the pre-extraction recovery methods did.
type dutyIntroductionHost struct {
	owner *dutyContext
}

var _ introduction.Host = dutyIntroductionHost{}

func (host dutyIntroductionHost) Mu() *sync.Mutex {
	return &host.owner.mu
}

func (host dutyIntroductionHost) Now() time.Time {
	return host.owner.endpoint.clock()
}

func (host dutyIntroductionHost) LeaseContext() context.Context {
	return host.owner.lease.Context()
}

func (host dutyIntroductionHost) FailDutyContexts(err error) {
	host.owner.endpoint.failDutyContexts(err)
}
