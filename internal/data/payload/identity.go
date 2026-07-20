package payload

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"

	model "ardents/internal/data/model"

	"github.com/ipfs/go-cid"
	mh "github.com/multiformats/go-multihash"
)

func DeriveIdentity(raw []byte) (string, string, error) {
	sum := sha256.Sum256(raw)
	hash := "sha256:" + hex.EncodeToString(sum[:])
	encoded, err := mh.Encode(sum[:], mh.SHA2_256)
	if err != nil {
		return "", "", err
	}
	return hash, cid.NewCidV1(cid.Raw, encoded).String(), nil
}

func ApplyDerivedIdentity(blob *model.Blob, hash, blobCID string) error {
	if blob.ID != "" && blob.ID != blobCID {
		return fmt.Errorf("blob id mismatch")
	}
	if blob.Hash != "" && blob.Hash != hash {
		return fmt.Errorf("blob hash mismatch")
	}
	if blob.CID != "" && blob.CID != blobCID {
		return fmt.Errorf("blob cid mismatch")
	}
	blob.ID = blobCID
	blob.Hash = hash
	blob.CID = blobCID
	return nil
}

func ValidateMetadataIdentity(blob model.Blob) error {
	if blob.ID != "" && blob.CID != "" && blob.ID != blob.CID {
		return fmt.Errorf("blob id mismatch")
	}
	return nil
}
