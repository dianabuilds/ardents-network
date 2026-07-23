package content

import (
	"bytes"
	"fmt"
	"testing"

	model "ardents/internal/content/catalog"

	"github.com/stretchr/testify/require"
)

func TestStreamUsesCanonicalChunkSizeAndPreservesOrder(t *testing.T) {
	payload := bytes.Repeat([]byte("x"), PlaintextChunkSize+1)
	var chunks [][]byte
	count, total, err := Stream(bytes.NewReader(payload), func(index int, chunk []byte) error {
		require.Equal(t, len(chunks), index)
		chunks = append(chunks, append([]byte(nil), chunk...))
		return nil
	})
	require.NoError(t, err)
	require.Equal(t, 2, count)
	require.Equal(t, int64(len(payload)), total)
	require.Len(t, chunks[0], PlaintextChunkSize)
	require.Len(t, chunks[1], 1)
	require.Equal(t, payload, append(chunks[0], chunks[1]...))
}

func TestStreamRejectsEmptyPayloadAndPropagatesCancellation(t *testing.T) {
	_, _, err := Stream(bytes.NewReader(nil), func(int, []byte) error { return nil })
	require.ErrorContains(t, err, "empty")

	stop := fmt.Errorf("cancelled")
	_, _, err = Stream(bytes.NewReader(bytes.Repeat([]byte("x"), PlaintextChunkSize+1)), func(index int, _ []byte) error {
		if index == 1 {
			return stop
		}
		return nil
	})
	require.ErrorIs(t, err, stop)
}

func TestPlanBuildsDeterministicBoundedManifestTree(t *testing.T) {
	chunkIDs := make([]string, MaxLeafRefs+1)
	for index := range chunkIDs {
		chunkIDs[index] = fmt.Sprintf("chunk-%04d", index)
	}
	spec := ManifestSpec{
		Owner: contentTestOwner(0x35), MediaType: "application/octet-stream", KeyID: "key-1",
		Access: "participants", Retention: "durable", TotalPlaintextBytes: 42,
	}
	first, err := Plan(chunkIDs, spec)
	require.NoError(t, err)
	second, err := Plan(chunkIDs, spec)
	require.NoError(t, err)
	require.Equal(t, first, second)
	require.Len(t, first.Leaves, 2)
	require.Equal(t, "chunk-root", first.Root.Kind)
	require.Len(t, first.Root.Refs, 2)
	require.Equal(t, "manifest", first.Root.Refs[0].Kind)
	require.Equal(t, first.Leaves[0].ID, first.Root.Refs[0].ID)
	require.NotEqual(t, first.Leaves[0].ID, first.Leaves[1].ID)
}

func TestPlanRejectsInvalidOrOversizedShape(t *testing.T) {
	_, err := Plan(nil, ManifestSpec{})
	require.Error(t, err)
	tooMany := make([]string, MaxLeafRefs*MaxRootRefs+1)
	for index := range tooMany {
		tooMany[index] = fmt.Sprintf("chunk-%d", index)
	}
	_, err = Plan(tooMany, ManifestSpec{
		Owner: contentTestOwner(0x35), MediaType: "application/octet-stream", KeyID: "key",
		TotalPlaintextBytes: int64(len(tooMany)),
	})
	require.ErrorContains(t, err, "maximum")
}

func TestValidateManifestRejectsTamperedIdentityAndShape(t *testing.T) {
	plan, err := Plan([]string{"chunk-1", "chunk-2"}, ManifestSpec{
		Owner: contentTestOwner(0x35), MediaType: "application/octet-stream", KeyID: "key-1",
		TotalPlaintextBytes: PlaintextChunkSize + 1,
	})
	require.NoError(t, err)
	require.NoError(t, ValidateManifest(plan.Root))

	tampered := plan.Root
	tampered.Refs = append([]model.Ref(nil), plan.Root.Refs...)
	tampered.Refs[0].ID = "different-chunk"
	require.ErrorContains(t, ValidateManifest(tampered), "identity")
	tampered = plan.Root
	tampered.Refs = append([]model.Ref(nil), plan.Root.Refs...)
	tampered.Refs[0].Kind = "manifest"
	require.ErrorContains(t, ValidateManifest(tampered), "ref")
}

func TestResolvePreservesLeafAndChunkOrder(t *testing.T) {
	ids := make([]string, MaxLeafRefs+1)
	for index := range ids {
		ids[index] = fmt.Sprintf("chunk-%d", index)
	}
	plan, err := Plan(ids, ManifestSpec{
		Owner: contentTestOwner(0x35), MediaType: "application/octet-stream", KeyID: "key-1",
		TotalPlaintextBytes: int64(len(ids)),
	})
	require.NoError(t, err)
	leaves := map[string]model.Manifest{}
	for _, leaf := range plan.Leaves {
		leaves[leaf.ID] = leaf
	}
	resolved, err := Resolve(plan.Root, func(id string) (model.Manifest, bool) {
		leaf, ok := leaves[id]
		return leaf, ok
	})
	require.NoError(t, err)
	require.Equal(t, ids, resolved.ChunkIDs)

	swapped := map[string]model.Manifest{plan.Leaves[0].ID: plan.Leaves[1], plan.Leaves[1].ID: plan.Leaves[0]}
	_, err = Resolve(plan.Root, func(id string) (model.Manifest, bool) {
		leaf, ok := swapped[id]
		return leaf, ok
	})
	require.ErrorContains(t, err, "mismatch")
}
