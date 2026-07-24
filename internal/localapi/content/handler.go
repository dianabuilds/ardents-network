package content

import (
	"errors"
	"time"

	domain "ardents/internal/content"
	"ardents/internal/identity/principal"
)

var ErrQueryRequired = errors.New("content query is required")

type Reader interface {
	GetObject(principal.ID, string) (domain.Object, bool)
	ListObjects(principal.ID) []domain.Object
	GetBlob(string) (domain.Blob, bool)
	ListBlobs() []domain.Blob
	GetManifest(principal.ID, string) (domain.Manifest, bool)
	ListManifests(principal.ID) []domain.Manifest
	InventorySnapshot() domain.InventorySnapshot
}

type Commands interface {
	PublishObject(domain.Object) (domain.Object, error)
	PublishBlob(domain.PublishBlobCommand) (domain.Blob, error)
	RetainBlob(string, time.Time) (domain.Blob, error)
	PinBlob(string) (domain.Blob, error)
	DropBlob(string) (domain.Blob, error)
	PublishManifest(domain.Manifest) (domain.Manifest, error)
}

type QueryHandler struct {
	content  Reader
	commands Commands
}

func NewHandler(content Reader, commands Commands) (*QueryHandler, error) {
	if content == nil {
		return nil, ErrQueryRequired
	}
	return &QueryHandler{content: content, commands: commands}, nil
}
