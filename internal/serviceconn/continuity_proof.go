package serviceconn

import (
	"context"
	"errors"
	"io"
	"time"

	nativeconnection "github.com/dianabuilds/ardents-network/internal/service/connection"
)

var errActiveViolation = errors.New("detected Service Connection integrity violation")

type continuityState struct {
	sendBase uint64
	sendEnd  uint64
	recvNext uint64
}

type peerContinuity struct {
	sendBase   uint64
	sendEnd    uint64
	recvNext   uint64
	peerNonce  [32]byte
	localNonce [32]byte
}

func exchangeContinuityProof(ctx context.Context, attachment *securedAttachment, continuity [32]byte,
	credential Credential, binding Recovery, client bool, state continuityState) (peerContinuity, error) {
	_ = credential
	_ = binding
	deadline := time.Now().Add(15 * time.Second)
	if value, ok := ctx.Deadline(); ok && value.Before(deadline) {
		deadline = value
	}
	_ = attachment.connection.SetDeadline(deadline)
	defer attachment.connection.SetDeadline(time.Time{})
	role, peerRole := nativeconnection.RoleClient, nativeconnection.RolePublisher
	if !client {
		role, peerRole = peerRole, role
	}
	local, err := nativeconnection.NewContinuity(continuity, role, attachment.generation, state.sendBase,
		state.sendEnd, state.recvNext, attachment.context, attachment.exporterCommitment)
	if err != nil {
		return peerContinuity{}, err
	}
	var record nativeconnection.Record
	if client {
		if err := nativeconnection.Write(attachment.connection, nativeconnection.Record{Continuity: &local}); err != nil {
			return peerContinuity{}, err
		}
		record, err = nativeconnection.Read(attachment.connection)
	} else {
		record, err = nativeconnection.Read(attachment.connection)
		if err == nil {
			err = nativeconnection.Write(attachment.connection, nativeconnection.Record{Continuity: &local})
		}
	}
	if err != nil || record.Continuity == nil || nativeconnection.VerifyContinuity(continuity, *record.Continuity,
		peerRole, attachment.generation, attachment.context, attachment.exporterCommitment) != nil {
		return peerContinuity{}, errActiveViolation
	}
	peer := record.Continuity
	return peerContinuity{sendBase: peer.SendBase, sendEnd: peer.SendEnd, recvNext: peer.ReceiveNext,
		peerNonce: peer.Nonce, localNonce: local.Nonce}, nil
}

func readNativeContinuity(reader io.Reader) (nativeconnection.Continuity, error) {
	record, err := nativeconnection.Read(reader)
	if err != nil || record.Continuity == nil {
		return nativeconnection.Continuity{}, errActiveViolation
	}
	return *record.Continuity, nil
}
