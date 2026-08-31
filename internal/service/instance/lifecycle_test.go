package instance

import (
	"crypto"
	"crypto/ed25519"
	"crypto/rand"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/service/publication"
)

func TestAcceptedResponsePublishesThroughNonExportingBindingAndRestartRequiresSuccessor(t *testing.T) {
	now := time.Date(2030, 4, 5, 6, 7, 8, 0, time.UTC)
	network := [32]byte{21}
	rootPath, publicationPath := t.TempDir(), t.TempDir()
	root, err := Initialize(InitializeConfig{Root: rootPath, NetworkID: network,
		NotBefore: now, NotAfter: now.Add(time.Hour)})
	if err != nil {
		t.Fatal(err)
	}
	authority, authorityPrivate, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	response := issuedResponse(t, root, authorityPrivate, 1)
	accepted, err := root.Accept(response)
	if err != nil || accepted.State != StateAccepted || accepted.Generation != 1 {
		t.Fatalf("accept response = %+v / %v", accepted, err)
	}
	repeated, err := root.Accept(response)
	if err != nil || repeated != accepted {
		t.Fatalf("repeat exact response = %+v / %v", repeated, err)
	}
	credential, err := root.Credential()
	if err != nil || credential.NetworkID != network || credential.Generation != accepted.Generation ||
		credential.AuthorityPublic != [32]byte(authority) {
		t.Fatalf("accepted public Credential = %+v / %v", credential, err)
	}
	binding, err := root.OpenBinding(0)
	if err != nil {
		t.Fatal(err)
	}
	message := []byte("host Instance proof")
	signature, err := binding.Sign(nil, message, crypto.Hash(0))
	if err != nil || !ed25519.Verify(ed25519.PublicKey(binding.Public().(ed25519.PublicKey)), message, signature) {
		t.Fatalf("non-exporting binding signature: %v", err)
	}
	owner, err := publication.Open(publication.Config{Root: publicationPath, NetworkID: network,
		Authority: authority, Clock: func() time.Time { return now }})
	if err != nil {
		t.Fatal(err)
	}
	current, err := owner.Publish(t.Context(), publication.PublishInput{Credential: binding.Credential(),
		InstanceSigner: binding, Acknowledgement: []byte("host Introduction acknowledgement"), At: now})
	if err != nil {
		t.Fatalf("publish opened binding: %v", err)
	}
	floor, err := owner.Floor()
	if err != nil || floor != current.Credential.Generation {
		t.Fatalf("publication floor = %d / %v", floor, err)
	}
	if err := binding.CommitPublished(floor); err != nil {
		t.Fatal(err)
	}
	if err := owner.Close(); err != nil {
		t.Fatal(err)
	}
	if err := root.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := Open(rootPath)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	if _, err := reopened.OpenBinding(floor); !errors.Is(err, ErrSuccessorRequired) {
		t.Fatalf("consumed generation reopened with error %v", err)
	}
}

func TestDifferentResponseConflictsAndWithdrawalErasesBinding(t *testing.T) {
	now := time.Date(2030, 5, 6, 7, 8, 9, 0, time.UTC)
	rootPath := t.TempDir()
	root, err := Initialize(InitializeConfig{Root: rootPath, NetworkID: [32]byte{31},
		NotBefore: now, NotAfter: now.Add(time.Hour)})
	if err != nil {
		t.Fatal(err)
	}
	_, firstAuthority, _ := ed25519.GenerateKey(rand.Reader)
	first := issuedResponse(t, root, firstAuthority, 1)
	if _, err := root.Accept(first); err != nil {
		t.Fatal(err)
	}
	binding, err := root.OpenBinding(0)
	if err != nil {
		t.Fatal(err)
	}
	_, otherAuthority, _ := ed25519.GenerateKey(rand.Reader)
	different := issuedResponse(t, root, otherAuthority, 1)
	if _, err := root.Accept(different); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("different response error = %v", err)
	}
	if _, err := binding.Sign(nil, []byte("must fail"), crypto.Hash(0)); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("conflicted binding sign error = %v", err)
	}
	if err := root.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := Open(rootPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := reopened.OpenBinding(0); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("conflicted root reopened with error %v", err)
	}
	_ = reopened.Close()

	withdrawRoot, err := Initialize(InitializeConfig{Root: t.TempDir(), NetworkID: [32]byte{32},
		NotBefore: now, NotAfter: now.Add(time.Hour)})
	if err != nil {
		t.Fatal(err)
	}
	_, authorityPrivate, _ := ed25519.GenerateKey(rand.Reader)
	if _, err := withdrawRoot.Accept(issuedResponse(t, withdrawRoot, authorityPrivate, 1)); err != nil {
		t.Fatal(err)
	}
	withdrawBinding, err := withdrawRoot.OpenBinding(0)
	if err != nil {
		t.Fatal(err)
	}
	if err := withdrawBinding.Withdraw(); err != nil {
		t.Fatal(err)
	}
	if _, err := withdrawBinding.Sign(nil, []byte("must fail"), crypto.Hash(0)); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("withdrawn binding sign error = %v", err)
	}
	if err := withdrawRoot.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestMalformedResponseIsTerminalAndAmbiguousRootFailsClosed(t *testing.T) {
	now := time.Date(2030, 6, 7, 8, 9, 10, 0, time.UTC)
	rootPath := t.TempDir()
	root, err := Initialize(InitializeConfig{Root: rootPath, NetworkID: [32]byte{41},
		NotBefore: now, NotAfter: now.Add(time.Hour)})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := root.Accept([]byte("malformed Authority response")); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("malformed response error = %v", err)
	}
	if _, err := root.OpenBinding(0); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("rejected generation binding error = %v", err)
	}
	if err := root.Close(); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(rootPath, ".ambiguous-recovery"), []byte("surplus"), 0o600); err != nil {
		t.Fatal(err)
	}
	if reopened, err := Open(rootPath); err == nil || reopened != nil {
		t.Fatalf("ambiguous Instance root reopened: %v", err)
	}
}

func issuedResponse(t *testing.T, root *Root, authority ed25519.PrivateKey, generation uint64) []byte {
	t.Helper()
	request, err := root.Request()
	if err != nil {
		t.Fatal(err)
	}
	view, err := ParseRequest(request)
	if err != nil {
		t.Fatal(err)
	}
	credential, err := (publication.Credential{InstancePublic: view.InstancePublic,
		IntroductionHPKEPublic: view.IntroductionPublic, Generation: generation,
		NotBefore: view.NotBefore, NotAfter: view.NotAfter, NetworkID: view.NetworkID,
		Capabilities: publication.CapabilityPublish | publication.CapabilityConnect}).Issue(authority)
	if err != nil {
		t.Fatal(err)
	}
	response, err := BuildResponse(request, credential)
	if err != nil {
		t.Fatal(err)
	}
	return response
}
