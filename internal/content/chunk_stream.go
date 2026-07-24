package content

import (
	"ardents/internal/identity/principal"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
)

const PlaintextChunkSize = 64 * 1024

func Stream(reader io.Reader, emit func(int, []byte) error) (int, int64, error) {
	if reader == nil || emit == nil {
		return 0, 0, fmt.Errorf("chunk stream dependencies are unavailable")
	}
	count := 0
	var total int64
	for {
		chunk := make([]byte, PlaintextChunkSize)
		read, err := io.ReadFull(reader, chunk)
		if err == io.EOF && read == 0 {
			if count == 0 {
				return 0, 0, fmt.Errorf("chunk stream payload is empty")
			}
			return count, total, nil
		}
		if err != nil && !errors.Is(err, io.ErrUnexpectedEOF) {
			return count, total, err
		}
		chunk = chunk[:read]
		if err := emit(count, chunk); err != nil {
			return count, total, err
		}
		count++
		total += int64(read)
		if errors.Is(err, io.ErrUnexpectedEOF) {
			return count, total, nil
		}
	}
}

const stagedChunkCheckpointInterval = 16

type ChunkedPayloadSpec struct {
	Owner     principal.ID
	MediaType string
	KeyID     string
	Access    string
	Retention string
}

type ChunkedPayloadResult struct {
	Root                Manifest
	ChunkCount          int
	TotalPlaintextBytes int64
}

type chunkStoreOutcome struct {
	IDs   []string
	Count int
	Total int64
}

func (s *Service) StoreChunkedPayload(
	ctx context.Context,
	spec ChunkedPayloadSpec,
	reader io.Reader,
	key []byte,
) (ChunkedPayloadResult, error) {
	if ctx == nil {
		return ChunkedPayloadResult{}, fmt.Errorf("chunked payload context is required")
	}
	if err := ctx.Err(); err != nil {
		return ChunkedPayloadResult{}, err
	}
	stored, err := s.storeChunkStream(ctx, spec, reader, key)
	if err != nil {
		return s.rollbackChunkedResult(err, spec.Owner, stored.IDs, nil)
	}
	root, manifestIDs, err := s.publishChunkManifests(stored, spec)
	if err != nil {
		return s.rollbackChunkedResult(err, spec.Owner, stored.IDs, manifestIDs)
	}
	if err := s.finalizeChunkedPayload(stored.IDs, root.Retention); err != nil {
		return s.rollbackChunkedResult(err, spec.Owner, stored.IDs, manifestIDs)
	}
	return ChunkedPayloadResult{Root: root, ChunkCount: stored.Count, TotalPlaintextBytes: stored.Total}, nil
}

func (s *Service) storeChunkStream(ctx context.Context, spec ChunkedPayloadSpec, reader io.Reader, key []byte) (chunkStoreOutcome, error) {
	outcome := chunkStoreOutcome{IDs: make([]string, 0)}
	count, total, err := Stream(reader, func(_ int, plaintext []byte) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		if len(outcome.IDs) >= MaxLeafRefs*MaxRootRefs {
			return fmt.Errorf("chunked payload exceeds maximum shape")
		}
		blob, storeErr := s.storeStagedChunk(Blob{
			MediaType: spec.MediaType,
			Retention: "staging",
		}, plaintext, key, spec.KeyID)
		if blob.Reference.String() != "" {
			outcome.IDs = append(outcome.IDs, blob.Reference.String())
		}
		if storeErr == nil && len(outcome.IDs)%stagedChunkCheckpointInterval == 0 {
			storeErr = s.Save()
		}
		return storeErr
	})
	outcome.Count, outcome.Total = count, total
	return outcome, err
}

func (s *Service) publishChunkManifests(stored chunkStoreOutcome, spec ChunkedPayloadSpec) (Manifest, []string, error) {
	plan, err := Plan(stored.IDs, ManifestSpec{
		Owner: spec.Owner, MediaType: spec.MediaType, KeyID: spec.KeyID,
		Access: spec.Access, Retention: spec.Retention, TotalPlaintextBytes: stored.Total,
	})
	if err != nil {
		return Manifest{}, nil, err
	}
	created := make([]string, 0, len(plan.Leaves)+1)
	root := manifestSnapshot(plan.Root)
	for _, leaf := range plan.Leaves {
		published, publishErr := s.PublishManifest(manifestSnapshot(leaf))
		if published.ID != "" {
			created = append(created, published.ID)
		}
		if publishErr != nil {
			return Manifest{}, created, publishErr
		}
		if published.ID == plan.Root.ID {
			root = published
		}
	}
	if plan.Root.ID != plan.Leaves[0].ID {
		published, publishErr := s.PublishManifest(manifestSnapshot(plan.Root))
		if published.ID != "" {
			created = append(created, published.ID)
		}
		if publishErr != nil {
			return Manifest{}, created, publishErr
		}
		root = published
	}
	return root, created, nil
}

func (s *Service) rollbackChunkedResult(cause error, owner principal.ID, blobIDs, manifestIDs []string) (ChunkedPayloadResult, error) {
	return ChunkedPayloadResult{}, errors.Join(cause, s.rollbackChunkedPayload(owner, blobIDs, manifestIDs))
}

func (s *Service) storeStagedChunk(blob Blob, plaintext, key []byte, keyID string) (Blob, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	stored, err := StoreEncrypted(blob, plaintext, key, keyID, func(item Blob, ciphertext []byte) (Blob, error) {
		return Store(&s.blobs, item, ciphertext, s.writePayloadLocked)
	})
	if err != nil {
		return stored, err
	}
	stored.State = "staging"
	s.blobs.Put(stored)
	return stored, nil
}

func (s *Service) finalizeChunkedPayload(blobIDs []string, retention string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if retention == "" {
		retention = "durable"
	}
	for _, id := range blobIDs {
		blob, ok := s.blobs.Get(id)
		if !ok || blob.Retention != "staging" {
			return fmt.Errorf("chunked payload staging state is inconsistent")
		}
		blob.Retention = retention
		blob.State = "available-local"
		s.blobs.Put(blob)
	}
	return s.saveLocked()
}

func (s *Service) rollbackChunkedPayload(owner principal.ID, blobIDs, manifestIDs []string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	var rollbackErr error
	for _, id := range manifestIDs {
		s.manifests.Delete(owner, id)
	}
	for _, id := range blobIDs {
		err := os.Remove(s.payloadPath(id))
		if err != nil && !os.IsNotExist(err) {
			rollbackErr = errors.Join(rollbackErr, err)
		}
		s.blobs.Delete(id)
	}
	return errors.Join(rollbackErr, s.saveLocked())
}
