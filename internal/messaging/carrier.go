// Package messaging owns private authenticated envelopes, selectors, and replay protection.
// It does not own carrier lifecycle or discovery knowledge.
package messaging

import "context"

type Carrier interface {
	PublishPrivateEnvelope(context.Context, SealedEnvelope) error
	FetchPrivateEnvelopes(context.Context, []string, string) ([]SealedEnvelope, error)
}

type LiveCarrier interface {
	PublishPrivateEnvelope(context.Context, SealedEnvelope) error
	SubscribePrivateEnvelopes(context.Context, string) (<-chan SealedEnvelope, error)
}
