package chunking

import (
	"fmt"

	model "ardents/internal/data/model"
)

type ResolvedPlan struct {
	Root                model.Manifest
	Leaves              []model.Manifest
	ChunkIDs            []string
	TotalPlaintextBytes int64
}

func Resolve(root model.Manifest, lookup func(string) (model.Manifest, bool)) (ResolvedPlan, error) {
	if err := ValidateManifest(root); err != nil {
		return ResolvedPlan{}, err
	}
	if root.Kind == "chunk-leaf" {
		return resolvedLeaf(root), nil
	}
	if lookup == nil {
		return ResolvedPlan{}, fmt.Errorf("chunk manifest resolver is unavailable")
	}
	rootMetadata, _ := canonicalMetadata(root.Metadata)
	plan := ResolvedPlan{Root: root, Leaves: make([]model.Manifest, 0, len(root.Refs)), TotalPlaintextBytes: rootMetadata.TotalPlaintextBytes}
	var nextStart int64
	for _, ref := range root.Refs {
		leaf, ok := lookup(ref.ID)
		if !ok {
			return ResolvedPlan{}, fmt.Errorf("chunk leaf manifest %q is unavailable", ref.ID)
		}
		if err := ValidateManifest(leaf); err != nil {
			return ResolvedPlan{}, err
		}
		metadata, _ := canonicalMetadata(leaf.Metadata)
		if leaf.Kind != "chunk-leaf" || leaf.ID != ref.ID || metadata.ChunkStart != nextStart || !sameManifestContract(root, leaf) {
			return ResolvedPlan{}, fmt.Errorf("chunk leaf manifest contract mismatch")
		}
		plan.Leaves = append(plan.Leaves, leaf)
		for _, chunkRef := range leaf.Refs {
			plan.ChunkIDs = append(plan.ChunkIDs, chunkRef.ID)
		}
		nextStart += metadata.ChunkCount
	}
	if nextStart != rootMetadata.TotalChunkCount {
		return ResolvedPlan{}, fmt.Errorf("chunk manifest tree is incomplete")
	}
	return plan, nil
}

func resolvedLeaf(leaf model.Manifest) ResolvedPlan {
	metadata, _ := canonicalMetadata(leaf.Metadata)
	ids := make([]string, len(leaf.Refs))
	for index := range leaf.Refs {
		ids[index] = leaf.Refs[index].ID
	}
	return ResolvedPlan{Root: leaf, Leaves: []model.Manifest{leaf}, ChunkIDs: ids, TotalPlaintextBytes: metadata.TotalPlaintextBytes}
}

func sameManifestContract(root, leaf model.Manifest) bool {
	rootMetadata, _ := canonicalMetadata(root.Metadata)
	leafMetadata, _ := canonicalMetadata(leaf.Metadata)
	return root.Owner == leaf.Owner && root.Access == leaf.Access && root.Retention == leaf.Retention &&
		rootMetadata.TotalChunkCount == leafMetadata.TotalChunkCount &&
		rootMetadata.TotalPlaintextBytes == leafMetadata.TotalPlaintextBytes &&
		rootMetadata.MediaType == leafMetadata.MediaType && rootMetadata.Cipher == leafMetadata.Cipher && rootMetadata.KeyID == leafMetadata.KeyID
}
