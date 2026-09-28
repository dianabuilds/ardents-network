//go:build linux

package service

import (
	"context"
	"crypto"
	"crypto/tls"
	"net"

	servicepublication "github.com/dianabuilds/ardents-network/internal/service/publication"
)

// This file is the deliberate test seam for the Endpoint root's protected
// TLS tests, which own the real fixture and therefore stay in the root
// package. Every symbol here is exercised only by tests; the deadcode
// allowlist records them as such.

// SecureClient completes the protected Service client handshake with the
// fixed profile groups.
func SecureClient(ctx context.Context, raw net.Conn, credential servicepublication.Credential,
	exporterContext [32]byte, generation uint64) (*SecuredAttachment, [32]byte, error) {
	return secureProtectedServiceClient(ctx, raw, credential, exporterContext, generation)
}

// SecurePublisher completes the protected Service publisher handshake with
// the fixed profile groups.
func SecurePublisher(ctx context.Context, raw net.Conn, credential servicepublication.Credential,
	signer crypto.Signer, exporterContext [32]byte, generation uint64) (*SecuredAttachment, [32]byte, error) {
	return secureProtectedServicePublisher(ctx, raw, credential, signer, exporterContext, generation)
}

// InstanceCertificate builds the exact single-certificate Instance chain.
func InstanceCertificate(credential servicepublication.Credential, signer crypto.Signer) (tls.Certificate, error) {
	return instanceCertificate(credential, signer)
}

// VerifyInstance returns the TLS connection verifier pinned to one Instance key.
func VerifyInstance(expected [32]byte) func(tls.ConnectionState) error {
	return verifyInstance(expected)
}

// NewProtectedTransport wraps raw in the exactly-once protected Service
// transport retirement.
func NewProtectedTransport(raw net.Conn) net.Conn {
	return &protectedServiceTransport{Conn: raw}
}

// NewRetirementCarrier wraps an authenticated TLS connection so a witnessed
// peer retirement maps its lower EOF to nativeconnection.ErrAttachmentRetired.
func NewRetirementCarrier(connection *tls.Conn) net.Conn {
	return &authenticatedRetirementCarrier{Conn: connection}
}

// Connection exposes the authenticated TLS connection of a secured attachment.
func (attachment *SecuredAttachment) Connection() *tls.Conn {
	return attachment.connection
}

// Binding exposes the retained authority seam of an opened stream so root
// fixtures can assert exact job ownership.
func (connection *Stream) Binding() Binding {
	if connection == nil {
		return nil
	}
	return connection.binding
}

// RunErr returns the internal terminal cause. Read it only after Finished
// has closed.
func (connection *Stream) RunErr() error { return connection.runErr }

// FinishErr returns the joined cleanup and native retirement result. Read it
// only after Finished has closed.
func (connection *Stream) FinishErr() error { return connection.finishErr }
