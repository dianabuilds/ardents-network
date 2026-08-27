//go:build ignore

package main

import (
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"io"
	"time"

	"github.com/dianabuilds/ardents-network/internal/route"
)

const (
	initiatorRole       = byte(1)
	rendezvousRole      = byte(3)
	responderRole       = byte(4)
	maximumBindingBytes = 512
)

var (
	experimentNetwork = sha256.Sum256([]byte("r092-rendezvous-network"))
	experimentEpoch   = sha256.Sum256([]byte("r092-rendezvous-epoch"))
)

func clientBinding(side, token string, sender, rendezvous [32]byte, deadline time.Time) (route.LegBinding, error) {
	role := initiatorRole
	if side == "responder" {
		role = responderRole
	} else if side != "initiator" {
		return route.LegBinding{}, errors.New("binding side is unsupported")
	}
	return route.LegBinding{NetworkID: experimentNetwork, Epoch: 1, Digest: experimentEpoch,
		AttachmentID: sha256.Sum256([]byte(token)), SenderRole: role, PeerRole: rendezvousRole,
		SenderNodeID: sender, PeerNodeID: rendezvous, NotAfter: deadline.UTC()}, nil
}

func reciprocalBinding(peer route.LegBinding, rendezvous [32]byte) route.LegBinding {
	return route.LegBinding{NetworkID: peer.NetworkID, Epoch: peer.Epoch, Digest: peer.Digest,
		AttachmentID: peer.AttachmentID, SenderRole: rendezvousRole, PeerRole: peer.SenderRole,
		SenderNodeID: rendezvous, PeerNodeID: peer.SenderNodeID, NotAfter: peer.NotAfter}
}

func writeBinding(writer io.Writer, binding route.LegBinding) error {
	raw, err := route.EncodeLegBinding(binding)
	if err != nil {
		return err
	}
	if len(raw) > maximumBindingBytes {
		return errors.New("binding exceeds experiment bound")
	}
	header := []byte{byte(len(raw) >> 8), byte(len(raw))}
	if _, err := writer.Write(header); err != nil {
		return err
	}
	_, err = writer.Write(raw)
	return err
}

func readBinding(reader io.Reader) (route.LegBinding, error) {
	header := make([]byte, 2)
	if _, err := io.ReadFull(reader, header); err != nil {
		return route.LegBinding{}, err
	}
	length := int(binary.BigEndian.Uint16(header))
	if length == 0 || length > maximumBindingBytes {
		return route.LegBinding{}, errors.New("binding frame length is invalid")
	}
	raw := make([]byte, length)
	if _, err := io.ReadFull(reader, raw); err != nil {
		return route.LegBinding{}, err
	}
	return route.DecodeLegBinding(raw)
}

func validateIncomingBinding(binding route.LegBinding, state *tlsLegState, material identitySet,
	deadline time.Time) error {
	if binding.NetworkID != experimentNetwork || binding.Epoch != 1 || binding.Digest != experimentEpoch ||
		binding.PeerRole != rendezvousRole || binding.PeerNodeID != material.serverID ||
		!binding.NotAfter.Equal(deadline) || time.Now().UTC().After(binding.NotAfter) {
		return errors.New("binding context is unauthorized")
	}
	if binding.SenderRole != initiatorRole && binding.SenderRole != responderRole {
		return errors.New("binding sender role is unauthorized")
	}
	if state.peerID != binding.SenderNodeID {
		return errors.New("binding sender does not match TLS identity")
	}
	expected := material.initiatorID
	if binding.SenderRole == responderRole {
		expected = material.responderID
	}
	if binding.SenderNodeID != expected {
		return errors.New("binding identity is not authorized for its side")
	}
	return nil
}

type tlsLegState struct{ peerID [32]byte }
