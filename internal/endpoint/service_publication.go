package endpoint

import (
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"errors"
	"time"

	"github.com/dianabuilds/ardents-network/internal/service/publication"
)

func (endpoint *endpoint) publish(ctx context.Context, input PublicationRequest) (result PublicationResult, err error) {
	receipt, consumeErr := endpoint.consume(input.Capability, input.Principal, "administration")
	if consumeErr != nil {
		return publicationDenied(consumeErr.Error())
	}
	result.Receipt = receipt
	if endpoint.publications == nil {
		return publicationFailed("service unavailable", "publisher has no publication owner", errors.New("publication root is unavailable"))
	}
	if err := validateCredential(input.Credential, endpoint.authority, endpoint.network, input.At, publishCapability|connectCapability); err != nil {
		return publicationFailed("service target authentication failure", "Service Credential is not valid for publication", err)
	}
	if len(input.IntroductionAcknowledgement) == 0 && input.IntroductionSocket != "" {
		acknowledgement, err := requestIntroductionAcknowledgement(ctx, input.IntroductionSocket,
			input.Credential, endpoint.broker, endpoint.resources)
		if err != nil {
			return publicationFailed("service unavailable", "Introduction acknowledgement request failed", err)
		}
		input.IntroductionAcknowledgement = acknowledgement
	}
	if !validAcknowledgement(input.IntroductionAcknowledgement, input.Credential, endpoint.network,
		endpoint.broker, endpoint.introduction) {
		return publicationFailed("service unavailable", "fresh Introduction publication acknowledgement is absent", errors.New("publication not acknowledged"))
	}
	if len(input.InstancePrivate) != ed25519.PrivateKeySize ||
		!input.InstancePrivate.Public().(ed25519.PublicKey).Equal(ed25519.PublicKey(input.Credential.InstancePublic[:])) {
		return publicationFailed("service target authentication failure", "matching Instance Key possession was not proved", errors.New("instance Key mismatch"))
	}
	current, err := endpoint.publications.Publish(ctx, publication.PublishInput{Credential: input.Credential,
		InstanceSigner: input.InstancePrivate, Acknowledgement: input.IntroductionAcknowledgement, At: input.At})
	if err != nil {
		return publicationFailed("service target authentication failure", "exclusive Instance generation could not be published", err)
	}
	endpoint.resources("control-file", 1)
	return PublicationResult{Class: "published", Record: current.Record,
		IntroductionReceipt:         sha256.Sum256(input.IntroductionAcknowledgement),
		IntroductionAcknowledgement: append([]byte(nil), input.IntroductionAcknowledgement...),
		AuthenticatedTarget:         current.Credential.Target, Generation: current.Credential.Generation}, nil
}

func (endpoint *endpoint) unpublish(ctx context.Context, input WithdrawalRequest) (result WithdrawalResult, err error) {
	receipt, consumeErr := endpoint.consume(input.Capability, input.Principal, "administration")
	if consumeErr != nil {
		return withdrawalDenied(consumeErr.Error())
	}
	result.Receipt = receipt
	if endpoint.publications == nil {
		return withdrawalFailed("service unavailable", "publisher has no publication owner", errors.New("publication root is unavailable"))
	}
	lease, err := endpoint.publications.AcquireAt(ctx, input.At)
	if err != nil {
		return withdrawalFailed("service unavailable", "Service is not currently published", err)
	}
	current := lease.Current()
	if err := lease.Close(); err != nil {
		return withdrawalFailed("service unavailable", "current publication could not be released", err)
	}
	if err := endpoint.publications.Unpublish(ctx); err != nil {
		return withdrawalFailed("service unavailable", "Service publication could not be withdrawn", err)
	}
	return WithdrawalResult{Class: "unpublished", AuthenticatedTarget: current.Credential.Target,
		Generation: current.Credential.Generation, Receipt: receipt}, nil
}

func decodePublication(encoded []byte, authority, network [32]byte, at time.Time) (Credential, error) {
	current, err := publication.Decode(encoded, ed25519.PublicKey(authority[:]), network, at)
	if err != nil {
		return Credential{}, err
	}
	return current.Credential, nil
}
