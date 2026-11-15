package delivery

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"syscall"

	"github.com/dianabuilds/ardents-network/internal/successor/endpoint/delivery/generation"
	"github.com/dianabuilds/ardents-network/internal/successor/endpoint/delivery/journal"
)

// These records describe the original generation-file births. They do not
// supply an Installation lease, Release proof or permission to continue a prefix.
type generationFileInventory struct {
	Schema           string   `json:"schema"`
	GenerationDigest string   `json:"generation_digest"`
	BindingDigest    string   `json:"binding_digest"`
	Files            []string `json:"files"`
}

type generationFileBirth struct {
	Schema           string `json:"schema"`
	GenerationDigest string `json:"generation_digest"`
	Name             string `json:"name"`
	Device           uint64 `json:"device"`
	Inode            uint64 `json:"inode"`
	Digest           string `json:"digest"`
	Size             int64  `json:"size"`
	Mode             uint32 `json:"mode"`
	GID              uint32 `json:"gid"`
}

func generationFileRecordName(name string) string {
	return "generation-file-" + digestHex([]byte(name)) + ".json"
}

// readGenerationFileBirths uses the independent installed reader's original
// files and lease; the caller owns admission and retains that reader lifetime.
func readGenerationFileBirths(ctx context.Context, reader *installedRoot, directory string, selected generationSelection, binding generationBinding, expected map[string][]byte, inventory map[string]bool) (map[string]journal.RetainedRecord, error) {
	records := make(map[string]journal.RetainedRecord)
	markerPath := filepath.Join(directory, "generation-files.json")
	marker, markerErr := reader.read(ctx, markerPath, 4<<10, 0600, 0)
	present := !errors.Is(markerErr, os.ErrNotExist)
	if present {
		expectedMarker, err := canonicalJSON(generationFileInventory{Schema: "ardents-endpoint-generation-files-v1", GenerationDigest: selected.GenerationDigest, BindingDigest: selected.BindingDigest, Files: generation.Names()})
		if markerErr != nil || err != nil || !bytes.Equal(marker, expectedMarker) {
			return nil, errors.Join(ErrBinding, markerErr, err)
		}
		inventory["generation-files.json"] = true
		records["generation-files.json"] = journal.RetainedRecord{Identity: reader.files[markerPath].identity, Bytes: marker}
	}
	for _, name := range generation.Names() {
		path := filepath.Join(directory, generationFileRecordName(name))
		body, err := reader.read(ctx, path, 4<<10, 0600, 0)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if !present {
			return nil, ErrBinding
		}
		var birth generationFileBirth
		mode := uint32(0640)
		if name == "ardents-linux-amd64" || name == "ardents-text-linux-amd64" {
			mode = 0555
		}
		digest := binding.Files[name]
		if name == "binding.json" {
			digest = selected.BindingDigest
		}
		if err != nil || decodeCanonical(body, 4<<10, &birth) != nil || birth.Schema != "ardents-endpoint-generation-file-v1" || birth.GenerationDigest != selected.GenerationDigest || birth.Name != name || birth.Digest != digest || !canonicalDigest(digest) || birth.Size < 1 || birth.Size > 64<<20 || birth.Mode != mode || birth.GID != reader.gid || birth.Device == 0 || birth.Inode == 0 {
			return nil, errors.Join(ErrBinding, err)
		}
		if name == "binding.json" {
			encoded, err := canonicalJSON(binding)
			if err != nil || birth.Size != int64(len(encoded)) {
				return nil, errors.Join(ErrBinding, err)
			}
		} else if expected != nil && (len(expected[name]) == 0 || birth.Size != int64(len(expected[name]))) {
			return nil, ErrBinding
		}
		filename := filepath.Join(reader.lease.path, "generations", selected.GenerationDigest, name)
		actual, err := reader.readObserved(ctx, filename, birth.Size, os.FileMode(mode), reader.gid, true)
		observed := reader.files[filename]
		if err != nil || observed.identity == nil || int64(len(actual)) > birth.Size {
			return nil, errors.Join(ErrBinding, err)
		}
		native, ok := observed.identity.Sys().(*syscall.Stat_t)
		if !ok || uint64(native.Dev) != birth.Device || native.Ino != birth.Inode || int64(len(actual)) == birth.Size && digestHex(actual) != birth.Digest {
			return nil, ErrBinding
		}
		recordName := generationFileRecordName(name)
		inventory[recordName] = true
		records[recordName] = journal.RetainedRecord{Identity: reader.files[path].identity, Bytes: body}
	}
	if present && len(records) != len(generation.Names())+1 {
		return nil, ErrBinding
	}
	return records, ctx.Err()
}
