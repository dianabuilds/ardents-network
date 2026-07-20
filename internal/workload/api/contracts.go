package api

import "context"

type Service interface {
	GetWorkloadStatus(string) (WorkloadStatusSnapshot, error)
	ListWorkloads() ([]WorkloadStatusSnapshot, error)
	RegisterWorkload(WorkloadSpecSnapshot) error
	RegisterWorkloadContext(context.Context, WorkloadSpecSnapshot) error
	StartWorkload(string) error
	StopWorkload(string) error
	RestartWorkload(string) error
	StartWorkloadContext(context.Context, string) error
	StopWorkloadContext(context.Context, string) error
	RestartWorkloadContext(context.Context, string) error
}
