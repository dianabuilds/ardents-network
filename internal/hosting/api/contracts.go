package api

type Service interface {
	GetHostedService(string) (HostedServiceStatusSnapshot, error)
	ListHostedServices() ([]HostedServiceSnapshot, error)
	GetServicePublicationStatus(string) (PublicationStatusSnapshot, error)
}
