package infrastructure

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestSourceRetirementBlockerRefusesCompleteBytesAfterUncertainSyncOrClose(t *testing.T) {
	root := t.TempDir()
	directory := filepath.Join(root, "source-startup")
	if err := os.Mkdir(directory, 0750); err != nil {
		t.Fatal(err)
	}
	blocker := filepath.Join(directory, "source-retirement-pending")
	if err := os.WriteFile(blocker, []byte("retained original refusal"), 0600); err != nil {
		t.Fatal(err)
	}
	// Failure-only physical control: there is no accepted Installation or domain
	// result. Complete-looking bytes cannot bypass the original blocker.
	if err := os.WriteFile(filepath.Join(directory, "source-retirement-journal"), sourceRetirementTranscriptVector(t), 0600); err != nil {
		t.Fatal(err)
	}
	selection := &sourceSelection{binding: sourceBinding{Request: SourceRequest{InstallationRoot: root}}}
	started := &sourceStartHistory{number: 0}
	if _, err := readSourceRetirement(selection, started); !errors.Is(err, ErrBinding) {
		t.Fatal("reader reached detached bytes through original blocker", err)
	}
	for _, failure := range []error{errors.New("original sync outcome uncertain"), os.ErrClosed, context.Canceled} {
		storage := &sourceStopStorage{ctx: context.Background(), selection: selection, directory: directory, closed: true, terminal: failure, pending: &sourceBirth{path: blocker}}
		if err := storage.publishRetirement(); !errors.Is(err, ErrBinding) {
			t.Fatal("uncertain/failed writer published retirement", err)
		}
		if _, err := os.Lstat(blocker); err != nil {
			t.Fatal("failed writer removed refusal barrier", err)
		}
	}
}

func TestSourceRetirementPublicationRefusesBeforeWriterClose(t *testing.T) {
	storage := &sourceStopStorage{ctx: context.Background()}
	if err := storage.publishRetirement(); !errors.Is(err, ErrBinding) {
		t.Fatal("incomplete writer published retirement", err)
	}
}
