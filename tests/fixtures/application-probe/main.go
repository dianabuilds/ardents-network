package main

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
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

type rootFile struct {
	PrivateSeed string `json:"root_private_seed"`
}

type sessionSigner struct {
	principal  string
	credential *sdkidentity.Artifact
	device     ed25519.PrivateKey
	root       ed25519.PrivateKey
}

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	switch os.Args[1] {
	case "create":
		if len(os.Args) != 4 {
			usage()
			os.Exit(2)
		}
		createIdentity(os.Args[2], os.Args[3])
	case "enroll":
		if len(os.Args) != 7 {
			usage()
			os.Exit(2)
		}
		enroll(os.Args[2], os.Args[3], os.Args[4], os.Args[5], os.Args[6])
	case "use":
		if len(os.Args) != 5 {
			usage()
			os.Exit(2)
		}
		use(os.Args[2], os.Args[3], os.Args[4])
	default:
		usage()
		os.Exit(2)
	}
}

func usage() {
	_, _ = fmt.Fprintln(os.Stderr, "usage: application-probe <create IDENTITY_FILE ROOT_FILE|enroll SOCKET NODE_PRINCIPAL TICKET_FILE IDENTITY_FILE ROOT_FILE|use SOCKET NODE_PRINCIPAL IDENTITY_FILE>")
}

func createIdentity(identityPath, rootPath string) {
	rootPublic, root, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		fail(err)
	}
	devicePublic, device, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		fail(err)
	}
	principal, err := sdkidentity.PrincipalID(rootPublic)
	if err != nil {
		fail(err)
	}
	deviceID, err := sdkidentity.DeviceID(devicePublic)
	if err != nil {
		fail(err)
	}
	now := time.Now().UTC().Truncate(time.Second)
	credential, err := sdkidentity.SignKeyCredential(sdkidentity.KeyCredentialSpec{
		Subject: principal, RootPublicKey: rootPublic,
		DeviceID: deviceID, DevicePublicKey: devicePublic,
		Purposes:  []sdkidentity.CredentialPurpose{sdkidentity.PurposeAuthenticate},
		NotBefore: now.Add(-time.Minute), NotAfter: now.Add(24 * time.Hour),
	}, root)
	if err != nil {
		fail(err)
	}
	raw, err := credential.MarshalBinary()
	if err != nil {
		fail(err)
	}
	defer clear(raw)
	writePrivateJSON(identityPath, identityFile{
		Credential:       base64.RawURLEncoding.EncodeToString(raw),
		DevicePrivateKey: base64.RawURLEncoding.EncodeToString(device),
	})
	writePrivateJSON(rootPath, rootFile{PrivateSeed: base64.RawURLEncoding.EncodeToString(root.Seed())})
	clear(root)
	clear(device)
	_, _ = fmt.Fprintln(os.Stdout, principal)
}

func enroll(socket, nodePrincipal, ticketPath, identityPath, rootPath string) {
	signer, err := loadSessionSigner(identityPath)
	if err != nil {
		fail(err)
	}
	defer clear(signer.device)
	signer.root, err = loadRoot(rootPath)
	if err != nil {
		fail(err)
	}
	defer clear(signer.root)
	encoded, err := readPrivateFile(ticketPath)
	if err != nil {
		fail(fmt.Errorf("Application enrollment ticket is unavailable"))
	}
	defer clear(encoded)
	ticket, err := client.ParseApplicationEnrollmentTicket(strings.TrimSpace(string(encoded)))
	if err != nil {
		fail(err)
	}
	result, err := client.EnrollApplication(context.Background(), client.EnrollmentConfig{
		SocketPath: socket, NodePrincipal: nodePrincipal, Ticket: ticket, Signer: signer,
	})
	if err != nil {
		fail(err)
	}
	if result.Principal != signer.principal {
		fail(fmt.Errorf("unexpected enrolled Principal"))
	}
}

func use(socket, nodePrincipal, identityPath string) {
	signer, err := loadSessionSigner(identityPath)
	if err != nil {
		fail(err)
	}
	defer clear(signer.device)
	application, err := client.New(client.Config{
		SocketPath: socket, NodePrincipal: nodePrincipal, Signer: signer,
	})
	if err != nil {
		fail(err)
	}
	putGet(application)
}

func (s *sessionSigner) SignEnrollmentChallenge(_ context.Context, challenge sdkidentity.Challenge) ([]byte, error) {
	return sdkidentity.SignEnrollmentChallenge(challenge, s.root)
}

func loadSessionSigner(path string) (*sessionSigner, error) {
	raw, err := readPrivateFile(path)
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

func loadRoot(path string) (ed25519.PrivateKey, error) {
	raw, err := readPrivateFile(path)
	if err != nil {
		return nil, fmt.Errorf("Application root file is unavailable")
	}
	defer clear(raw)
	var encoded rootFile
	if json.Unmarshal(raw, &encoded) != nil {
		return nil, fmt.Errorf("Application root file is invalid")
	}
	seed, err := base64.RawURLEncoding.DecodeString(encoded.PrivateSeed)
	if err != nil || len(seed) != ed25519.SeedSize {
		clear(seed)
		return nil, fmt.Errorf("Application root file is invalid")
	}
	defer clear(seed)
	return ed25519.NewKeyFromSeed(seed), nil
}

func readPrivateFile(path string) ([]byte, error) {
	path = strings.TrimSpace(path)
	if strings.ContainsRune(path, '\x00') || !filepath.IsAbs(path) {
		return nil, fmt.Errorf("protected file path must be absolute")
	}
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() ||
		(runtime.GOOS != "windows" && info.Mode().Perm()&0o077 != 0) {
		return nil, fmt.Errorf("protected file is unavailable")
	}
	return os.ReadFile(path)
}

func writePrivateJSON(path string, value any) {
	path = strings.TrimSpace(path)
	if strings.ContainsRune(path, '\x00') || !filepath.IsAbs(path) {
		fail(fmt.Errorf("protected output path must be absolute"))
	}
	raw, err := json.Marshal(value)
	if err != nil {
		fail(fmt.Errorf("protected output is invalid"))
	}
	defer clear(raw)
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		fail(fmt.Errorf("protected output is unavailable"))
	}
	if _, err = file.Write(raw); err != nil {
		_ = file.Close()
		fail(fmt.Errorf("protected output write failed"))
	}
	if err = file.Close(); err != nil {
		fail(fmt.Errorf("protected output write failed"))
	}
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
