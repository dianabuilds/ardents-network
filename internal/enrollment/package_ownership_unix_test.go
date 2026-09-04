//go:build !windows

package enrollment

import (
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestVerifyRejectsCallerOwnedPackageStaticRoot(t *testing.T) {
	root, request := enrolledFixture(t)
	artifactName := "ardents-linux-amd64"
	artifact, err := os.ReadFile(filepath.Join(root, artifactName))
	if err != nil {
		t.Fatal(err)
	}
	installed := filepath.Join(t.TempDir(), "usr", "lib", "ardents", "ardents")
	if err := os.MkdirAll(filepath.Dir(installed), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(installed, artifact, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(root, artifactName)); err != nil {
		t.Fatal(err)
	}
	request.ExecutablePath = installed
	request.ArtifactPath = installed
	if _, err := Verify(request); err == nil || !strings.Contains(err.Error(), "root-owned non-writable") {
		t.Fatalf("Verify(caller-owned package enrollment) = %v, want root-ownership rejection", err)
	}
}

func TestPackageOwnershipPredicates(t *testing.T) {
	tests := []struct {
		name    string
		verify  func(os.FileInfo) error
		info    packageOwnershipInfo
		wantErr bool
	}{
		{
			name:   "accepts root-owned non-writable package directory",
			verify: verifyPackageDirectory,
			info:   packageOwnershipInfo{mode: os.ModeDir | 0o755, uid: 0},
		},
		{
			name:    "rejects caller-owned package directory",
			verify:  verifyPackageDirectory,
			info:    packageOwnershipInfo{mode: os.ModeDir | 0o755, uid: 1000},
			wantErr: true,
		},
		{
			name:    "rejects group-writable package directory",
			verify:  verifyPackageDirectory,
			info:    packageOwnershipInfo{mode: os.ModeDir | 0o775, uid: 0},
			wantErr: true,
		},
		{
			name:   "accepts root-owned non-writable single-link package file",
			verify: verifyPackageFile,
			info:   packageOwnershipInfo{mode: 0o755, uid: 0, links: 1},
		},
		{
			name:    "rejects multiply-linked package file",
			verify:  verifyPackageFile,
			info:    packageOwnershipInfo{mode: 0o755, uid: 0, links: 2},
			wantErr: true,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := test.verify(test.info)
			if (err != nil) != test.wantErr {
				t.Fatalf("ownership verification error = %v, wantErr %t", err, test.wantErr)
			}
		})
	}
}

type packageOwnershipInfo struct {
	mode  os.FileMode
	uid   uint32
	links uint64
}

func (info packageOwnershipInfo) Name() string       { return "package-static-entry" }
func (info packageOwnershipInfo) Size() int64        { return 1 }
func (info packageOwnershipInfo) Mode() os.FileMode  { return info.mode }
func (info packageOwnershipInfo) ModTime() time.Time { return time.Time{} }
func (info packageOwnershipInfo) IsDir() bool        { return info.mode.IsDir() }
func (info packageOwnershipInfo) Sys() any {
	return &syscall.Stat_t{Uid: info.uid, Nlink: info.links}
}
