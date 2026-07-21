// Package workload owns workload lifecycle orchestration and hosted-service readiness.
// It does not own network advertisement or remote discovery.
package workload

type RegisterWorkloadCommand struct {
	Spec SpecSnapshot `json:"spec"`
}

type StartWorkloadCommand struct {
	ID string `json:"id,omitempty"`
}

type StopWorkloadCommand struct {
	ID string `json:"id,omitempty"`
}

type RestartWorkloadCommand struct {
	ID string `json:"id,omitempty"`
}

type GetWorkloadStatusQuery struct {
	ID string `json:"id,omitempty"`
}

type ListWorkloadsQuery struct{}
