package main

import "github.com/dianabuilds/ardents-network/internal/endpoint/funding"

func fundingPlan(plan routePrefixPlan) funding.Plan {
	return funding.Plan{EntryRoot: plan.EntryRoot, InteriorRoot: plan.InteriorRoot,
		SourceInteriorRoot: plan.SourceInteriorRoot, ResponderInteriorRoot: plan.ResponderInteriorRoot,
		HostingRoot: plan.HostingRoot, Domain: plan.Domain, Deadline: plan.Deadline,
		Work: plan.Work, Termination: plan.Termination, Exclusions: plan.Exclusions}
}
