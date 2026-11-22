//go:build linux && network_signing_native

package main

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

// These are offline native output controls, never Release authentication or
// installed Source execution. Root, protected ancestors and actual ext4 are
// required by this explicit native profile; missing prerequisites fail.
func sourceArtifactNativeParent(t *testing.T) string {
	t.Helper()
	if os.Geteuid() != 0 {
		t.Fatal("invalid environment: native preparation requires root")
	}
	parent := t.TempDir()
	if err := os.Chmod(parent, 0700); err != nil {
		t.Fatal(err)
	}
	var fs syscall.Statfs_t
	if err := syscall.Statfs(parent, &fs); err != nil || fs.Type != 0xef53 {
		t.Fatal("invalid environment: actual ext4 required", err)
	}
	for path := parent; ; path = filepath.Dir(path) {
		info, err := os.Lstat(path)
		if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 || info.Mode().Perm()&0022 != 0 || info.Sys().(*syscall.Stat_t).Uid != 0 {
			t.Fatal("invalid environment: protected original ancestor required", path, err)
		}
		if path == "/" {
			break
		}
	}
	return parent
}

func TestSourcePreparationNativeOutputSubstitutionRefuses(t *testing.T) {
	for _, fault := range []string{"file-replacement", "hardlink", "directory-replacement", "late-extra", "closed-sync"} {
		t.Run(fault, func(t *testing.T) {
			parent := sourceArtifactNativeParent(t)
			out := filepath.Join(parent, "output")
			seen := false
			err := persistSourceArtifacts(t.Context(), out, []sourceArtifactFile{{"public", []byte("public bytes")}}, func() error { return nil }, func(stage string, file *os.File) {
				if seen || stage != "file-created" && stage != "before-directory-sync" {
					return
				}
				if (fault == "late-extra" || fault == "closed-sync") != (stage == "before-directory-sync") {
					return
				}
				seen = true
				var err error
				switch fault {
				case "file-replacement":
					err = os.Rename(filepath.Join(out, "public"), filepath.Join(parent, "retained"))
					if err == nil {
						err = os.WriteFile(filepath.Join(out, "public"), []byte("public bytes"), 0600)
					}
				case "hardlink":
					err = os.Link(filepath.Join(out, "public"), filepath.Join(parent, "alias"))
				case "directory-replacement":
					err = os.Rename(out, out+".retained")
					if err == nil {
						err = os.Mkdir(out, 0700)
					}
				case "late-extra":
					err = os.WriteFile(filepath.Join(out, "foreign"), []byte("keep"), 0600)
				case "closed-sync":
					err = file.Close()
				}
				if err != nil {
					t.Error(err)
				}
			})
			if !seen || err == nil {
				t.Fatal("substitution published", seen, err)
			}
		})
	}
}
