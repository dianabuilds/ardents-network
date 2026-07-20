package localrealm

import (
	"crypto/ed25519"
	"encoding/base64"
	"fmt"
	"io"

	identityapi "ardents/internal/identity/api"
)

func newAuthorityState(random io.Reader) (authorityState, error) {
	_, private, err := ed25519.GenerateKey(random)
	if err != nil {
		return authorityState{}, err
	}
	discovery, err := newChannelState(random)
	if err != nil {
		return authorityState{}, err
	}
	data, err := newChannelState(random)
	return authorityState{Version: authorityVersion,
		IssuerPrivate: base64.StdEncoding.EncodeToString(private), Discovery: discovery, Data: data}, err
}

func newChannelState(random io.Reader) (channelState, error) {
	id, secret := make([]byte, 16), make([]byte, 32)
	if _, err := io.ReadFull(random, id); err != nil {
		return channelState{}, err
	}
	if _, err := io.ReadFull(random, secret); err != nil {
		return channelState{}, err
	}
	return channelState{ID: base64.StdEncoding.EncodeToString(id),
		Secret: base64.StdEncoding.EncodeToString(secret), Generation: 1}, nil
}

func validateAuthority(state authorityState) (ed25519.PrivateKey, error) {
	key, err := base64.StdEncoding.DecodeString(state.IssuerPrivate)
	if err != nil || state.Version != authorityVersion || len(key) != ed25519.PrivateKeySize {
		return nil, fmt.Errorf("invalid authority")
	}
	for _, channel := range []channelState{state.Discovery, state.Data} {
		grant := grantState{ID: base64.StdEncoding.EncodeToString(make([]byte, 16))}
		if _, _, _, err := decodeGrantMaterial(channel, grant); err != nil || channel.Generation == 0 {
			return nil, fmt.Errorf("invalid channel")
		}
	}
	return ed25519.PrivateKey(key), nil
}

func decodeGrantMaterial(channel channelState, grant grantState) ([16]byte, identityapi.CapabilitySecret, [16]byte, error) {
	var channelID, grantID [16]byte
	rawChannel, err1 := base64.StdEncoding.DecodeString(channel.ID)
	rawSecret, err2 := base64.StdEncoding.DecodeString(channel.Secret)
	rawGrant, err3 := base64.StdEncoding.DecodeString(grant.ID)
	secret, ok := identityapi.NewCapabilitySecret(rawSecret)
	if err1 != nil || err2 != nil || err3 != nil || len(rawChannel) != 16 || len(rawGrant) != 16 || !ok || !secret.Valid() {
		return channelID, identityapi.CapabilitySecret{}, grantID, fmt.Errorf("invalid material")
	}
	copy(channelID[:], rawChannel)
	copy(grantID[:], rawGrant)
	return channelID, secret, grantID, nil
}

func loadAuthority(path string) (authorityState, error) {
	var state authorityState
	err := readPrivateJSON(path, &state)
	return state, err
}
