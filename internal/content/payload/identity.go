package payload

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"

	model "ardents/internal/content/catalog"

	"github.com/ipfs/go-cid"
	mh "github.com/multiformats/go-multihash"
)

func DeriveIdentity(raw []byte) (string, model.ContentReference, error) {
	sum := sha256.Sum256(raw)
	hash := "sha256:" + hex.EncodeToString(sum[:])
	encoded, err := mh.Encode(sum[:], mh.SHA2_256)
	if err != nil {
		return "", model.ContentReference{}, err
	}
	reference, err := model.ParseContentReference(cid.NewCidV1(cid.Raw, encoded).String())
	return hash, reference, err
}

func ApplyDerivedIdentity(blob *model.Blob, hash string, reference model.ContentReference) error {
	if blob.Reference.String() != "" && !blob.Reference.Equal(reference) {
		return fmt.Errorf("blob content reference mismatch")
	}
	if blob.Hash != "" && blob.Hash != hash {
		return fmt.Errorf("blob hash mismatch")
	}
	blob.Reference = reference
	blob.Hash = hash
	return nil
}

func ValidateMetadataIdentity(blob model.Blob) error {
	if blob.Reference.String() == "" {
		return fmt.Errorf("blob content reference is required")
	}
	return nil
}
