//go:build linux

package permissionfile

import (
	"bytes"
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

func TestPublicRequestRetryAndResponseHandover(t *testing.T) {
	directory := t.TempDir()
	if err := os.Chmod(directory, 0700); err != nil {
		t.Fatal(err)
	}
	requestPath, responsePath := filepath.Join(directory, "request"), filepath.Join(directory, "response")
	request := []byte("exact public request")
	requestFile, err := Open(requestPath)
	if err != nil {
		t.Fatal(err)
	}
	defer requestFile.Close()
	if err := requestFile.PublishRequest(request); err != nil {
		t.Fatal(err)
	}
	if err := requestFile.PublishRequest(request); err != nil {
		t.Fatalf("exact retry: %v", err)
	}
	if err := requestFile.PublishRequest([]byte("different request")); err == nil {
		t.Fatal("conflicting retry replaced request")
	}
	retained, err := os.ReadFile(requestPath)
	if err != nil || !bytes.Equal(retained, request) {
		t.Fatalf("retained request = %q / %v", retained, err)
	}
	response, err := Open(responsePath)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Close()
	if present, err := response.Present(); err != nil || present {
		t.Fatalf("response before approval = %t / %v", present, err)
	}
	approved := bytes.Repeat([]byte{3}, 228)
	if err := os.WriteFile(responsePath, approved, 0600); err != nil {
		t.Fatal(err)
	}
	if present, err := response.Present(); err != nil || !present {
		t.Fatalf("response after approval = %t / %v", present, err)
	}
	read, err := response.ReadResponse()
	if err != nil || !bytes.Equal(read, approved) {
		t.Fatalf("response bytes = %d / %v", len(read), err)
	}
}

func TestFilesRefuseUnsafeFilesystemInputs(t *testing.T) {
	rootPath := t.TempDir()
	if err := os.Chmod(rootPath, 0700); err != nil {
		t.Fatal(err)
	}
	root, name, err := openDirectory(filepath.Join(rootPath, "response"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := root.Close(); err != nil {
			t.Error(err)
		}
	}()
	regular := filepath.Join(rootPath, "regular")
	if err := os.WriteFile(regular, []byte("bounded"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, kind := range []string{"symlink", "hardlink", "fifo", "permissions", "oversize"} {
		t.Run(kind, func(t *testing.T) {
			path := filepath.Join(rootPath, name)
			switch kind {
			case "symlink":
				err = os.Symlink(regular, path)
			case "hardlink":
				err = os.Link(regular, path)
			case "fifo":
				err = syscall.Mkfifo(path, 0600)
			case "permissions":
				err = os.WriteFile(path, []byte("wide"), 0600)
				if err == nil {
					err = os.Chmod(path, 0644)
				}
			case "oversize":
				err = os.WriteFile(path, make([]byte, 229), 0600)
			}
			if err != nil {
				t.Fatal(err)
			}
			defer func() {
				if err := os.Remove(path); err != nil {
					t.Error(err)
				}
			}()
			if _, err := readFile(root, name, 228); err == nil {
				t.Fatal("unsafe input accepted")
			}
		})
	}
	if err := os.Chmod(rootPath, 0755); err != nil {
		t.Fatal(err)
	}
	if opened, _, err := openDirectory(filepath.Join(rootPath, name)); err == nil {
		if err := opened.Close(); err != nil {
			t.Error(err)
		}
		t.Fatal("shared directory accepted")
	}
	if _, _, err := openDirectory("relative/permission"); err == nil {
		t.Fatal("relative path accepted")
	}
}
