package main

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"ardents/sdk/go/client"
	sdkidentity "ardents/sdk/go/identity"
)

type identityFile struct {
	Credential       string `json:"credential"`
	DevicePrivateKey string `json:"device_private_key"`
}

type sessionSigner struct {
	principal  string
	credential *sdkidentity.Artifact
	device     ed25519.PrivateKey
}

func main() {
	if len(os.Args) != 4 {
		_, _ = fmt.Fprintln(os.Stderr, "usage: application-probe SOCKET NODE_PRINCIPAL APPLICATION_IDENTITY_FILE")
		os.Exit(2)
	}
	signer, err := loadSessionSigner(os.Args[3])
	if err != nil {
		fail(err)
	}
	defer clear(signer.device)
	application, err := client.New(client.Config{
		SocketPath: os.Args[1], NodePrincipal: os.Args[2], Signer: signer,
	})
	if err != nil {
		fail(err)
	}
	putGet(application)
}

func loadSessionSigner(path string) (*sessionSigner, error) {
	path = strings.TrimSpace(path)
	if strings.ContainsRune(path, '\x00') || !filepath.IsAbs(path) {
		return nil, fmt.Errorf("Application identity file path must be absolute")
	}
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() ||
		(runtime.GOOS != "windows" && info.Mode().Perm()&0o077 != 0) {
		return nil, fmt.Errorf("Application identity file is unavailable")
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("Application identity file is unavailable")
	}
	defer clear(raw)
	var encoded identityFile
	if json.Unmarshal(raw, &encoded) != nil {
		return nil, fmt.Errorf("Application identity file is invalid")
	}
	credentialRaw, err := base64.RawURLEncoding.DecodeString(encoded.Credential)
	if err != nil {
		return nil, fmt.Errorf("Application identity file is invalid")
	}
	defer clear(credentialRaw)
	deviceRaw, err := base64.RawURLEncoding.DecodeString(encoded.DevicePrivateKey)
	if err != nil || len(deviceRaw) != ed25519.PrivateKeySize {
		clear(deviceRaw)
		return nil, fmt.Errorf("Application identity file is invalid")
	}
	credential, err := sdkidentity.ParseKeyCredential(credentialRaw, time.Now().UTC())
	if err != nil {
		clear(deviceRaw)
		return nil, fmt.Errorf("Application identity file is invalid")
	}
	payload := credential.KeyCredential()
	if payload == nil || !bytes.Equal(payload.DevicePublicKey, ed25519.PrivateKey(deviceRaw).Public().(ed25519.PublicKey)) {
		clear(deviceRaw)
		return nil, fmt.Errorf("Application identity file is invalid")
	}
	return &sessionSigner{
		principal: payload.Subject, credential: credential,
		device: ed25519.PrivateKey(deviceRaw),
	}, nil
}

func (s *sessionSigner) Principal(context.Context) (string, error) {
	return s.principal, nil
}

func (s *sessionSigner) Credential(context.Context) (*sdkidentity.Artifact, error) {
	return s.credential, nil
}

func (s *sessionSigner) SignAuthenticationChallenge(_ context.Context, challenge sdkidentity.Challenge) ([]byte, error) {
	return sdkidentity.SignAuthenticationChallenge(challenge, s.credential, s.device)
}

func putGet(application *client.Client) {
	reference, err := application.Content.Put(context.Background(), []byte("native application probe"))
	if err != nil {
		fail(err)
	}
	payload, err := application.Content.Get(context.Background(), reference)
	if err != nil {
		fail(err)
	}
	if string(payload) != "native application probe" {
		fail(fmt.Errorf("unexpected content payload"))
	}
}

func fail(err error) {
	_, _ = fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
