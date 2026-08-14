package recoverysmoke

import (
	"crypto/sha256"
	"encoding/json"
	"net"
	"path/filepath"

	"github.com/dianabuilds/ardents-network/internal/qualification/byteio"
)

func configureRecoveryFixture(root string, fixture prepared) error {
	endpointValues := map[string]any{
		"BytesEachDirection": 4 << 20, "CandidateView": hex32(fixture.routeManifest),
		"IsolationContext":   hex32(sha256.Sum256(append([]byte("isolation\x00"), fixture.manifest[:]...))),
		"DestinationBinding": hex32(sha256.Sum256(append([]byte("destination\x00"), fixture.target[:]...))),
		"RouteProfile":       "h3-route-tracer-v1", "WorkSafetyNotAfter": fixture.credentials[0].NotAfter,
		"WorkSafetyMaximum": fixture.credentials[0].NotAfter, "NoNewRecoveryAfter": fixture.credentials[0].NotAfter,
	}
	for _, role := range []string{"client", "publisher"} {
		path := filepath.Join(root, "generations", "1", role+".json")
		if err := updatePlan(path, func(plan map[string]any) {
			for key, value := range endpointValues {
				plan[key] = value
			}
		}); err != nil {
			return err
		}
	}
	for _, role := range []string{"client", "initiator", "introduction", "rendezvous", "responder", "publisher"} {
		path := filepath.Join(root, "route", "plans", role+".json")
		if err := updatePlan(path, func(plan map[string]any) {
			plan["Attachments"] = 2
			applyRouteDeadline(plan, role)
			if role == "rendezvous" {
				plan["Next"] = "172.31.21.14:4604"
			}
			if listen, ok := plan["Listen"].(string); ok {
				_, port, err := net.SplitHostPort(listen)
				if err == nil {
					plan["Listen"] = net.JoinHostPort("0.0.0.0", port)
				}
			}
			if role == "client" || role == "publisher" {
				plan["RawAttachment"] = true
			}
			if role == "client" {
				delete(plan, "PublisherPin")
			}
			if role == "publisher" {
				delete(plan, "ServiceCertificate")
				delete(plan, "ServiceKey")
			}
		}); err != nil {
			return err
		}
	}
	return nil
}

func applyRouteDeadline(plan map[string]any, role string) {
	plan["Deadline"] = "15s"
	if role == "rendezvous" || role == "responder" {
		plan["Deadline"] = "10.5s"
	}
}

func updatePlan(path string, update func(map[string]any)) error {
	raw, err := byteio.ReadFile(path, 64<<10)
	if err != nil {
		return err
	}
	var plan map[string]any
	if err := json.Unmarshal(raw, &plan); err != nil {
		return err
	}
	update(plan)
	return byteio.WriteJSON(path, plan, 64<<10)
}

func setRouteAttachments(root string, count uint32) error {
	for _, role := range []string{"client", "initiator", "introduction", "rendezvous", "responder", "publisher"} {
		path := filepath.Join(root, "route", "plans", role+".json")
		if err := updatePlan(path, func(plan map[string]any) {
			delete(plan, "AttachmentPlans")
			delete(plan, "ConcurrentAttachments")
			delete(plan, "Lifetime")
			plan["Attachments"] = count
		}); err != nil {
			return err
		}
	}
	return nil
}
