package testkit

import (
	"bytes"

	networkapi "ardents/internal/network/api"
	networkprivacy "ardents/internal/network/privacy"
)

const (
	FindingReadableTopic           = "privacy.capture.readable_topic"
	FindingReadablePayload         = "privacy.capture.readable_payload"
	FindingOpaqueSelectorInvalid   = "privacy.capture.opaque_selector_invalid"
	FindingEncryptedPayloadInvalid = "privacy.capture.encrypted_payload_invalid"
)

func InspectPrivateCapture(envelope networkapi.Envelope, forbidden ...[]byte) []string {
	var findings []string
	if err := networkprivacy.ValidateOpaqueSelector(envelope.PubsubTopic, envelope.ContentTopic); err != nil {
		findings = append(findings, FindingOpaqueSelectorInvalid)
	}
	if err := networkprivacy.ValidateEncryptedPayload(envelope.Payload); err != nil {
		findings = append(findings, FindingEncryptedPayloadInvalid)
	}
	for _, marker := range forbidden {
		if len(marker) == 0 {
			continue
		}
		if bytes.Contains([]byte(envelope.ContentTopic), marker) {
			findings = appendUniqueFinding(findings, FindingReadableTopic)
		}
		if bytes.Contains(envelope.Payload, marker) {
			findings = appendUniqueFinding(findings, FindingReadablePayload)
		}
	}
	return findings
}

func appendUniqueFinding(findings []string, value string) []string {
	for _, finding := range findings {
		if finding == value {
			return findings
		}
	}
	return append(findings, value)
}
