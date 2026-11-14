package main

import "github.com/dianabuilds/ardents-network/internal/successor/endpoint/funding"

func fundingPlan(plan routePrefixPlan) funding.Plan {
	return funding.Plan{EntryRoot: plan.EntryRoot, InteriorRoot: plan.InteriorRoot,
		SourceInteriorRoot: plan.SourceInteriorRoot, ResponderInteriorRoot: plan.ResponderInteriorRoot,
		HostingRoot: plan.HostingRoot, Domain: plan.Domain, Deadline: plan.Deadline,
		Work: plan.Work, Termination: plan.Termination, Exclusions: plan.Exclusions}
}

func fundingAuthority(authority admissionAuthority) funding.Authority {
	return funding.Observe(authority.current, authority.witness, authority.presentation)
}
