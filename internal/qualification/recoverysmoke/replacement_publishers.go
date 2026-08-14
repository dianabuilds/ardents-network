package recoverysmoke

import (
	"encoding/json"
	"fmt"
	"path/filepath"

	"github.com/dianabuilds/ardents-network/internal/qualification/byteio"
)

func configurePublisherReplacementPlans(root string, selections []selectedRoute, lifetime string) error {
	path := filepath.Join(root, "route", "plans", "publisher.json")
	raw, err := byteio.ReadFile(path, 64<<10)
	if err != nil {
		return err
	}
	var base map[string]any
	if err := json.Unmarshal(raw, &base); err != nil {
		return fmt.Errorf("decode publisher replacement plan: %w", err)
	}
	attempts := make([]map[string]any, len(selections))
	for proposal, selection := range selections {
		attempts[proposal] = map[string]any{"Listen": fmt.Sprintf("0.0.0.0:%d", 4605+proposal),
			"UpstreamPin": hex32(selection["responder"].PublicKey)}
		if proposal == 2 {
			attempts[proposal]["IntroductionSetupSocket"] = "/run/ardents/recovery-introduction-service/setup.sock"
			attempts[proposal]["IntroductionSetupPeer"] = hex32(selections[2]["introduction"].PublicKey)
			attempts[proposal]["IntroductionSetupNode"] = hex32(selections[2]["introduction"].NodeID)
			attempts[proposal]["ServiceCertificate"] = "/run/ardents/secrets/cert.pem"
			attempts[proposal]["ServiceKey"] = "/run/ardents/secrets/key.pem"
		}
	}
	delete(base, "Attachments")
	base["ConcurrentAttachments"], base["AttachmentPlans"], base["Lifetime"] = true, attempts, lifetime
	return byteio.WriteJSON(path, base, 64<<10)
}

func replacementPlanString(path, key string) (string, error) {
	raw, err := byteio.ReadFile(path, 64<<10)
	if err != nil {
		return "", err
	}
	var value map[string]any
	if err := json.Unmarshal(raw, &value); err != nil {
		return "", fmt.Errorf("decode replacement plan: %w", err)
	}
	result, ok := value[key].(string)
	if !ok || result == "" {
		return "", fmt.Errorf("replacement plan field %s is absent", key)
	}
	return result, nil
}
