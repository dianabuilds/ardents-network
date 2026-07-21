package content

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestContentContractsPreservePersistedReferenceFields(t *testing.T) {
	objectWire, err := json.Marshal(Object{ID: "object-1", BlobRefs: []Ref{{Kind: "blob", ID: "blob-1"}}})
	require.NoError(t, err)
	require.Contains(t, string(objectWire), `"blob_refs"`)

	var object Object
	require.NoError(t, json.Unmarshal(objectWire, &object))
	require.Equal(t, []Ref{{Kind: "blob", ID: "blob-1"}}, object.BlobRefs)

	manifestWire, err := json.Marshal(Manifest{ID: "manifest-1", Refs: []Ref{{Kind: "blob", ID: "blob-1"}}})
	require.NoError(t, err)

	var manifest Manifest
	require.NoError(t, json.Unmarshal(manifestWire, &manifest))
	require.Equal(t, []Ref{{Kind: "blob", ID: "blob-1"}}, manifest.Refs)
}
