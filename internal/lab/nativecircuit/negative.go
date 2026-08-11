package nativecircuit

import (
	"bytes"
	"context"
	"crypto/ecdh"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"encoding/binary"
	"errors"
	"io"
	"net"
	"sync"
	"sync/atomic"
	"time"
)

// RunNegative executes one fixed fail-closed R-013 negative in the qualified
// application image. Success means the invalid input was rejected before any
// unauthenticated Application bytes were accepted.
func RunNegative(ctx context.Context, name string) error {
	switch name {
	case "wrong-instance":
		return negativeWrongInstance(ctx)
	case "modified-record":
		return negativeModifiedRecord(ctx)
	case "replay":
		return negativeInvitation(true)
	case "wrong-binding":
		return negativeInvitation(false)
	case "oversized-frame":
		input := make([]byte, 4)
		binary.BigEndian.PutUint32(input, maximumFrameLength+1)
		if _, err := readFrame(bytes.NewReader(input)); err == nil {
			return errors.New("oversized frame was accepted")
		}
		return nil
	case "invalid-state":
		manager := newIntroductionManager()
		slot, err := randomCryptoHandle()
		if err != nil {
			return err
		}
		if err := manager.deliver(ctx, slot, []byte("sealed")); err == nil {
			return errors.New("unregistered Introduction transition was accepted")
		}
		return nil
	default:
		return errors.New("negative case is outside the fixed R-013 set")
	}
}

func negativeWrongInstance(ctx context.Context) error {
	active, err := generateEndpointFixture()
	if err != nil {
		return err
	}
	wrongChain, wrongKey, err := generateAlternateEndpointLeaf(active)
	if err != nil {
		return err
	}
	wrongCertificate, err := tls.X509KeyPair(wrongChain, wrongKey)
	if err != nil {
		return err
	}
	roots := x509.NewCertPool()
	if !roots.AppendCertsFromPEM(active.rootPEM) {
		return errors.New("active Target root is invalid")
	}
	leaf, err := parseDigest(active.leafSHA256)
	if err != nil {
		return err
	}
	nonce, err := randomCryptoHandle()
	if err != nil {
		return err
	}
	user, service := net.Pipe()
	done := make(chan error, 1)
	go func() { _, serviceErr := runEndpointService(ctx, service, wrongCertificate, nonce); done <- serviceErr }()
	observation, userErr := runEndpointUser(ctx, user, endpointTrust{Roots: roots, LeafSHA256: leaf}, nonce, []byte("forbidden"))
	<-done
	if userErr == nil || observation.ApplicationBytesVerified || observation.ApplicationBytes != 0 {
		return errors.New("wrong Instance reached the Application stream")
	}
	return nil
}

func negativeModifiedRecord(ctx context.Context) error {
	fixture, err := generateEndpointFixture()
	if err != nil {
		return err
	}
	certificate, err := tls.X509KeyPair(fixture.chainPEM, fixture.privatePEM)
	if err != nil {
		return err
	}
	roots := x509.NewCertPool()
	if !roots.AppendCertsFromPEM(fixture.rootPEM) {
		return errors.New("target root is invalid")
	}
	leaf, err := parseDigest(fixture.leafSHA256)
	if err != nil {
		return err
	}
	nonce, err := randomCryptoHandle()
	if err != nil {
		return err
	}
	user, proxyUser := net.Pipe()
	proxyService, service := net.Pipe()
	serviceDone := make(chan error, 1)
	go func() {
		_, serviceErr := runEndpointService(ctx, service, certificate, nonce)
		serviceDone <- serviceErr
	}()
	var armed sync.Once
	ready := make(chan struct{})
	var modified atomic.Bool
	proxyDone := make(chan error, 2)
	go func() {
		encrypted := 0
		proxyDone <- copyNativeTLSRecords(proxyUser, proxyService, func(recordType byte, _ []byte) {
			if recordType == 23 {
				encrypted++
				if encrypted == 2 {
					armed.Do(func() { close(ready) })
				}
			}
		})
	}()
	go func() {
		proxyDone <- copyNativeTLSRecords(proxyService, proxyUser, func(recordType byte, payload []byte) {
			if recordType == 23 && len(payload) > 0 && !modified.Load() {
				select {
				case <-ready:
					payload[len(payload)-1] ^= 1
					modified.Store(true)
				default:
				}
			}
		})
	}()
	observation, userErr := runEndpointUser(ctx, user, endpointTrust{Roots: roots, LeafSHA256: leaf}, nonce, []byte("forbidden"))
	_ = proxyUser.Close()
	_ = proxyService.Close()
	<-proxyDone
	<-proxyDone
	<-serviceDone
	if userErr == nil || !modified.Load() || observation.ApplicationBytesVerified {
		return errors.New("modified protected record reached the Application stream")
	}
	return nil
}

func negativeInvitation(replay bool) error {
	privateKey, err := ecdh.X25519().GenerateKey(rand.Reader)
	if err != nil {
		return err
	}
	now := time.Now()
	join, err := randomCryptoHandle()
	if err != nil {
		return err
	}
	nonce, err := randomCryptoHandle()
	if err != nil {
		return err
	}
	sealed, err := sealInvitation(privateKey.PublicKey(), invitation{
		Profile: candidateProfile, RunID: "negative", Rendezvous: "rv:37001",
		JoinToken: join, HandshakeNonce: nonce, ExpiresUnix: now.Add(time.Minute).Unix(),
	})
	if err != nil {
		return err
	}
	guard := newInvitationGuard(candidateProfile, "negative", "rv:37001", now)
	if replay {
		if _, err := guard.open(privateKey, sealed); err != nil {
			return err
		}
	} else {
		guard = newInvitationGuard(c3Profile, "negative", "rv:37001", now)
	}
	if _, err := guard.open(privateKey, sealed); err == nil {
		return errors.New("replayed or wrongly bound invitation was accepted")
	}
	return nil
}

func copyNativeTLSRecords(reader io.Reader, writer io.Writer, inspect func(byte, []byte)) error {
	for {
		header := make([]byte, 5)
		if _, err := io.ReadFull(reader, header); err != nil {
			return err
		}
		size := int(binary.BigEndian.Uint16(header[3:5]))
		if size < 1 || size > 18*1024 {
			return io.ErrUnexpectedEOF
		}
		payload := make([]byte, size)
		if _, err := io.ReadFull(reader, payload); err != nil {
			return err
		}
		inspect(header[0], payload)
		if err := writeAll(writer, header); err != nil {
			return err
		}
		if err := writeAll(writer, payload); err != nil {
			return err
		}
	}
}
