package messaging

import "strings"

const CodeSelectorMalformed = "privacy.selector.malformed"

func ValidateCarrierEnvelope(envelope SealedEnvelope) error {
	if err := ValidateOpaqueSelector(envelope.PubsubTopic, envelope.ContentTopic); err != nil {
		return err
	}
	return ValidateEncryptedPayload(envelope.Payload)
}

func ValidateOpaqueSelector(pubsubTopic, contentTopic string) error {
	const prefix, suffix, tokenSize = "/ardents/1/", "/proto", 32
	if pubsubTopic != DefaultPubsubTopic || !strings.HasPrefix(contentTopic, prefix) ||
		!strings.HasSuffix(contentTopic, suffix) {
		return envelopeError(CodeSelectorMalformed, "private carrier selector shape is invalid")
	}
	token := strings.TrimSuffix(strings.TrimPrefix(contentTopic, prefix), suffix)
	if len(token) != tokenSize {
		return envelopeError(CodeSelectorMalformed, "private carrier selector length is invalid")
	}
	for _, value := range token {
		if (value < 'a' || value > 'z') && (value < '2' || value > '7') {
			return envelopeError(CodeSelectorMalformed, "private carrier selector encoding is invalid")
		}
	}
	return nil
}

func ValidateEncryptedPayload(payload []byte) error {
	_, err := parseHeader(payload)
	return err
}
