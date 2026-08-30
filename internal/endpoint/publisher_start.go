package endpoint

import (
	"context"
	"crypto/sha256"
	"errors"
	"net"

	"github.com/dianabuilds/ardents-network/internal/application/broker"
	"github.com/dianabuilds/ardents-network/internal/service/publication"
)

func (endpoint *endpoint) startPublisher(ctx context.Context, input PublisherStartRequest) (PublicationResult, error) {
	receipt, consumeErr := endpoint.consume(input.Capability, input.Principal, "administration")
	if consumeErr != nil {
		return publicationDenied(consumeErr.Error())
	}
	endpoint.publisherMu.Lock()
	defer endpoint.publisherMu.Unlock()
	if endpoint.publications == nil || endpoint.publisherBinding == nil || endpoint.publisherSession != nil {
		return publisherStartFailed(receipt, "service unavailable", "Publisher start owner or exclusive slot is unavailable",
			errors.New("publisher start is unavailable"))
	}
	binding := endpoint.publisherBinding
	credential := binding.Credential()
	if err := validateCredential(credential, endpoint.authority, endpoint.network, input.At, publishCapability|connectCapability); err != nil {
		return publisherStartFailed(receipt, "service target authentication failure", "Service Instance binding is not valid for publication", err)
	}

	var slotTranscript []byte
	var slotConnection net.Conn
	readinessStarted := false
	current, err := endpoint.publications.PublishAfterReadiness(ctx, publication.PublishInput{
		Credential: credential, InstanceSigner: binding, At: input.At,
	}, func(readinessContext context.Context) ([]byte, error) {
		readinessStarted = true
		connection, transcript, openErr := endpoint.openPublisherIntroductionSlot(readinessContext,
			endpoint.publisherProfile, binding, credential)
		if openErr != nil {
			return nil, openErr
		}
		if commitErr := binding.CommitPublished(credential.Generation); commitErr != nil {
			return nil, errors.Join(commitErr, connection.Close())
		}
		slotConnection = connection
		slotTranscript = append([]byte(nil), transcript...)
		return transcript, nil
	})
	if err != nil {
		if slotConnection != nil {
			_ = slotConnection.Close()
		}
		if readinessStarted {
			err = errors.Join(err, binding.Withdraw())
			endpoint.publisherBinding = nil
		}
		return publisherStartFailed(receipt, "service unavailable", "Publisher publication and Introduction readiness did not commit", err)
	}
	lease, err := endpoint.publications.AcquireAt(ctx, input.At)
	if err != nil {
		if slotConnection != nil {
			_ = slotConnection.Close()
		}
		err = errors.Join(err, endpoint.publications.Unpublish(ctx), binding.Withdraw())
		endpoint.publisherBinding = nil
		return publisherStartFailed(receipt, "service unavailable", "committed Publisher generation could not be retained", err)
	}
	session := &PublisherIntroduction{endpoint: endpoint, profile: clonePublisherIntroductionProfile(endpoint.publisherProfile),
		recipient: binding, lease: lease, slot: slotConnection}
	endpoint.publisherSession = session
	endpoint.resources("control-file", 1)
	result := PublicationResult{Class: "published", Record: current.Record,
		IntroductionReceipt: sha256.Sum256(slotTranscript), AuthenticatedTarget: current.Credential.Target,
		Generation: current.Credential.Generation, Receipt: receipt}
	return result, nil
}

func publisherStartFailed(receipt broker.Receipt, class, reason string, cause error) (PublicationResult, error) {
	result, err := publicationFailed(class, reason, cause)
	result.Receipt = receipt
	return result, err
}
