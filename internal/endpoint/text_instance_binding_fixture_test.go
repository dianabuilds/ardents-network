//go:build linux

package endpoint

import (
	"crypto/ed25519"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/service/instance"
	"github.com/dianabuilds/ardents-network/internal/service/publication"
)

// serviceInstanceFixtureRoot creates the owner-only directory required by a
// Service Instance, independent of the test process umask.
func serviceInstanceFixtureRoot(t *testing.T) string {
	t.Helper()
	root := filepath.Join(t.TempDir(), "service-instance-root")
	if err := os.Mkdir(root, 0o700); err != nil {
		t.Fatal(err)
	}
	return root
}

// publicationStoreRoot creates the owner-only directory for a Publication root,
// independent of the test process umask.
func publicationStoreRoot(t *testing.T) string {
	t.Helper()
	root := filepath.Join(t.TempDir(), "publication")
	if err := os.Mkdir(root, 0o700); err != nil {
		t.Fatal(err)
	}
	return root
}

// acceptedInstanceBinding initializes one Service Instance generation and
// durably accepts the Authority-signed public Credential for it, returning the
// open owner-private binding used by the text runtime behavior tests.
func acceptedInstanceBinding(t *testing.T, rootPath string, network [32]byte, authority ed25519.PrivateKey,
	now, deadline time.Time,
) (*instance.Root, *instance.Binding) {
	t.Helper()
	root, err := instance.Initialize(instance.InitializeConfig{Root: rootPath, NetworkID: network, NotBefore: now, NotAfter: deadline})
	if err != nil {
		t.Fatal(err)
	}
	request, err := root.Request()
	if err != nil {
		t.Fatal(err)
	}
	view, err := instance.ParseRequest(request)
	if err != nil {
		t.Fatal(err)
	}
	credential, err := (publication.Credential{
		InstancePublic: view.InstancePublic,
		Generation:     1, NotBefore: view.NotBefore, NotAfter: view.NotAfter, NetworkID: view.NetworkID,
		Capabilities: publication.CapabilityPublish | publication.CapabilityConnect,
	}).Issue(authority)
	if err != nil {
		t.Fatal(err)
	}
	response, err := instance.BuildResponse(request, credential)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := root.Accept(response); err != nil {
		t.Fatal(err)
	}
	binding, err := root.OpenBinding(0)
	if err != nil {
		t.Fatal(err)
	}
	return root, binding
}
