package data

import blobpkg "ardents/internal/data/blob"

const blobCipherAES256GCM = blobpkg.CipherAES256GCM

func (s *Service) StoreEncryptedBlob(blob Blob, plaintext, key []byte, keyID string) (Blob, error) {
	return blobpkg.StoreEncrypted(blob, plaintext, key, keyID, s.StoreBlob)
}

func (s *Service) DecryptBlobPayload(id string, key []byte) ([]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	return blobpkg.DecryptPayload(&s.blobs, id, key, func(id string) ([]byte, error) {
		return s.readPayloadLocked(id)
	})
}
