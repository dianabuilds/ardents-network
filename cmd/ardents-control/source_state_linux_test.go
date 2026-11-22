package main

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestSourcePreparationInputsRetainActualFilesAndLatchLoss(t *testing.T) {
	for _, change := range []string{"replace", "same-inode", "cancel"} {
		t.Run(change, func(t *testing.T) {
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			path := filepath.Join(t.TempDir(), "public")
			original := []byte("original-public-input\n")
			if err := os.WriteFile(path, original, 0644); err != nil {
				t.Fatal(err)
			}
			inputs := &sourceControlInputs{ctx: ctx}
			defer inputs.close()
			raw, err := inputs.read(path, 128)
			if err != nil || !bytes.Equal(raw, original) {
				t.Fatal(string(raw), err)
			}
			if err := inputs.check(); err != nil {
				t.Fatal("own read changed original custody", err)
			}
			switch change {
			case "replace":
				if err := os.Rename(path, path+".retained"); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, original, 0644); err != nil {
					t.Fatal(err)
				}
			case "same-inode":
				if err := os.WriteFile(path, []byte("changed-original-input\n"), 0644); err != nil {
					t.Fatal(err)
				}
			case "cancel":
				cancel()
			}
			failure := inputs.check()
			if failure == nil || change == "cancel" && !errors.Is(failure, context.Canceled) {
				t.Fatal("original loss not retained", failure)
			}
			if change == "replace" {
				if err := os.Remove(path); err != nil {
					t.Fatal(err)
				}
				if err := os.Rename(path+".retained", path); err != nil {
					t.Fatal(err)
				}
			} else if change == "same-inode" {
				if err := os.WriteFile(path, original, 0644); err != nil {
					t.Fatal(err)
				}
			}
			if inputs.check() != failure {
				t.Fatal("restoration replaced original failure")
			}
		})
	}
}

func TestSourcePreparationReobservesEarlierNamespaceAfterLaterRead(t *testing.T) {
	for _, change := range []string{"unchanged", "file", "ancestor"} {
		t.Run(change, func(t *testing.T) {
			root := t.TempDir()
			earlier := filepath.Join(root, "earlier")
			if err := os.Mkdir(earlier, 0700); err != nil {
				t.Fatal(err)
			}
			first, last := filepath.Join(earlier, "public"), filepath.Join(root, "last")
			for _, path := range []string{first, last} {
				if err := os.WriteFile(path, []byte("same public bytes"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			inputs := &sourceControlInputs{ctx: t.Context()}
			defer inputs.close()
			for _, path := range []string{first, last} {
				if _, err := inputs.read(path, 128); err != nil {
					t.Fatal(err)
				}
			}
			lastFile := inputs.files[len(inputs.files)-1].file
			entered, release := make(chan struct{}), make(chan struct{})
			done := make(chan error, 1)
			go func() {
				done <- inputs.observeRead(func(file *os.File, raw []byte) (int, error) {
					if file == lastFile {
						close(entered)
						<-release
					}
					return file.ReadAt(raw, 0)
				})
			}()
			<-entered // All earlier native content and link checks have finished.
			var mutation error
			if change == "file" {
				mutation = os.Rename(first, first+".original")
				if mutation == nil {
					mutation = os.WriteFile(first, []byte("same public bytes"), 0600)
				}
			} else if change == "ancestor" {
				mutation = os.Rename(earlier, earlier+".original")
				if mutation == nil {
					mutation = os.Mkdir(earlier, 0700)
				}
				if mutation == nil {
					mutation = os.WriteFile(first, []byte("same public bytes"), 0600)
				}
			}
			close(release)
			err := <-done // Join the actual final native read before releasing inputs.
			if mutation != nil || (err == nil) != (change == "unchanged") {
				t.Fatalf("post-read namespace observation: change=%s mutation=%v result=%v", change, mutation, err)
			}
		})
	}
}
