package content

import (
	"errors"
	"time"

	domain "ardents/internal/content"
	localauth "ardents/internal/localapi/auth"
)

var ErrQueryRequired = errors.New("content query is required")

type Reader interface {
	GetObject(string) (domain.Object, bool)
	ListObjects() []domain.Object
	GetBlob(string) (domain.Blob, bool)
	ListBlobs() []domain.Blob
	GetManifest(string) (domain.Manifest, bool)
	ListManifests() []domain.Manifest
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
	auth     localauth.Config
}

func NewHandler(content Reader, commands Commands, auth localauth.Config) (*QueryHandler, error) {
	if content == nil {
		return nil, ErrQueryRequired
	}
	return &QueryHandler{content: content, commands: commands, auth: auth}, nil
}
