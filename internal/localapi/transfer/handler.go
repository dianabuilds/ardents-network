// Package transfer owns peer-transfer protocol handlers and mappings.
// It does not own local content lifecycle, transfer execution, or retention.
package transfer

import (
	"context"

	domain "ardents/internal/content"
	localauth "ardents/internal/localapi/auth"
	transferdomain "ardents/internal/transfer"
)

type Sources interface {
	ListBlobSources(string) []domain.BlobSourceRecord
}

type Records interface {
	Get(string) (transferdomain.Record, bool)
	List() []transferdomain.Record
}

type Fetcher interface {
	FetchBlob(context.Context, string) (domain.Blob, error)
}

type Handler struct {
	sources Sources
	records Records
	fetcher Fetcher
	auth    localauth.Config
}

func NewHandler(sources Sources, records Records, fetcher Fetcher, auth localauth.Config) *Handler {
	return &Handler{sources: sources, records: records, fetcher: fetcher, auth: auth}
}
