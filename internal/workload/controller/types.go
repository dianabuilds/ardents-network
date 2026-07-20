package controller

import (
	"context"

	"ardents/internal/workload/desiredstate"
	"ardents/internal/workload/execution"
	"ardents/internal/workload/observedstate"
	domainworkload "ardents/internal/workload/workload"
)

const (
	DesiredPresent  = desiredstate.Present
	DesiredRunning  = desiredstate.Running
	DesiredStopped  = desiredstate.Stopped
	DesiredDisabled = desiredstate.Disabled
	DesiredRemoved  = desiredstate.Removed
)

const (
	ObservedAccepted  = observedstate.Accepted
	ObservedPreparing = observedstate.Preparing
	ObservedRunning   = observedstate.Running
	ObservedStopping  = observedstate.Stopping
	ObservedStopped   = observedstate.Stopped
	ObservedFailed    = observedstate.Failed
	ObservedDegraded  = observedstate.Degraded
	ObservedRemoved   = observedstate.Removed
)

const (
	DefaultRestartPolicy = domainworkload.DefaultRestartPolicy
	DefaultRestartBudget = 2
)

type ServiceSpec = domainworkload.ServiceSpec
type Spec = domainworkload.Spec
type PreparedWorkload = execution.PreparedWorkload
type Instance = execution.Instance
type PublishedServiceStatus = observedstate.PublishedServiceStatus
type Status = observedstate.Status

type Executor interface {
	Prepare(context.Context, Spec) (PreparedWorkload, error)
	Start(context.Context, PreparedWorkload) (Instance, error)
	Stop(context.Context, Instance) error
	Inspect(context.Context, string) (Instance, error)
}

type Remover interface {
	Remove(context.Context, Instance) error
}

type Inventory interface {
	Managed(context.Context) ([]Instance, error)
}

type AncillaryReconciler interface {
	ReconcileAncillary(context.Context, []Instance) error
}

type AdmissionFunc func(Spec, []Status) error

type persistentState struct {
	Items map[string]Status `json:"items"`
}
