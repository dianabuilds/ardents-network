package blob

import (
	"fmt"

	model "ardents/internal/data/model"
	datapayload "ardents/internal/data/payload"
	statepkg "ardents/internal/data/state"
)

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

func DecryptPayload(blobs *statepkg.BlobStore, id string, key []byte, readPayload func(string) ([]byte, error)) ([]byte, error) {
	blob, raw, encKey, err := loadEncryptedBlob(blobs, id, key, readPayload)
	if err != nil {
		return nil, err
	}
	return datapayload.Decrypt(blob, raw, encKey)
}

func loadEncryptedBlob(blobs *statepkg.BlobStore, id string, key []byte, readPayload func(string) ([]byte, error)) (model.Blob, []byte, []byte, error) {
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
