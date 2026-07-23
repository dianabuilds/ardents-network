package content

import (
	model "ardents/internal/content/catalog"
	"ardents/internal/content/payload"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAccessResourceIDRequiresCanonicalContentReference(t *testing.T) {
	id, err := AccessResourceID("obj-1")
	require.NoError(t, err)
	require.Equal(t, "obj-1", id)
	for _, value := range []string{"", " obj-1", "obj-1\n", strings.Repeat("x", 513)} {
		_, err := AccessResourceID(value)
		require.Error(t, err)
	}
}

func TestPublishBlobAccessResourceIDDerivesAndValidatesPayload(t *testing.T) {
	command := PublishBlobCommand{Payload: []byte("hello")}
	id, err := PublishBlobAccessResourceID(command)
	require.NoError(t, err)
	require.NotEmpty(t, id)

	reference, err := model.ParseContentReference(id)
	require.NoError(t, err)
	command.Blob.Reference = reference
	again, err := PublishBlobAccessResourceID(command)
	require.NoError(t, err)
	require.Equal(t, id, again)

	_, different, err := payload.DeriveIdentity([]byte("different"))
	require.NoError(t, err)
	command.Blob.Reference = different
	_, err = PublishBlobAccessResourceID(command)
	require.Error(t, err)
	_, err = PublishBlobAccessResourceID(PublishBlobCommand{})
	require.Error(t, err)
	_, err = PublishBlobAccessResourceID(PublishBlobCommand{Payload: make([]byte, MaxAccessPayloadBytes+1)})
	require.Error(t, err)
}
