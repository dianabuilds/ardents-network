package workload

import (
	"context"

	"ardents/internal/hosting"
	domain "ardents/internal/workload"
)

type Runtime interface {
	Get(string) (domain.StatusSnapshot, error)
	List() ([]domain.StatusSnapshot, error)
	Register(context.Context, domain.SpecSnapshot) error
	Start(context.Context, string) error
	Stop(context.Context, string) error
	Restart(context.Context, string) error
}

type Hosting interface {
	GetHostedService(string) (hosting.ServiceStatusSnapshot, error)
	ListHostedServices() ([]hosting.ServiceSnapshot, error)
	GetServicePublicationStatus(string) (hosting.PublicationSnapshot, error)
}

type Service struct {
	workload Runtime
	hosting  Hosting
}

func NewHandler(runtime Runtime, hosting Hosting) *Service {
	return &Service{workload: runtime, hosting: hosting}
}
