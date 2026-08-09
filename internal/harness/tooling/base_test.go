package tooling

import (
	"errors"
	"strings"
	"testing"
)

func TestVerifyLocalToolingBaseFailsClosedWhenMissingOrDifferent(t *testing.T) {
	t.Parallel()
	reference := "ubuntu@sha256:" + strings.Repeat("1", 64)
	identity := toolBundle{BaseImage: reference}

	missing := func(string, ...string) ([]byte, error) {
		return []byte("No such image"), errors.New("inspect failed")
	}
	if _, err := verifyLocalToolingBase(identity, missing); err == nil {
		t.Fatal("missing base image was accepted")
	}

	different := func(string, ...string) ([]byte, error) {
		return []byte(`[{"Id":"sha256:` + strings.Repeat("2", 64) + `","RepoDigests":["ubuntu@sha256:` + strings.Repeat("3", 64) + `"]}]`), nil
	}
	if _, err := verifyLocalToolingBase(identity, different); err == nil {
		t.Fatal("different base image digest was accepted")
	}
}

func TestVerifyLocalToolingBaseAcceptsExactLocalDigest(t *testing.T) {
	t.Parallel()
	reference := "ubuntu@sha256:" + strings.Repeat("1", 64)
	imageID := "sha256:" + strings.Repeat("2", 64)
	run := func(name string, arguments ...string) ([]byte, error) {
		if name != "docker" || len(arguments) != 3 || arguments[0] != "image" || arguments[1] != "inspect" || arguments[2] != reference {
			t.Fatalf("inspect invocation = %q %q", name, arguments)
		}
		return []byte(`[{"Id":"` + imageID + `","RepoDigests":["` + reference + `"]}]`), nil
	}
	observed, err := verifyLocalToolingBase(toolBundle{BaseImage: reference}, run)
	if err != nil {
		t.Fatal(err)
	}
	if observed != imageID {
		t.Fatalf("image ID = %q", observed)
	}
}
