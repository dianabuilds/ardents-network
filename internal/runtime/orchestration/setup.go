package orchestration

import (
	"time"

	appdata "ardents/internal/data"
	dataapi "ardents/internal/data/api"
	discovery "ardents/internal/discovery"
	transport "ardents/internal/network/api"
	policyapi "ardents/internal/policy/api"
	workloadcontroller "ardents/internal/workload/controller"
	"ardents/internal/workload/observedstate"
	domainworkload "ardents/internal/workload/workload"
)

func ApplyTrustAnchors(trust *discovery.TrustEvaluator, anchors []string) {
	if trust == nil {
		return
	}
	for _, anchor := range anchors {
		trust.Trust(anchor)
	}
}

func ConfigureLocalServices(
	policy policyapi.Service,
	workload *workloadcontroller.Service,
	data *appdata.Service,
	trans transport.Service,
	bootSources []string,
	bootstrapObserver func(transport.BootstrapDialReport),
) {
	if workload != nil && policy != nil {
		workload.SetAdmission(func(spec domainworkload.Spec, items []observedstate.Status) error {
			return policy.AdmitWorkload(spec, items)
		})
	}
	if data != nil && policy != nil {
		data.SetRetentionAuthorizer(func(blob dataapi.BlobSnapshot, relay bool, expiresAt time.Time) error {
			return policy.AllowBlobRetention(blob, relay, expiresAt, time.Now().UTC())
		})
	}
	if trans != nil {
		trans.SetBootstrapNodes(bootSources)
		trans.SetBootstrapObserver(bootstrapObserver)
	}
}
