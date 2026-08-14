package recoverysmoke

import (
	"errors"
	"fmt"
	"path/filepath"
	"sort"

	"github.com/dianabuilds/ardents-network/internal/route"
)

type replacementPlan struct {
	selections []selectedRoute
	services   []string
}

const replacementSetupDeadline = "2s"

func configureReplacementPlans(root string, fixture prepared, lifetime string) (replacementPlan, error) {
	selections, err := replacementSelections(fixture.candidates)
	if err != nil || lifetime == "" {
		return replacementPlan{}, errors.Join(err, errors.New("replacement lifetime is required"))
	}
	plan := replacementPlan{selections: selections}
	used := make(map[string]bool, 12)
	for _, selection := range selections {
		for _, role := range replacementRoles {
			name, nameErr := candidateService(fixture.candidates, selection[role])
			if nameErr != nil {
				return replacementPlan{}, nameErr
			}
			used[name] = true
		}
	}
	for name := range used {
		plan.services = append(plan.services, name)
	}
	sort.Strings(plan.services)
	if err := configureClientReplacementPlan(root, fixture.candidates, selections, fixture.publisherRoutePublic, lifetime); err != nil {
		return replacementPlan{}, err
	}
	if err := configurePublisherReplacementPlans(root, selections, lifetime); err != nil {
		return replacementPlan{}, err
	}
	clientPeer, err := replacementPlanString(filepath.Join(root, "route", "plans", "initiator.json"), "UpstreamPin")
	if err != nil {
		return replacementPlan{}, err
	}
	for service := range used {
		if err := configureNodeReplacementPlan(root, fixture.candidates, service, selections, clientPeer,
			fixture.publisherRoutePublic, lifetime); err != nil {
			return replacementPlan{}, err
		}
	}
	for _, role := range []string{"client", "publisher"} {
		path := filepath.Join(root, "generations", "1", role+".json")
		if err := updatePlan(path, func(value map[string]any) { value["Lifetime"] = lifetime }); err != nil {
			return replacementPlan{}, err
		}
	}
	return plan, nil
}

func configureClientReplacementPlan(root string, candidates []route.Position,
	selections []selectedRoute, publisherPublic [32]byte, lifetime string) error {
	attempts := []map[string]any{{}, {}, {}, {}}
	for _, role := range replacementRoles {
		first := hex32(selections[0][role].NodeID)
		secondCandidate, err := roleCandidate(candidates, role, 1)
		if err != nil {
			return err
		}
		second := hex32(secondCandidate.NodeID)
		if role != "rendezvous" {
			attempts[1]["ExcludedIdentities"] = appendString(attempts[1]["ExcludedIdentities"], first)
			attempts[3]["ExcludedIdentities"] = appendString(attempts[3]["ExcludedIdentities"], first)
		}
		attempts[2]["ExcludedIdentities"] = appendString(attempts[2]["ExcludedIdentities"], first, second)
		if role == "rendezvous" {
			attempts[3]["ExcludedIdentities"] = appendString(attempts[3]["ExcludedIdentities"], first, second)
		}
	}
	attempts[2]["IntroductionSetupSocket"] = "/run/ardents/recovery-introduction-user/setup.sock"
	attempts[2]["IntroductionSetupPublic"] = hex32(selections[2]["introduction"].PublicKey)
	attempts[2]["IntroductionServicePublic"] = hex32(publisherPublic)
	path := filepath.Join(root, "route", "plans", "client.json")
	return updatePlan(path, func(value map[string]any) {
		delete(value, "Attachments")
		value["AttachmentPlans"], value["Lifetime"] = attempts, lifetime
		value["Deadline"] = replacementSetupDeadline
	})
}

func roleCandidate(candidates []route.Position, role string, ordinal int) (route.Position, error) {
	for _, candidate := range candidates {
		if candidate.Role != role {
			continue
		}
		if ordinal == 0 {
			return candidate, nil
		}
		ordinal--
	}
	return route.Position{}, errors.New("route candidate ordinal is outside the finite view")
}

func appendString(value any, additions ...string) []string {
	values, _ := value.([]string)
	return append(values, additions...)
}

func configureNodeReplacementPlan(root string, candidates []route.Position, service string,
	selections []selectedRoute, clientPeer string, publisherPublic [32]byte, lifetime string) error {
	position, err := serviceCandidate(candidates, service)
	if err != nil {
		return err
	}
	path := filepath.Join(root, "route", "plans", service+".json")
	var base map[string]any
	if err := updatePlan(path, func(value map[string]any) { base = value }); err != nil {
		return err
	}
	attempts := make([]map[string]any, 0, len(selections))
	for proposal, selection := range selections {
		current := selection[position.Role]
		if current.NodeID != position.NodeID {
			continue
		}
		index := routeRoleIndex(position.Role)
		upstream := base["UpstreamPin"]
		if index > 0 {
			upstream = hex32(selection[replacementRoles[index-1]].PublicKey)
		}
		var nextID, nextAddress, nextPin any
		if index < len(replacementRoles)-1 {
			next := selection[replacementRoles[index+1]]
			nextID, nextAddress, nextPin = hex32(next.NodeID), next.Endpoint, hex32(next.PublicKey)
		} else {
			nextID, nextAddress, nextPin = base["NextNodeID"], fmt.Sprintf("172.31.20.16:%d", 4605+proposal), base["NextPin"]
		}
		attempts = append(attempts, map[string]any{"UpstreamPin": upstream,
			"NextNodeID": nextID, "Next": nextAddress, "NextPin": nextPin})
	}
	return updatePlan(path, func(value map[string]any) {
		delete(value, "Attachments")
		value["AttachmentPlans"], value["Lifetime"] = attempts, lifetime
		value["Deadline"] = replacementSetupDeadline
		if position.Role == "introduction" && position.NodeID == selections[2]["introduction"].NodeID {
			value["IntroductionSetupSocket"] = "/run/ardents/recovery-introduction-user/setup.sock"
			value["IntroductionSetupPeer"] = clientPeer
			value["IntroductionForwardSocket"] = "/run/ardents/recovery-introduction-service/setup.sock"
			value["IntroductionForwardPublic"] = hex32(publisherPublic)
		}
	})
}

func candidateService(candidates []route.Position, wanted route.Position) (string, error) {
	match := 0
	for _, candidate := range candidates {
		if candidate.Role != wanted.Role {
			continue
		}
		match++
		if candidate.NodeID == wanted.NodeID {
			if match == 1 {
				return wanted.Role, nil
			}
			return wanted.Role + "-" + string(rune('0'+match)), nil
		}
	}
	return "", errors.New("selected Route candidate is outside the finite fixture")
}

func serviceCandidate(candidates []route.Position, service string) (route.Position, error) {
	for _, candidate := range candidates {
		name, err := candidateService(candidates, candidate)
		if err == nil && name == service {
			return candidate, nil
		}
	}
	return route.Position{}, errors.New("route service has no finite candidate")
}

func routeRoleIndex(wanted string) int {
	for index, role := range replacementRoles {
		if role == wanted {
			return index
		}
	}
	return -1
}
