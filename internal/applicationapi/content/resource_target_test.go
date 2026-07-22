package content_test

import (
	applicationcontent "ardents/internal/applicationapi/content"
	applicationv1 "ardents/sdk/go/protocol/applicationv1"
	applicationv1connect "ardents/sdk/go/protocol/applicationv1/applicationv1connect"
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCanonicalizeApplicationPutResource(t *testing.T) {
	target, err := applicationcontent.CanonicalizeResource(
		applicationv1connect.ContentServicePutProcedure,
		&applicationv1.PutContentRequest{Payload: []byte("content")},
	)
	require.NoError(t, err)
	require.Equal(t, applicationcontent.ResourceTarget{Kind: "content-owner"}, target)

	target, err = applicationcontent.CanonicalizeResource(
		applicationv1connect.ContentServicePutProcedure,
		&applicationv1.PutContentRequest{Payload: make([]byte, applicationv1.MaxUnaryPayloadBytes)},
	)
	require.NoError(t, err)
	require.Equal(t, applicationcontent.ResourceTarget{Kind: "content-owner"}, target)
}

func TestCanonicalizeApplicationPutRejectsInvalidPayload(t *testing.T) {
	var nilRequest *applicationv1.PutContentRequest
	tests := []struct {
		name    string
		message any
		target  error
	}{
		{"nil", nil, applicationcontent.ErrInvalidResourceTarget},
		{"typed nil", nilRequest, applicationcontent.ErrInvalidResourceTarget},
		{"wrong type", &applicationv1.GetContentRequest{}, applicationcontent.ErrInvalidResourceTarget},
		{"empty", &applicationv1.PutContentRequest{}, applicationcontent.ErrInvalidResourceTarget},
		{"too large", &applicationv1.PutContentRequest{Payload: make([]byte, applicationv1.MaxUnaryPayloadBytes+1)}, applicationcontent.ErrPayloadTooLarge},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := applicationcontent.CanonicalizeResource(applicationv1connect.ContentServicePutProcedure, test.message)
			require.ErrorIs(t, err, test.target)
		})
	}
}

func TestCanonicalizeApplicationGetUsesExactBlobReference(t *testing.T) {
	target, err := applicationcontent.CanonicalizeResource(
		applicationv1connect.ContentServiceGetProcedure,
		&applicationv1.GetContentRequest{Reference: &applicationv1.ContentReference{Kind: "blob", Id: "bafkreiexact"}},
	)
	require.NoError(t, err)
	require.Equal(t, applicationcontent.ResourceTarget{Kind: "owned-content", ID: "bafkreiexact"}, target)
}

func TestCanonicalizeApplicationGetRejectsNonCanonicalReference(t *testing.T) {
	tests := []struct {
		name      string
		reference *applicationv1.ContentReference
	}{
		{"missing", nil},
		{"unknown kind", &applicationv1.ContentReference{Kind: "object", Id: "id"}},
		{"case changed kind", &applicationv1.ContentReference{Kind: "Blob", Id: "id"}},
		{"empty id", &applicationv1.ContentReference{Kind: "blob"}},
		{"leading whitespace", &applicationv1.ContentReference{Kind: "blob", Id: " id"}},
		{"trailing whitespace", &applicationv1.ContentReference{Kind: "blob", Id: "id "}},
		{"control byte", &applicationv1.ContentReference{Kind: "blob", Id: "id\n"}},
		{"too long", &applicationv1.ContentReference{Kind: "blob", Id: strings.Repeat("x", 513)}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := applicationcontent.CanonicalizeResource(
				applicationv1connect.ContentServiceGetProcedure,
				&applicationv1.GetContentRequest{Reference: test.reference},
			)
			require.ErrorIs(t, err, applicationcontent.ErrInvalidResourceTarget)
		})
	}
}

func TestCanonicalizeApplicationResourceRejectsUnknownFieldsRecursively(t *testing.T) {
	rootUnknown := &applicationv1.PutContentRequest{Payload: []byte("content")}
	rootUnknown.ProtoReflect().SetUnknown([]byte{0x98, 0x06, 0x01})
	_, err := applicationcontent.CanonicalizeResource(applicationv1connect.ContentServicePutProcedure, rootUnknown)
	require.ErrorIs(t, err, applicationcontent.ErrInvalidResourceTarget)

	nestedUnknown := &applicationv1.GetContentRequest{Reference: &applicationv1.ContentReference{Kind: "blob", Id: "id"}}
	nestedUnknown.Reference.ProtoReflect().SetUnknown([]byte{0x98, 0x06, 0x01})
	_, err = applicationcontent.CanonicalizeResource(applicationv1connect.ContentServiceGetProcedure, nestedUnknown)
	require.ErrorIs(t, err, applicationcontent.ErrInvalidResourceTarget)
}

func TestCanonicalizeApplicationResourceRejectsUnknownProcedureBeforeRequestUse(t *testing.T) {
	_, err := applicationcontent.CanonicalizeResource("/unknown", nil)
	require.True(t, errors.Is(err, applicationcontent.ErrUnknownProcedure))
}
