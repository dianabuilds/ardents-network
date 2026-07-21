package content

import (
	model "ardents/internal/content/catalog"
	datapayload "ardents/internal/content/payload"
	"fmt"
)

const blobCipherAES256GCM = CipherAES256GCM

func (s *Service) StoreEncryptedBlob(blob Blob, plaintext, key []byte, keyID string) (Blob, error) {
	return StoreEncrypted(blob, plaintext, key, keyID, s.StoreBlob)
}

func (s *Service) DecryptBlobPayload(id string, key []byte) ([]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	return DecryptPayload(&s.blobs, id, key, func(id string) ([]byte, error) {
		return s.readPayloadLocked(id)
	})
}

const CipherAES256GCM = datapayload.AES256GCMCipher

func StoreEncrypted(blob model.Blob, plaintext, key []byte, keyID string, store func(model.Blob, []byte) (model.Blob, error)) (model.Blob, error) {
	if len(plaintext) == 0 {
		return model.Blob{}, fmt.Errorf("encrypted blob requires plaintext payload")
	}
	encKey, err := datapayload.ValidateKey(key)
	if err != nil {
		return model.Blob{}, err
	}
	ciphertext, nonce, err := datapayload.Encrypt(plaintext, encKey)
	if err != nil {
		return model.Blob{}, err
	}
	blob = Normalize(blob)
	blob.Encrypted = true
	blob.Cipher = datapayload.AES256GCMCipher
	blob.KeyID = datapayload.NormalizeKeyID(keyID, encKey)
	blob.Nonce = nonce
	return store(blob, ciphertext)
}

func DecryptPayload(blobs *model.BlobStore, id string, key []byte, readPayload func(string) ([]byte, error)) ([]byte, error) {
	blob, raw, encKey, err := loadEncryptedBlob(blobs, id, key, readPayload)
	if err != nil {
		return nil, err
	}
	return datapayload.Decrypt(blob, raw, encKey)
}

func loadEncryptedBlob(blobs *model.BlobStore, id string, key []byte, readPayload func(string) ([]byte, error)) (model.Blob, []byte, []byte, error) {
	blob, ok := blobs.Get(id)
	if !ok {
		return model.Blob{}, nil, nil, fmt.Errorf("blob not found")
	}
	if !blob.Encrypted {
		return model.Blob{}, nil, nil, fmt.Errorf("blob is not encrypted")
	}
	if blob.Cipher != datapayload.AES256GCMCipher {
		return model.Blob{}, nil, nil, fmt.Errorf("unsupported blob cipher")
	}
	encKey, err := datapayload.ValidateKey(key)
	if err != nil {
		return model.Blob{}, nil, nil, err
	}
	raw, err := readPayload(id)
	if err != nil {
		return model.Blob{}, nil, nil, err
	}
	return blob, raw, encKey, nil
}
