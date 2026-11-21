package main

import endpointpublisher "github.com/dianabuilds/ardents-network/internal/endpoint/publisher"

type publicationPlan struct {
	InstanceRoot string `json:"instance_root"`
	Root         string `json:"root"`
}

func publisherPlan(plan publicationPlan) endpointpublisher.Plan {
	return endpointpublisher.Plan{InstanceRoot: plan.InstanceRoot, Root: plan.Root}
}
