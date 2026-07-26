package adapter

import (
	"context"
	"strings"

	"ardents/sdk/go/content"
	sdkerrors "ardents/sdk/go/errors"
	applicationv1 "ardents/sdk/go/protocol/applicationv1"
	applicationv1connect "ardents/sdk/go/protocol/applicationv1/applicationv1connect"

	"connectrpc.com/connect"
)

type Content struct {
	client applicationv1connect.ContentServiceClient
}

func NewContent(httpClient connect.HTTPClient, endpoint string, options ...connect.ClientOption) *Content {
	clientOptions := []connect.ClientOption{
		connect.WithReadMaxBytes(applicationv1.MaxUnaryMessageBytes),
		connect.WithSendMaxBytes(applicationv1.MaxUnaryMessageBytes),
	}
	clientOptions = append(clientOptions, options...)
	return &Content{
		client: applicationv1connect.NewContentServiceClient(
			httpClient,
			strings.TrimRight(endpoint, "/"),
			clientOptions...,
		),
	}
}

func (c *Content) Put(ctx context.Context, payload []byte, options ...content.PutOption) (content.Reference, error) {
	configured := content.PutOptions{}
	for _, option := range options {
		if option != nil {
			option(&configured)
		}
	}
	req := connect.NewRequest(&applicationv1.PutContentRequest{Payload: append([]byte(nil), payload...), MediaType: configured.MediaType})
	response, err := c.client.Put(ctx, req)
	if err != nil {
		return content.Reference{}, mapError(err)
	}
	reference := response.Msg.GetReference()
	if reference == nil {
		return content.Reference{}, &sdkerrors.Error{Code: sdkerrors.Internal, Operation: "application.content.put", Message: "content response has no reference"}
	}
	return content.Reference{Kind: reference.GetKind(), ID: reference.GetId()}, nil
}

func (c *Content) Get(ctx context.Context, reference content.Reference) ([]byte, error) {
	req := connect.NewRequest(&applicationv1.GetContentRequest{Reference: &applicationv1.ContentReference{Kind: reference.Kind, Id: reference.ID}})
	response, err := c.client.Get(ctx, req)
	if err != nil {
		return nil, mapError(err)
	}
	return append([]byte(nil), response.Msg.GetPayload()...), nil
}
